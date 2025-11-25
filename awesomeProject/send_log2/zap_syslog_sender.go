package main

import (
	"context"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	// 默认日志消息格式
	DEFAULT_LOG_MESSAGE = "12-Sep-2025 17:03:56.635 queries: client @0x7f22f404b620 223.2.43.8#23253 (api.miwifi.com): view ext2: query: api.miwifi.com IN AAAA + (202.119.104.31)"
	// 默认批量大小
	DEFAULT_BATCH_SIZE = 100
	// 默认批量发送间隔
	DEFAULT_BATCH_INTERVAL = 10 * time.Millisecond
)

// SyslogWriter 实现zapcore.WriteSyncer接口，用于写入syslog
type SyslogWriter struct {
	conn    net.Conn
	raddr   string
	network string
	mu      sync.Mutex
}

// NewSyslogWriter 创建syslog写入器
func NewSyslogWriter(network, raddr string) (*SyslogWriter, error) {
	conn, err := net.Dial(network, raddr)
	if err != nil {
		return nil, fmt.Errorf("无法连接到syslog服务器: %v", err)
	}

	return &SyslogWriter{
		conn:    conn,
		raddr:   raddr,
		network: network,
	}, nil
}

// Write 实现io.Writer接口
func (w *SyslogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 如果连接已关闭，重新连接
	if w.conn == nil {
		w.conn, err = net.Dial(w.network, w.raddr)
		if err != nil {
			return 0, err
		}
	}

	// 构建RFC3164格式的syslog消息
	// <priority>timestamp hostname tag message
	// 使用INFO级别(14)和用户设施(1)
	timestamp := time.Now().Format("Jan 2 15:04:05")
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// 从消息中提取内容，去掉zap添加的JSON格式
	message := string(p)
	// 尝试解析JSON并提取message字段
	if strings.HasPrefix(message, "{") {
		// 简单处理，实际应用中可能需要更完整的JSON解析
		if idx := strings.Index(message, `"message":"`); idx != -1 {
			start := idx + len(`"message":"`)
			end := strings.Index(message[start:], `"`)
			if end != -1 {
				message = message[start : start+end]
			}
		}
	}

	syslogMsg := fmt.Sprintf("<14>%s %s zap-syslog: %s", timestamp, hostname, message)

	// 发送消息
	_, err = w.conn.Write([]byte(syslogMsg))
	if err != nil {
		// 发送失败，关闭连接
		w.conn.Close()
		w.conn = nil
		return 0, err
	}

	return len(p), nil
}

// Sync 实现zapcore.WriteSyncer接口
func (w *SyslogWriter) Sync() error {
	return nil
}

// Close 关闭连接
func (w *SyslogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		err := w.conn.Close()
		w.conn = nil
		return err
	}
	return nil
}

// TokenBucket 令牌桶实现
type TokenBucket struct {
	capacity int64     // 桶的容量
	tokens   int64     // 当前令牌数
	rate     int64     // 令牌生成速率，每秒多少个
	lastTime time.Time // 上次更新时间
	mu       sync.Mutex
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(capacity, rate int64) *TokenBucket {
	return &TokenBucket{
		capacity: capacity,
		tokens:   capacity,
		rate:     rate,
		lastTime: time.Now(),
	}
}

// Take 获取令牌，返回是否成功获取
func (tb *TokenBucket) Take(count int64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	// 计算从上次更新到现在新增的令牌数
	elapsed := now.Sub(tb.lastTime).Seconds()
	newTokens := int64(elapsed * float64(tb.rate))

	// 更新令牌数，不超过桶的容量
	tb.tokens = tb.tokens + newTokens
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastTime = now

	// 检查是否有足够的令牌
	if tb.tokens >= count {
		tb.tokens -= count
		return true
	}
	return false
}

// BatchSender 批量发送器
type BatchSender struct {
	logger        *zap.Logger
	batchSize     int
	batchInterval time.Duration
	bufferPool    sync.Pool
	sendChan      chan string
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	sentCount     int64
}

// NewBatchSender 创建批量发送器
func NewBatchSender(raddr string, batchSize int, batchInterval time.Duration) (*BatchSender, error) {
	// 创建syslog写入器
	syslogWriter, err := NewSyslogWriter("udp", raddr)
	if err != nil {
		return nil, fmt.Errorf("创建syslog写入器失败: %v", err)
	}

	// 创建zap核心
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(syslogWriter),
		zap.InfoLevel,
	)

	// 创建zap日志器
	logger := zap.New(core, zap.AddCaller())

	ctx, cancel := context.WithCancel(context.Background())

	sender := &BatchSender{
		logger:        logger,
		batchSize:     batchSize,
		batchInterval: batchInterval,
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]string, 0, batchSize)
			},
		},
		sendChan: make(chan string, batchSize*100), // 增加缓冲通道大小为批量大小的100倍
		ctx:      ctx,
		cancel:   cancel,
	}

	// 启动异步发送协程
	sender.wg.Add(1)
	go sender.asyncSender()

	return sender, nil
}

