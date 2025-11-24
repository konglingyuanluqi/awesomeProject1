package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/panjf2000/ants/v2"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

// 日志处理器接口
type LogProcessor interface {
	Process(logParts format.LogParts) error
}

// 批量日志处理器接口
type BatchLogProcessor interface {
	ProcessBatch(logs []format.LogParts) error
}

// 性能指标结构
type PerformanceMetrics struct {
	ProcessedCount int64
	ErrorCount     int64
	AvgProcessTime time.Duration
}

// 默认日志处理器实现
type DefaultLogProcessor struct {
	batchSize    int
	batchTimeout time.Duration
	buffer       []format.LogParts
	bufferMutex  sync.Mutex
	bufferPool   sync.Pool
	errorCount   int64
	lastFlush    time.Time
}

func NewDefaultLogProcessor(batchSize int, batchTimeout time.Duration) *DefaultLogProcessor {
	return &DefaultLogProcessor{
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		buffer:       make([]format.LogParts, 0, batchSize),
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024)
			},
		},
		lastFlush: time.Now(),
	}
}

func (p *DefaultLogProcessor) Process(logParts format.LogParts) error {
	p.bufferMutex.Lock()
	defer p.bufferMutex.Unlock()

	p.buffer = append(p.buffer, logParts)

	// 检查是否达到批处理大小
	if len(p.buffer) >= p.batchSize {
		p.flushBuffer()
		return nil
	}

	// 检查是否超时
	if time.Since(p.lastFlush) >= p.batchTimeout && len(p.buffer) > 0 {
		p.flushBuffer()
	}

	return nil
}

// 刷新缓冲区
func (p *DefaultLogProcessor) flushBuffer() {
	if len(p.buffer) == 0 {
		return
	}

	batch := make([]format.LogParts, len(p.buffer))
	copy(batch, p.buffer)
	p.buffer = p.buffer[:0]
	p.lastFlush = time.Now()
	go p.processBatch(batch)
}

func (p *DefaultLogProcessor) processBatch(logs []format.LogParts) {
	start := time.Now()
	if err := p.ProcessBatch(logs); err != nil {
		atomic.AddInt64(&p.errorCount, 1)
		fmt.Printf("批量处理日志出错: %v\n", err)
	}
	// 可以在这里记录处理时间等指标
	_ = time.Since(start)
}

func (p *DefaultLogProcessor) ProcessBatch(logs []format.LogParts) error {
	for _, log := range logs {
		//  TODO 处理单条日志
		fmt.Printf("处理日志: %+v\n", log)
	}
	return nil
}

// ANTS工作池结构
type AntsWorkerPool struct {
	pool       *ants.Pool
	processor  LogProcessor
	ctx        context.Context
	cancel     context.CancelFunc
	totalCount int64
	errorCount int64
	metrics    PerformanceMetrics
}

// 创建新的ANTS工作池
func NewAntsWorkerPool(workers int, processor LogProcessor) (*AntsWorkerPool, error) {
	ctx, cancel := context.WithCancel(context.Background())

	pool, err := ants.NewPool(
		workers,
		ants.WithPreAlloc(true),
		ants.WithNonblocking(true),
		ants.WithPanicHandler(func(i interface{}) {
			fmt.Printf("协程池发生panic: %v\n", i)
		}),
	)

	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建ANTS协程池失败: %v", err)
	}

	return &AntsWorkerPool{
		pool:      pool,
		processor: processor,
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

// 动态调整协程池大小
func (wp *AntsWorkerPool) AdjustPoolSize() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	currentLoad := wp.pool.Running()
	capacity := wp.pool.Cap()

	if float64(currentLoad)/float64(capacity) > 0.8 {
		wp.pool.Tune(capacity + runtime.NumCPU())
	}
	if float64(currentLoad)/float64(capacity) < 0.2 {
		newCapacity := capacity - runtime.NumCPU()
		if newCapacity < runtime.NumCPU() {
			newCapacity = runtime.NumCPU()
		}
		wp.pool.Tune(newCapacity)
	}
}

// 启动工作池
func (wp *AntsWorkerPool) Start() {
	// ANTS协程池已经自动启动
}

// 停止工作池
func (wp *AntsWorkerPool) Stop() {
	wp.cancel()
	wp.pool.Release()
}

