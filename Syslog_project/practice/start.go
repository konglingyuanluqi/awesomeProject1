package main

import (
	"fmt"
	"github.com/coredns/coredns/core/dnsserver"
	"log"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type syslogInput struct {
	syslogConfig *syslogConfig
	CustomRegexp []*regexp.Regexp
	server       *syslog.Server
	stopSignal   chan struct{}
	stopFlag     *stopFlag
	dnsServer    *dnsserver.Config

	configPath string
	reload     time.Duration
	mtime      time.Time
}

var bindRegexp = regexp.MustCompile(`(?P<datetime>.+) queries: info: client .+ (?P<client_ip>.+)#.+query: (?P<query_name>.+) (?P<query_class>\w+) (?P<query_type>\w+)`)
var unboundRegexp = regexp.MustCompile(`info: (?P<client_ip>.+) (?P<query_name>.+) (?P<query_type>\w+) (?P<query_class>\w+)`)
var huaYuRegexp = regexp.MustCompile(`.+ .+ (?P<client_ip>.+)#.+ .+ .+ (?P<query_name>.+) (?P<query_class>\w+) (?P<query_type>\w+) .+`)
var zdnsRegexp = regexp.MustCompile(`\w+ (?P<datetime>.+) client (?P<client_ip>.+) (?P<client_port>.+): view .+: (?P<query_name>.+) IN (?P<query_type>\w+) (?P<rcode>\w+) .+`)

type syslogConfig struct {
	Addr         string
	Port         int
	Proto        []string
	Worker       int
	Disable      bool
	Regexp       []string
	TimeLayout   string
	TimeLocation string
}

type stopFlag struct {
	flag bool
}

func main() {
	syslogInput := &syslogInput{
		syslogConfig: &syslogConfig{
			Addr:  "0.0.0.0:1514",
			Port:  1514,
			Proto: []string{"UDP", "TCP"},
		},
	}
	syslogInput.SyslogDoCapture(&stopFlag{flag: false})
}

func contains(arr []string, str string) bool {
	for _, v := range arr {
		if strings.ToUpper(v) == strings.ToUpper(str) {
			return true
		}
	}
	return false
}

func (s *syslogInput) SyslogDoCapture(stop *stopFlag) {

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in f", r)
		}
	}()

	if stop.flag == true {
		return
	}

	s.CustomRegexp = []*regexp.Regexp{}

	for _, v := range s.syslogConfig.Regexp {
		log.Println(v)
		s.CustomRegexp = append(s.CustomRegexp, regexp.MustCompile(v))
	}

	// 获取系统CPU核心数
	cpuNum := runtime.NumCPU()
	fmt.Printf("检测到 %d 个CPU核心\n", cpuNum)

	// 创建一个 syslog 服务器实例
	server := syslog.NewServer()
	server.SetFormat(syslog.RFC3164)

	// 动态计算缓冲区大小
	workerCount := cpuNum * 3
	bufferSize := workerCount * 1000
	channel := make(syslog.LogPartsChannel, bufferSize)
	handler := syslog.NewChannelHandler(channel)
	server.SetHandler(handler)

	proto := ""
	var listenErr error

	if contains(s.syslogConfig.Proto, "UDP") {
		proto += " UDP"
		listenErr = server.ListenUDP(s.syslogConfig.Addr)
	}
	if contains(s.syslogConfig.Proto, "TCP") {
		proto += " TCP"
		if listenErr == nil { // 只有UDP没出错才继续TCP
			listenErr = server.ListenTCP(s.syslogConfig.Addr)
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

	server.Wait()
	// 等待关闭信号
	<-sigChan
	fmt.Println("收到关闭信号，正在优雅关闭...")

	// 停止接收新日志
	server.Kill()

	// 停止工作池
	pool.Stop()

	fmt.Printf("程序已关闭，共处理了 %d 条日志\n", pool.GetTotalCount())
}
