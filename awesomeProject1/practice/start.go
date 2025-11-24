package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/panjf2000/ants/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
	"log"
	"net/http"
	_ "net/http/pprof" // 自动注册pprof路由
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

	processor *DefaultLogProcessor
}

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
	// 定义命令行参数
	addr := flag.String("addr", "0.0.0.0", "监听地址")
	port := flag.Int("port", 1514, "监听端口")
	proto := flag.String("proto", "UDP,TCP", "监听协议，多个协议用逗号分隔")
	worker := flag.Int("worker", 0, "工作协程数量，0表示自动根据CPU核心数计算")
	pprofPort := flag.String("pprof", "6060", "pprof监听端口")
	batchSize := flag.Int("batchSize", 5000, "批处理大小")
	timeout := flag.Int("timeout", 100, "批处理超时时间(毫秒)")
	timeLayout := flag.String("timeLayout", "", "时间格式")
	timeLocation := flag.String("timeLocation", "Asia/Shanghai", "时区")

	// 解析命令行参数
	flag.Parse()

	// 启动一个HTTP服务器，用于pprof
	go func() {
		log.Println(http.ListenAndServe("localhost:"+*pprofPort, nil))
	}()

	// 解析协议列表
	protoList := strings.Split(*proto, ",")
	for i, p := range protoList {
		protoList[i] = strings.TrimSpace(p)
	}

	// 获取CPU核心数
	cpuNum := runtime.NumCPU()
	// 如果没有指定worker数量，则根据CPU核心数自动计算
	workerCount := *worker
	if workerCount <= 0 {
		workerCount = cpuNum * 10
	}

	syslogInput := &syslogInput{
		syslogConfig: &syslogConfig{
			Addr:         *addr + ":" + strconv.Itoa(*port),
			Port:         *port,
			Proto:        protoList,
			Worker:       workerCount,
			TimeLayout:   *timeLayout,
			TimeLocation: *timeLocation,
		},
		stopFlag: &stopFlag{flag: false},
	}

	// 打印配置信息
	log.Printf("启动配置: 地址=%s, 协议=%v, 工作协程数=%d, 批处理大小=%d, 超时=%dms",
		syslogInput.syslogConfig.Addr,
		syslogInput.syslogConfig.Proto,
		workerCount,
		*batchSize,
		*timeout)

	syslogInput.SyslogDoCapture(syslogInput.stopFlag)
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
	fmt.Printf("检测到 %d 个CPU核心\n", cpuNum)

	// 创建一个 syslog 服务器实例
	server := syslog.NewServer()
	server.SetFormat(syslog.RFC3164)

	// 使用配置中的worker数量
	workerCount := s.syslogConfig.Worker
	if workerCount <= 0 {
		workerCount = cpuNum * 10
	}
	bufferSize := workerCount * 5000
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
	// 使用命令行参数
	batchSize := 5000
	timeout := time.Millisecond * 100
	// 从命令行参数获取批处理大小和超时时间
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "batchSize":
			if val, err := strconv.Atoi(f.Value.String()); err == nil {
				batchSize = val
			}
		case "timeout":
			if val, err := strconv.Atoi(f.Value.String()); err == nil {
				timeout = time.Duration(val) * time.Millisecond
			}
		}
	})
	processor := NewDefaultLogProcessor(batchSize, timeout, s)

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

	//// 启动一个 goroutine 从通道读取日志并分发到工作池
	//go func(channel syslog.LogPartsChannel, pool *AntsWorkerPool) {
	//	for logParts := range channel {
	//		if err := pool.AddJobWithBackpressure(logParts); err != nil {
	//			fmt.Printf("添加任务失败: %v\n", err)
	//		}
	//	}
	//}(channel, pool)
	// 修改分发协程数量为CPU核心数
	fmt.Printf("启动 %d 个分发协程处理日志\n", cpuNum)

	// 使用WaitGroup确保所有分发协程都能正确启动和关闭
	var wg sync.WaitGroup
	wg.Add(cpuNum)

	for i := 0; i < cpuNum; i++ {
		go func(id int, channel syslog.LogPartsChannel, pool *AntsWorkerPool) {
			defer wg.Done()
			fmt.Printf("分发协程 %d 已启动\n", id)

			for logParts := range channel {
				if err := pool.AddJobWithBackpressure(logParts); err != nil {
					// 记录错误但继续处理下一条日志
					atomic.AddInt64(&pool.errorCount, 1)
					fmt.Printf("分发协程 %d 添加任务失败: %v\n", id, err)
				}
			}

			fmt.Printf("分发协程 %d 已退出\n", id)
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

	fmt.Printf("程序已关闭，共处理了 %d 条日志\n", pool.GetTotalCount())
}

func NewDefaultLogProcessor(batchSize int, batchTimeout time.Duration, handler LogBatchHandler) *DefaultLogProcessor {
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
		handler:   handler,
	}
}

/**
metrics
*/

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

const pluginName = "syslog"

