package main

import (
	"fmt"
	"sync/atomic"
	"time"
)

// 性能指标结构
type PerformanceMetrics struct {
	ProcessedCount int64
	ErrorCount     int64
	AvgProcessTime time.Duration
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
