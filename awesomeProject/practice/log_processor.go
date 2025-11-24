package main

import (
	"gopkg.in/mcuadros/go-syslog.v2/format"
	"sync"
	"time"
)

// 日志处理器接口
type LogProcessor interface {
	Process(logParts format.LogParts) error
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
		p.FlushBuffer()
		return nil
	}

	// 检查是否超时
	if time.Since(p.lastFlush) >= p.batchTimeout && len(p.buffer) > 0 {
		p.FlushBuffer()
	}

	return nil
}
