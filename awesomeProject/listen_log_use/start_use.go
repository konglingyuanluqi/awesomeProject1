package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
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

// 默认日志处理器实现
type DefaultLogProcessor struct{}

func (p *DefaultLogProcessor) Process(logParts format.LogParts) error {
	// 在这里实现具体的日志处理逻辑
	// 例如：存储到数据库、写入文件、发送到消息队列等
	// 解析并格式化日志内容
	timestamp, _ := logParts["timestamp"]
	content, _ := logParts["content"]
	priority, _ := logParts["priority"]
	version, _ := logParts["version"]
	appName, _ := logParts["app_name"]
	hostname, _ := logParts["hostname"]

	fmt.Printf("时间戳: %v", timestamp)
	fmt.Printf("主机名: %v", hostname)
	fmt.Printf("应用名: %v", appName)
	fmt.Printf("优先级: %v", priority)
	if version != nil {
		fmt.Printf("版本: %v", version)
	}
	fmt.Printf("内容: %v", content)
	fmt.Println("================")
	fmt.Println()
	return nil
}

// ANTS工作池结构
type AntsWorkerPool struct {
	pool       *ants.Pool
	processor  LogProcessor
	ctx        context.Context
	cancel     context.CancelFunc
	totalCount int64 // 处理的总日志数
}

// 创建新的ANTS工作池
func NewAntsWorkerPool(workers int, processor LogProcessor) (*AntsWorkerPool, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建ANTS协程池
	pool, err := ants.NewPool(
		workers,
		ants.WithPreAlloc(true),    // 预分配协程
		ants.WithNonblocking(true), // 非阻塞模式
		ants.WithPanicHandler(func(i interface{}) {
			fmt.Printf("协程池发生panic: %v", i)

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

// 启动工作池
func (wp *AntsWorkerPool) Start() {
	// ANTS协程池已经自动启动
}

// 停止工作池
func (wp *AntsWorkerPool) Stop() {
	wp.cancel()
	wp.pool.Release()
}

// 添加任务到工作池
func (wp *AntsWorkerPool) AddJob(logParts format.LogParts) {
	select {
	case <-wp.ctx.Done():
		// 上下文已取消，不再接受新任务
		return
	default:
		// 提交任务到ANTS协程池
		err := wp.pool.Submit(func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("处理日志时发生panic: %v", r)

				}
			}()

			if err := wp.processor.Process(logParts); err != nil {
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

// 获取已处理的日志总数
func (wp *AntsWorkerPool) GetTotalCount() int64 {
	return atomic.LoadInt64(&wp.totalCount)
}

// 获取协程池状态
func (wp *AntsWorkerPool) Status() string {
	return fmt.Sprintf("协程数: %d, 运行中: %d, 等待任务: %d, 完成任务: %d \n",

		wp.pool.Cap(), wp.pool.Running(), wp.pool.Waiting(), wp.pool.Cap()-wp.pool.Running())
}

func main() {
	// 获取系统CPU核心数
	cpuNum := runtime.NumCPU()
	fmt.Printf("检测到 %d 个CPU核心 \n", cpuNum)

	// 创建一个 syslog 服务器实例
	server := syslog.NewServer()

	// 设置服务器解析的日志格式（例如 RFC3164）
	server.SetFormat(syslog.RFC3164)

	// 创建一个缓冲通道来接收处理后的日志，提高吞吐量
	channel := make(syslog.LogPartsChannel, 10000) // 增加缓冲区大小
	handler := syslog.NewChannelHandler(channel)
	server.SetHandler(handler)

	// 配置服务器监听 TCP 和 UDP 端口
	server.ListenTCP("0.0.0.0:1514")
	server.ListenUDP("0.0.0.0:1514")

	// 启动服务器
	server.Boot()

	// 创建日志处理器
	processor := &DefaultLogProcessor{}

	// 创建ANTS工作池，工作协程数为CPU核心数的2倍
	workerCount := cpuNum * 2
	pool, err := NewAntsWorkerPool(workerCount, processor)
	if err != nil {
		fmt.Printf("创建工作池失败: %v", err)

		return
	}
	pool.Start()

	fmt.Printf("已启动 %d 个工作协程处理日志 \n", workerCount)

	// 启动统计协程，每5秒打印一次统计信息
	go func(pool *AntsWorkerPool) {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		var lastCount int64
		for {
			select {
			case <-ticker.C:
				currentCount := pool.GetTotalCount()
				increment := currentCount - lastCount
				fmt.Printf("已处理总日志数: %d, 最近5秒处理: %d, %s \n",

					currentCount, increment, pool.Status())
				lastCount = currentCount
			}
		}
	}(pool)

	// 启动一个 goroutine 从通道读取日志并分发到工作池
	go func(channel syslog.LogPartsChannel, pool *AntsWorkerPool) {
		for logParts := range channel {
			pool.AddJob(logParts)
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

	fmt.Printf("程序已关闭，共处理了 %d 条日志 \n", pool.GetTotalCount())

}
