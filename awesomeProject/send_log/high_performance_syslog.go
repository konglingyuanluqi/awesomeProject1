package main

import (
	"context"
	"fmt"
	"log/syslog"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// SyslogSender 高性能syslog发送器
type SyslogSender struct {
	writer     *syslog.Writer
	batchSize  int
	bufferPool sync.Pool
}

// NewSyslogSender 创建高性能syslog发送器
func NewSyslogSender(network, raddr string, priority syslog.Priority, tag string) (*SyslogSender, error) {
	writer, err := syslog.Dial(network, raddr, priority, tag)
	if err != nil {
		return nil, fmt.Errorf("创建syslog写入器失败: %v", err)
	}

	return &SyslogSender{
		writer:    writer,
		batchSize: 1000, // 每批发送1000条日志
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024)
			},
		},
	}, nil
}

// Close 关闭syslog连接
func (s *SyslogSender) Close() error {
	return s.writer.Close()
}

// SendLog 发送单条日志
func (s *SyslogSender) SendLog(message string) error {
	return s.writer.Info(message)
}

// BatchSendLogs 批量发送日志
func (s *SyslogSender) BatchSendLogs(messages []string) error {
	for _, msg := range messages {
		if err := s.writer.Info(msg); err != nil {
			return err
		}
	}
	return nil
}

// LogWorker 日志工作协程
type LogWorker struct {
	sender      *SyslogSender
	messageChan chan string
	counter     int64
}

// NewLogWorker 创建日志工作协程
func NewLogWorker(sender *SyslogSender, bufferSize int) *LogWorker {
	return &LogWorker{
		sender:      sender,
		messageChan: make(chan string, bufferSize),
	}
}

// Start 启动工作协程
func (w *LogWorker) Start(ctx context.Context) {
	go func() {
		batch := make([]string, 0, w.sender.batchSize)
		ticker := time.NewTicker(100 * time.Millisecond) // 每100ms处理一批
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// 处理剩余日志
				if len(batch) > 0 {
					w.sender.BatchSendLogs(batch)
				}
				return
			case msg := <-w.messageChan:
				batch = append(batch, msg)
				if len(batch) >= w.sender.batchSize {
					w.sender.BatchSendLogs(batch)
					batch = batch[:0] // 重置切片但保留容量
				}
			case <-ticker.C:
				// 定时处理积累的日志
				if len(batch) > 0 {
					w.sender.BatchSendLogs(batch)
					batch = batch[:0]
				}
			}
		}
	}()
}

// Send 发送日志消息
func (w *LogWorker) Send(message string) {
	select {
	case w.messageChan <- message:
		atomic.AddInt64(&w.counter, 1)
	default:
		// 通道已满，丢弃日志
	}
}

// GetCount 获取已发送日志数
func (w *LogWorker) GetCount() int64 {
	return atomic.LoadInt64(&w.counter)
}

// generateRandomLogMessage 生成随机日志消息
func generateRandomLogMessage() string {
	levels := []string{"INFO", "WARN", "ERROR", "DEBUG"}
	services := []string{"auth", "payment", "order", "inventory", "notification"}

	level := levels[rand.Intn(len(levels))]
	service := services[rand.Intn(len(services))]

	return fmt.Sprintf("[%s] Service: %s, Message: Random log message with timestamp %d",
		level, service, time.Now().UnixNano())
}

// 生成随机整数
func randInt(min, max int) int {
	if min >= max {
		return min
	}
	return min + rand.Intn(max-min+1)
}

// 生成随机IP地址
func randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		randInt(1, 255),
		randInt(0, 255),
		randInt(0, 255),
		randInt(1, 254))
}

const (
	INFO = "12-Sep-2025 17:03:56.635 queries: client @0x7f22f404b620 223.2.43.8#23253 (api.miwifi.com): view ext2: query: api.miwifi.com IN AAAA + (202.119.104.31)"
)

func main() {
	// 设置GOMAXPROCS为CPU核心数
	runtime.GOMAXPROCS(runtime.NumCPU())

	// 创建syslog发送器
	sender, err := NewSyslogSender("udp", "localhost:1515", syslog.LOG_LOCAL0, "high_perf_test")
	if err != nil {
		fmt.Printf("创建syslog发送器失败: %v", err)
		os.Exit(1)
	}
	defer sender.Close()

	// 创建多个工作协程
	workerCount := runtime.NumCPU() * 4 // 使用4倍CPU核心数的工作协程
	workers := make([]*LogWorker, workerCount)

	for i := 0; i < workerCount; i++ {
		workers[i] = NewLogWorker(sender, 10000) // 每个工作协程缓冲10000条消息
	}

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动所有工作协程
	for _, worker := range workers {
		worker.Start(ctx)
	}

	// 设置信号监听，用于优雅关闭程序
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动统计协程
	//statsChan := make(chan int64, workerCount)
	var totalSent int64
	var lastTotalSent int64

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 计算总发送数
				totalSent = 0
				for _, worker := range workers {
					totalSent += worker.GetCount()
				}

				// 计算QPS
				qps := totalSent - lastTotalSent
				lastTotalSent = totalSent

				//if totalSent > 1000 {
				//	return
				//}

				fmt.Printf("总发送: %d, 当前QPS: %d \n", totalSent, qps)
			}
		}
	}()

	// 启动日志发送协程
	for i := 0; i < workerCount; i++ {
		go func(workerID int) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// 高频发送日志
					message := INFO
					workers[workerID].Send(message)

					// 控制发送速率，以达到目标QPS
					// 这里可以根据实际情况调整
					time.Sleep(time.Microsecond * 10)
				}
			}
		}(i)
	}

	fmt.Printf("高性能syslog发送器已启动，使用 %d 个工作协程 \n", workerCount)
	fmt.Println("按Ctrl+C停止发送")

	// 等待信号
	<-sigChan
	fmt.Println("收到关闭信号，正在优雅关闭...")
	cancel()
	time.Sleep(1 * time.Second)
	fmt.Printf("程序已关闭，共发送了 %d 条日志 \n", totalSent)
}
