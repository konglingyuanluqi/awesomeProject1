package main

import (
	"fmt"
	"gopkg.in/mcuadros/go-syslog.v2/format"
	"sync"
	"time"
)

// LogProcessor 日志处理器接口
type LogProcessor interface {
	Process(logParts format.LogParts) error
}

// DefaultLogProcessor 默认日志处理器实现
type DefaultLogProcessor struct {
	batchSize    int
	batchTimeout time.Duration
	buffer       []format.LogParts
	bufferMutex  sync.Mutex
	bufferPool   sync.Pool
	errorCount   int64
	lastFlush    time.Time
	handler      LogBatchHandler // 添加批处理回调处理器
}

// LogBatchHandler 日志批处理回调接口
type LogBatchHandler interface {
	HandleBatch(logs []format.LogParts) error
}

// NewDefaultLogProcessor 创建新的默认日志处理器
func NewDefaultLogProcessor(batchSize int, batchTimeout time.Duration, handler LogBatchHandler) *DefaultLogProcessor {
	return &DefaultLogProcessor{
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		buffer:       make([]format.LogParts, 0, batchSize*2),
		//bufferPool: sync.Pool{
		//	New: func() interface{} {
		//		return make([]byte, 0, 1024)
		//	},
		//},
		lastFlush: time.Now(),
		handler:   handler,
	}
}

// Process 处理单个日志条目
func (p *DefaultLogProcessor) Process(logParts format.LogParts) error {
	p.bufferMutex.Lock()
	defer p.bufferMutex.Unlock()

	p.buffer = append(p.buffer, logParts)

	// 检查是否达到批处理大小
	if len(p.buffer) >= p.batchSize {
		// 创建批处理副本
		batch := make([]format.LogParts, len(p.buffer))
		copy(batch, p.buffer)
		p.buffer = p.buffer[:0]
		p.lastFlush = time.Now()
		// 返回批次供外部处理
		return p.processBatch(batch)
	}

	// 检查是否超时
	if time.Since(p.lastFlush) >= p.batchTimeout && len(p.buffer) > 0 {
		// 创建批处理副本
		batch := make([]format.LogParts, len(p.buffer))
		copy(batch, p.buffer)
		p.buffer = p.buffer[:0]
		p.lastFlush = time.Now()
		// 返回批次供外部处理
		return p.processBatch(batch)
	}

	return nil
}

// processBatch 处理批次数据
func (p *DefaultLogProcessor) processBatch(logs []format.LogParts) error {
	if p.handler != nil {
		return p.handler.HandleBatch(logs)
	}
	return fmt.Errorf("no batch handler configured")
}

// FlushBuffer 刷新缓冲区
func (p *DefaultLogProcessor) FlushBuffer() {
	if len(p.buffer) == 0 {
		return
	}

	batch := make([]format.LogParts, len(p.buffer))
	copy(batch, p.buffer)
	p.buffer = p.buffer[:0]
	p.lastFlush = time.Now()
}