var (
	allowCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: pluginName,
		Name:      "total",
	}, []string{"type"})
	dropCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: pluginName,
		Name:      "drop_total",
	}, []string{"type"})
)

/**
logProcessor
*/

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
	handler      LogBatchHandler // 添加批处理回调处理器
}

// 处理批次数据
func (p *DefaultLogProcessor) processBatch(logs []format.LogParts) error {
	if p.handler != nil {
		return p.handler.HandleBatch(logs)
	}
	return fmt.Errorf("no batch handler configured")
}

/**
批处理
*/

// 批量日志处理器接口
type BatchLogProcessor interface {
	ProcessBatch(logs []format.LogParts) error
}

// 日志批处理回调接口
type LogBatchHandler interface {
	HandleBatch(logs []format.LogParts) error
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
	// 注意：这里需要外部传入处理函数，或者改为返回批次让外部处理
}

func (s *syslogInput) processBatch(logs []format.LogParts) {
	start := time.Now()
	if err := s.ProcessBatch(logs); err != nil {
		atomic.AddInt64(&s.processor.errorCount, 1)
		fmt.Printf("批量处理日志出错: %v\n", err)
	}
	// 可以在这里记录处理时间等指标
	_ = time.Since(start)
}

// 实现LogBatchHandler接口
func (s *syslogInput) HandleBatch(logs []format.LogParts) error {
	return s.ProcessBatch(logs)
}

// 处理批次日志
func (s *syslogInput) ProcessBatch(logs []format.LogParts) error {
	//parse := syslogParse.New()

	//if len(s.syslogConfig.TimeLayout) > 0 {
	//	loc := "Asia/Shanghai"
	//	if len(s.syslogConfig.TimeLocation) > 0 {
	//		loc = s.syslogConfig.TimeLocation
	//	}
	//	err := parse.SetTimeLayOut(s.syslogConfig.TimeLayout, loc)
	//	if err != nil {
	//		log.Println(err)
	//	}
	//}
	for _, logp := range logs {
		//  TODO 处理单条日志
		//var err error
		//var pb *dns360protocol.DnsMessage
		//matchFlag := false
		tag := logp["tag"].(string)
		content := logp["content"].(string)
		//client := logParts["client"].(string)
		if strings.Contains(tag, "360sdns") == false &&
			strings.Contains(tag, "360dns") == false { //避免循环写爆本地日志
			//log.("|tag=" + tag + "|content=" + content)
			log.Println("|tag=" + tag + "|content=" + content)
		}

		//for _, exp := range s.CustomRegexp {
		//	pb, err = parse.ParseRegexp(exp, content)
		//	if err == nil {
		//		matchFlag = true
		//		break
		//	}
		//}

		//if matchFlag {
		//	if s.dnsServer.XDNSServerIns != nil && pb != nil {
		//		allowCount.WithLabelValues(tag).Add(1)
		//		s.dnsServer.XDNSServerIns.ServeProtobuf(pb)
		//	} else {
		//		log.Printf("server_nil: tag=%s content=%s", tag, content)
		//		dropCount.WithLabelValues("server_nil").Add(1)
		//	}
		//} else {
		//	if len(s.CustomRegexp) > 0 {
		//		log.Printf("not_match: tag=%s |content=%s", tag, content)
		//		dropCount.WithLabelValues("not_match").Add(1)
		//	} else {
		//		log.Printf("rule_is_empty: tag=%s content=%s", tag, content)
		//		dropCount.WithLabelValues("rule_is_empty").Add(1)
		//	}
		//}
		//TODO 处理单条日志
		//fmt.Printf("处理日志: %+v\n", logp)
	}
	return nil
}

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

/**
ans
*/

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
		//ants.WithNonblocking(true),
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
// 移除单个任务的背压检查，改为批量检查
func (wp *AntsWorkerPool) AddJobBatch(logParts []format.LogParts) error {
	if wp.pool.Waiting() > wp.pool.Cap()*2 {
		return fmt.Errorf("处理队列过载，暂时拒绝新任务")
	}
	for _, log := range logParts {
		wp.AddJob(log)
	}
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

// 添加任务到工作池（带背压控制）
func (wp *AntsWorkerPool) AddJobWithBackpressure(logParts format.LogParts) error {
	// 检查当前等待队列长度，如果超过容量的80%，返回错误
	if wp.pool.Waiting() > wp.pool.Cap()*8/10 {
		return fmt.Errorf("处理队列过载，暂时拒绝新任务")
	}

	// 提交任务到协程池
	err := wp.pool.Submit(func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("处理日志时发生panic: %v\n", r)
				atomic.AddInt64(&wp.errorCount, 1)
			}
		}()

		if err := wp.processor.Process(logParts); err != nil {
			atomic.AddInt64(&wp.errorCount, 1)
			fmt.Printf("处理日志出错: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("提交任务到协程池失败: %v", err)
	}

	atomic.AddInt64(&wp.totalCount, 1)
	return nil
}
