package main

import (
	"context"
	"fmt"
	"gopkg.in/mcuadros/go-syslog.v2/format"
	"runtime"
	"sync/atomic"
)

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