// 添加任务到工作池（带背压控制）
func (wp *AntsWorkerPool) AddJobWithBackpressure(logParts format.LogParts) error {
	if wp.pool.Waiting() > wp.pool.Cap()*2 {
		return fmt.Errorf("处理队列过载，暂时拒绝新任务")
	}
	wp.AddJob(logParts)
	return nil
}

// 添加任务到工作池
func (wp *AntsWorkerPool) AddJob(logParts format.LogParts) {
	select {
	case <-wp.ctx.Done():
		return
	default:
		err := wp.pool.Submit(func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("处理日志时发生panic: %v\n", r)
				}
			}()

			if err := wp.processor.Process(logParts); err != nil {
				atomic.AddInt64(&wp.errorCount, 1)
				fmt.Printf("处理日志出错: %v\n", err)
			}
		})

		if err == nil {
			atomic.AddInt64(&wp.totalCount, 1)
		} else {
			fmt.Printf("提交任务到协程池失败: %v\n", err)
		}
	}
}

// 获取已处理的日志总数
func (wp *AntsWorkerPool) GetTotalCount() int64 {
	return atomic.LoadInt64(&wp.totalCount)
}

// 获取协程池状态
func (wp *AntsWorkerPool) Status() string {
	return fmt.Sprintf("协程数: %d, 运行中: %d, 等待任务: %d, 完成任务: %d\n",
		wp.pool.Cap(), wp.pool.Running(), wp.pool.Waiting(), wp.pool.Cap()-wp.pool.Running())
}

// 获取性能指标
func (wp *AntsWorkerPool) GetMetrics() PerformanceMetrics {
	return PerformanceMetrics{
		ProcessedCount: atomic.LoadInt64(&wp.totalCount),
		ErrorCount:     atomic.LoadInt64(&wp.errorCount),
		AvgProcessTime: wp.metrics.AvgProcessTime,
	}
}

func main() {
	// 获取系统CPU核心数
	cpuNum := runtime.NumCPU()
	fmt.Printf("检测到 %d 个CPU核心\n", cpuNum)

	// 创建一个 syslog 服务器实例
	server := syslog.NewServer()
	server.SetFormat(syslog.RFC3164)

	// 动态计算缓冲区大小
	workerCount := cpuNum * 2
	bufferSize := workerCount * 1000
	channel := make(syslog.LogPartsChannel, bufferSize)
	handler := syslog.NewChannelHandler(channel)
	server.SetHandler(handler)

	// 配置服务器监听 TCP 和 UDP 端口
	server.ListenTCP("0.0.0.0:1514")
	server.ListenUDP("0.0.0.0:1514")

	// 启动服务器
	server.Boot()

	// 创建日志处理器
	batchSize := 1000          // 批量处理大小
	timeout := time.Second * 1 // 超时时间
	processor := NewDefaultLogProcessor(batchSize, timeout)

	// 创建ANTS工作池
	pool, err := NewAntsWorkerPool(workerCount, processor)
	if err != nil {
		fmt.Printf("创建工作池失败: %v\n", err)
		return
	}
	pool.Start()

	fmt.Printf("已启动 %d 个工作协程处理日志\n", workerCount)

	// 启动统计协程
	go func(pool *AntsWorkerPool) {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		var lastCount int64
		for {
			select {
			case <-ticker.C:
				pool.AdjustPoolSize() // 动态调整协程池大小
				currentCount := pool.GetTotalCount()
				increment := currentCount - lastCount
				metrics := pool.GetMetrics()
				log.Printf("已处理总日志数: %d, 最近5秒处理: %d, 错误数: %d, %s",
					currentCount, increment, metrics.ErrorCount, pool.Status())
				lastCount = currentCount
			}
		}
	}(pool)

	// 启动一个 goroutine 从通道读取日志并分发到工作池
	go func(channel syslog.LogPartsChannel, pool *AntsWorkerPool) {
		for logParts := range channel {
			if err := pool.AddJobWithBackpressure(logParts); err != nil {
				fmt.Printf("添加任务失败: %v\n", err)
			}
		}
	}(channel, pool)

	// 设置信号监听，用于优雅关闭程序
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待关闭信号
	<-sigChan
	fmt.Println("收到关闭信号，正在优雅关闭...")

	// 停止接收新日志
	server.Kill()

	// 停止工作池
	pool.Stop()

	fmt.Printf("程序已关闭，共处理了 %d 条日志\n", pool.GetTotalCount())
}
