package main

import (
	"fmt"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// SyslogInput 处理syslog输入
type SyslogInput struct {
	syslogConfig *SyslogConfig
	CustomRegexp []*regexp.Regexp
	server       *syslog.Server
	stopSignal   chan struct{}
	stopFlag     *StopFlag
	dnsServer    *interface{} // 这里应该是实际的DNS服务器类型

	configPath string
	reload     time.Duration
	mtime      time.Time

	processor *DefaultLogProcessor
}

// StopFlag 用于控制停止标志
type StopFlag struct {
	flag bool
}

// NewSyslogInput 创建一个新的SyslogInput实例
func NewSyslogInput(config *SyslogConfig) *SyslogInput {
	return &SyslogInput{
		syslogConfig: config,
		stopFlag:     &StopFlag{flag: false},
	}
}

// contains 检查字符串是否在数组中
func contains(arr []string, str string) bool {
	for _, v := range arr {
		if strings.ToUpper(v) == strings.ToUpper(str) {
			return true
		}
	}
	return false
}

// SyslogDoCapture 开始捕获syslog消息
func (s *SyslogInput) SyslogDoCapture(stop *StopFlag) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in f: %v", r)
		}
	}()

	// 设置信号监听，用于优雅关闭程序
	sigChan := make(chan os.Signal, 1)

	if stop.flag == true {
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		return
	}

	s.CustomRegexp = []*regexp.Regexp{}

	for _, v := range s.syslogConfig.Regexp {
		log.Println(v)
		s.CustomRegexp = append(s.CustomRegexp, regexp.MustCompile(v))
	}

	// 获取系统CPU核心数
	cpuNum := runtime.NumCPU()
	fmt.Printf("检测到 %d 个CPU核心 \n", cpuNum)

	// 创建一个 syslog 服务器实例
	server := syslog.NewServer()
	server.SetFormat(syslog.RFC3164)

	// 使用配置中的worker数量
	workerCount := s.syslogConfig.Worker
	if workerCount <= 0 {
		workerCount = cpuNum * 2
	}
	bufferSize := workerCount * 5000
	channel := make(syslog.LogPartsChannel, bufferSize)
	handler := syslog.NewChannelHandler(channel)
	server.SetHandler(handler)

	proto := ""
	var listenErr error

	if contains(s.syslogConfig.Proto, "UDP") {
		proto += " UDP"
		// 解析地址
		udpAddr, err := net.ResolveUDPAddr("udp", s.syslogConfig.Addr)
		if err != nil {
			log.Printf("解析UDP地址失败: %v", err)
			time.Sleep(30 * time.Second)
			return
		}
		listenErr = server.ListenUDP(udpAddr.String())
	}
	if contains(s.syslogConfig.Proto, "TCP") {
		proto += " TCP"
		if listenErr == nil { // 只有UDP没出错才继续TCP
			// 解析地址
			tcpAddr, err := net.ResolveTCPAddr("tcp", s.syslogConfig.Addr)
			if err != nil {
				log.Printf("解析TCP地址失败: %v", err)
				time.Sleep(30 * time.Second)
				return
			}
			listenErr = server.ListenTCP(tcpAddr.String())
		}
	}

	if listenErr != nil {
		log.Println(listenErr)
		time.Sleep(30 * time.Second)
		return
	}

	if err := server.Boot(); err != nil {
		log.Println(err)
		time.Sleep(30 * time.Second)
		return
	}

	if proto != "" {
		log.Printf("syslog server start at %s%s", s.syslogConfig.Addr, proto)
	} else {
		log.Println("syslog server: no proto config")
	}
	s.server = server

	// 创建日志处理器
	batchSize := GetBatchSize()
	timeout := GetTimeout()
	processor := NewDefaultLogProcessor(batchSize, timeout, s)

	// 创建ANTS工作池
	pool, err := NewAntsWorkerPool(workerCount, processor)
	if err != nil {
		fmt.Printf("创建工作池失败: %v", err)
		return
	}
	pool.Start()

	fmt.Printf("已启动 %d 个工作协程处理日志 \n", workerCount)

	// 启动统计协程
	go func(pool *AntsWorkerPool) {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		var lastCount int64
		for {
			select {
			case <-ticker.C:
				pool.AdjustPoolSize() // 动态调整协程池大小
				currentCount := pool.GetTotalCount()
				increment := currentCount - lastCount
				metrics := pool.GetMetrics()
				log.Printf("已处理总日志数: %d, 最近5秒处理: %d, 错误数: %d, %s \n",
					currentCount, increment, metrics.ErrorCount, pool.Status())
				lastCount = currentCount
			}
		}
	}(pool)

	// 修改分发协程数量为CPU核心数
	fmt.Printf("启动 %d 个分发协程处理日志 \n", cpuNum)

	// 使用WaitGroup确保所有分发协程都能正确启动和关闭
	var wg sync.WaitGroup
	wg.Add(cpuNum)

	for i := 0; i < cpuNum; i++ {
		go func(id int, channel syslog.LogPartsChannel, pool *AntsWorkerPool) {
			defer wg.Done()
			fmt.Printf("分发协程 %d 已启动 \n", id)

			for logParts := range channel {
				if err := pool.AddJobWithBackpressure(logParts); err != nil {
					// 记录错误但继续处理下一条日志
					atomic.AddInt64(&pool.errorCount, 1)
					fmt.Printf("分发协程 %d 添加任务失败: %v", id, err)
				}
			}

			fmt.Printf("分发协程 %d 已退出", id)
		}(i, channel, pool)
	}

	// 添加一个goroutine来优雅地关闭分发协程
	go func() {
		<-sigChan
		fmt.Println("正在关闭分发协程...")
		// 关闭通道会导致所有分发协程退出循环
		close(channel)
		// 等待所有分发协程完成
		wg.Wait()
		fmt.Println("所有分发协程已关闭")
	}()

	server.Wait()

	//服务结束关闭
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// 等待关闭信号
	<-sigChan
	fmt.Println("收到关闭信号，正在优雅关闭...")

	// 停止接收新日志
	server.Kill()

	// 停止工作池
	pool.Stop()

	fmt.Printf("程序已关闭，共处理了 %d 条日志", pool.GetTotalCount())
}

// HandleBatch 实现LogBatchHandler接口
func (s *SyslogInput) HandleBatch(logs []format.LogParts) error {
	return s.ProcessBatch(logs)
}

// ProcessBatch 处理批次日志
func (s *SyslogInput) ProcessBatch(logs []format.LogParts) error {
	for _, logp := range logs {
		tag := logp["tag"].(string)
		content := logp["content"].(string)

		if strings.Contains(tag, "360sdns") == false &&
			strings.Contains(tag, "360dns") == false { //避免循环写爆本地日志
			log.Println("|tag=" + tag + "|content=" + content)
		}
		// TODO 处理单条日志
	}
	return nil
}
