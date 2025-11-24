package main

import (
	"fmt"
	"gopkg.in/mcuadros/go-syslog.v2/format"
	"sync/atomic"
	"time"
)

// 批量日志处理器接口
type BatchLogProcessor interface {
	ProcessBatch(logs []format.LogParts) error
}

// 刷新缓冲区
func (p *DefaultLogProcessor) FlushBuffer() {
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