// Send 发送日志
func (bs *BatchSender) Send(message string) {
	// 使用带超时的阻塞发送，避免日志丢失
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case bs.sendChan <- message:
		// 成功发送到通道
	case <-ctx.Done():
		// 超时，尝试强制发送
		select {
		case bs.sendChan <- message:
			// 强制发送成功
		default:
			// 通道仍然满，尝试非阻塞发送
			select {
			case bs.sendChan <- message:
			default:
				// 最终失败，静默处理，不打印日志
			}
		}
	}
}

// asyncSender 异步发送协程
func (bs *BatchSender) asyncSender() {
	defer bs.wg.Done()

	batch := bs.bufferPool.Get().([]string)
	batch = batch[:0] // 重置切片但保留容量
	ticker := time.NewTicker(bs.batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bs.ctx.Done():
			// 发送剩余的日志
			if len(batch) > 0 {
				bs.sendBatch(batch)
			}
			return
		case msg := <-bs.sendChan:
			batch = append(batch, msg)
			// 如果批量已满，立即发送
			if len(batch) >= bs.batchSize {
				bs.sendBatch(batch)
				batch = batch[:0] // 重置切片但保留容量
			}
		case <-ticker.C:
			// 定时发送积累的日志
			if len(batch) > 0 {
				bs.sendBatch(batch)
				batch = batch[:0] // 重置切片但保留容量
			}
		}
	}
}

// sendBatch 发送批量日志
func (bs *BatchSender) sendBatch(batch []string) {
	for _, msg := range batch {
		bs.logger.Info(msg)
		atomic.AddInt64(&bs.sentCount, 1)
	}

	// 将缓冲区放回对象池
	bs.bufferPool.Put(batch[:0])
}

// Close 关闭发送器
func (bs *BatchSender) Close() {
	bs.cancel()
	bs.wg.Wait()
	bs.logger.Sync()
}

// GetSentCount 获取已发送日志数量
func (bs *BatchSender) GetSentCount() int64 {
	return atomic.LoadInt64(&bs.sentCount)
}

func main() {
	// 添加命令行参数解析
	count := flag.Int("count", -1, "要发送的日志条数，-1表示持续发送")
	qps := flag.Int("qps", 100000, "每秒发送的日志数量")
	workers := flag.Int("workers", runtime.NumCPU(), "并发发送日志的协程数量")
	raddr := flag.String("raddr", "localhost:1515", "远程syslog服务器地址，格式为host:port")
	batchSize := flag.Int("batch", DEFAULT_BATCH_SIZE, "批量发送的日志数量")
	batchInterval := flag.Duration("interval", DEFAULT_BATCH_INTERVAL, "批量发送的时间间隔")
	flag.Parse()

	// 设置GOMAXPROCS为CPU核心数
	runtime.GOMAXPROCS(runtime.NumCPU())

	// 创建批量发送器
	sender, err := NewBatchSender(*raddr, *batchSize, *batchInterval)
	if err != nil {
		log.Fatal("无法创建批量发送器:", err)
	}
	defer sender.Close()

	// 创建令牌桶，容量为QPS的2倍，速率为QPS
	tokenBucket := NewTokenBucket(int64(*qps*2), int64(*qps))

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号监听，用于优雅关闭程序
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动统计协程
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		var lastCount int64
		startTime := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentCount := sender.GetSentCount()
				currentQPS := currentCount - lastCount
				lastCount = currentCount
				elapsed := time.Since(startTime).Seconds()
				avgQPS := float64(currentCount) / elapsed
				log.Printf("已发送: %d | 当前QPS: %d | 平均QPS: %.2f | 目标QPS: %d | 运行时间: %.1fs",
					currentCount, currentQPS, avgQPS, *qps, elapsed)
			}
		}
	}()

	// 计算每个worker需要发送的日志数量
	var perWorkerCount int
	if *count > 0 {
		perWorkerCount = *count / *workers
		if *count%*workers != 0 {
			perWorkerCount++
		}
	}

	// 启动worker协程
	var wg sync.WaitGroup
	wg.Add(*workers)

	for i := 0; i < *workers; i++ {
		go func(id int) {
			defer wg.Done()
			workerSendLogs(ctx, sender, tokenBucket, id, perWorkerCount)
		}(i)
	}

	log.Printf("高性能syslog发送器已启动，使用 %d 个工作协程，目标QPS: %d", *workers, *qps)
	if *count > 0 {
		log.Printf("将发送 %d 条日志", *count)
	} else {
		log.Println("将持续发送日志，按Ctrl+C停止")
	}

	// 等待信号
	<-sigChan
	log.Println("收到关闭信号，正在优雅关闭...")
	cancel()
	wg.Wait()
	time.Sleep(1 * time.Second)
	log.Printf("程序已关闭，共发送了 %d 条日志", sender.GetSentCount())
}

// worker发送日志的函数
func workerSendLogs(ctx context.Context, sender *BatchSender, tokenBucket *TokenBucket, id int, maxCount int) {
	sentCount := 0

	// 初始化随机数生成器
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 检查是否达到最大发送数量
			if maxCount > 0 && sentCount >= maxCount {
				return
			}

			// 从令牌桶获取令牌
			if tokenBucket.Take(1) {
				// 获取到令牌，发送日志
				sender.Send(DEFAULT_LOG_MESSAGE)
				sentCount++
			} else {
				// 没有获取到令牌，短暂等待
				time.Sleep(time.Duration(r.Intn(100)) * time.Microsecond)
			}
		}
	}
}
