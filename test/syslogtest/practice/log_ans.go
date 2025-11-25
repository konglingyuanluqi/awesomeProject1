package practice

import (
	"context"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
	"log"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"
)

// PerformanceMetrics 性能指标结构
type PerformanceMetrics struct {
	ProcessedCount int64
	ErrorCount     int64
	AvgProcessTime time.Duration
}

// AntsWorkerPool ANTS工作池结构
type AntsWorkerPool struct {
	pool       *ants.Pool
	processor  LogProcessor
	ctx        context.Context
	cancel     context.CancelFunc
	totalCount int64
	errorCount int64
	metrics    PerformanceMetrics
}

// NewAntsWorkerPool 创建新的ANTS工作池
func NewAntsWorkerPool(workers int, processor LogProcessor) (*AntsWorkerPool, error) {
	ctx, cancel := context.WithCancel(context.Background())

	if workers > runtime.NumCPU()*500 {
		workers = runtime.NumCPU() * 500 // 设置上限
	}

	pool, err := ants.NewPool(
		workers,
		//ants.WithPreAlloc(true),
		ants.WithExpiryDuration(30*time.Second), // 空闲协程30秒后回收
		//TODO 协程池阻塞队列大小
		ants.WithMaxBlockingTasks(50000), // 队列最多等待10000个任务
		ants.WithNonblocking(false),      // 保持阻塞模式，避免任务丢失
		ants.WithPanicHandler(func(i interface{}) {
			// 使用更高效的日志记录
			log.Printf("worker panic: %v\n%s", i, debug.Stack())
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

// Start 启动工作池
func (wp *AntsWorkerPool) Start() {
	// ANTS协程池已经自动启动
}

// Stop 停止工作池
func (wp *AntsWorkerPool) Stop() {
	wp.cancel()
	wp.pool.Release()
}

// AdjustPoolSize 动态调整协程池大小
func (wp *AntsWorkerPool) AdjustPoolSize() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	currentLoad := wp.pool.Running()
	capacity := wp.pool.Cap()
	waiting := wp.pool.Waiting()

	// 计算负载率，同时考虑运行中和等待的任务
	loadRatio := float64(currentLoad+waiting) / float64(capacity)

	// 对于快速大量的小任务，使用更激进的扩容策略
	if loadRatio > 0.7 { // 降低扩容阈值，更快响应
		// 根据等待队列长度决定扩容幅度
		var newCapacity int
		if waiting > capacity { // 等待队列超过当前容量
			newCapacity = capacity * 2 // 双倍扩容
		} else {
			newCapacity = capacity + capacity/2 // 增加50%
		}

		// TODO 协程池maxCapacity
		// 限制最大容量，避免无限制增长
		maxCapacity := runtime.NumCPU() * 600
		if newCapacity > maxCapacity {
			newCapacity = maxCapacity
		}

		if newCapacity > capacity {
			wp.pool.Tune(newCapacity)
		}
	}

	// 缩容策略更加保守，避免频繁缩容导致性能抖动
	if loadRatio < 0.15 && waiting < 10 { // 只有在负载极低且等待任务很少时才缩容
		newCapacity := capacity - capacity/4  // 减少25%
		if newCapacity < runtime.NumCPU()*2 { // 保持最小容量为CPU核心数的2倍
			newCapacity = runtime.NumCPU() * 2
		}

		if newCapacity < capacity {
			wp.pool.Tune(newCapacity)
		}
	}
}

// AddJob 添加任务到工作池
func (wp *AntsWorkerPool) AddJob(logParts format.LogParts) {
	select {
	case <-wp.ctx.Done():
		return
	default:
		err := wp.pool.Submit(func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("处理日志时发生panic: %v", r)
				}
			}()

			if err := wp.processor.Process(logParts); err != nil {
				atomic.AddInt64(&wp.errorCount, 1)
				fmt.Printf("处理日志出错: %v", err)
			}
		})

		if err == nil {
			atomic.AddInt64(&wp.totalCount, 1)
		} else {
			fmt.Printf("提交任务到协程池失败: %v", err)
		}
	}
}

// AddJobBatch 添加任务到工作池（批量）
func (wp *AntsWorkerPool) AddJobBatch(logParts []format.LogParts) error {
	if wp.pool.Waiting() > wp.pool.Cap()*2 {
		return fmt.Errorf("处理队列过载，暂时拒绝新任务")
	}
	for _, log := range logParts {
		wp.AddJob(log)
	}
	return nil
}

// AddJobWithBackpressure 添加任务到工作池（带背压控制）
func (wp *AntsWorkerPool) AddJobWithBackpressure(logParts format.LogParts) error {
	// 检查当前等待队列长度，如果超过容量的80%，返回错误
	if wp.pool.Waiting() > wp.pool.Cap()*8/10 {
		return fmt.Errorf("处理队列过载，暂时拒绝新任务")
	}

	// 提交任务到协程池
	err := wp.pool.Submit(func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("处理日志时发生panic: %v", r)
				atomic.AddInt64(&wp.errorCount, 1)
			}
		}()

		if err := wp.processor.Process(logParts); err != nil {
			atomic.AddInt64(&wp.errorCount, 1)
			fmt.Printf("处理日志出错: %v", err)
		}
	})

	if err != nil {
		return fmt.Errorf("提交任务到协程池失败: %v", err)
	}

	atomic.AddInt64(&wp.totalCount, 1)
	return nil
}

// GetTotalCount 获取已处理的日志总数
func (wp *AntsWorkerPool) GetTotalCount() int64 {
	return atomic.LoadInt64(&wp.totalCount)
}

// Status 获取协程池状态
func (wp *AntsWorkerPool) Status() string {
	return fmt.Sprintf("容量cap: %d, 运行中running: %d, 等待任务wating: %d, 空闲worker: %d",
		wp.pool.Cap(), wp.pool.Running(), wp.pool.Waiting(), wp.pool.Cap()-wp.pool.Running())
}

// GetMetrics 获取性能指标
func (wp *AntsWorkerPool) GetMetrics() PerformanceMetrics {
	return PerformanceMetrics{
		ProcessedCount: atomic.LoadInt64(&wp.totalCount),
		ErrorCount:     atomic.LoadInt64(&wp.errorCount),
		AvgProcessTime: wp.metrics.AvgProcessTime,
	}
}
