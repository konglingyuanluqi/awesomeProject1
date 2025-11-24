package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 添加命令行参数解析
	count := flag.Int("count", -1, "要发送的日志条数，-1表示持续发送")
	flag.Parse()

	// 连接到本地 Syslog 服务
	// 参数分别为：日志优先级（Facility|Severity）、标签（Tag）、日志选项
	logger, err := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, "")
	// 使用网络连接方式连接到指定端口的syslog服务
	//network := "udp"
	//raddr := "localhost:514"
	//logger, err := syslog.Dial(network, raddr, syslog.LOG_INFO|syslog.LOG_USER, "")

	if err != nil {
		log.Fatal("无法连接到 Syslog:", err)
	}

	defer logger.Close()

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号监听，用于优雅关闭程序
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动日志发送协程
	go sendLogsPeriodically(ctx, logger, 1*time.Millisecond, *count)

	// 等待信号
	<-sigChan
	logger.Info("收到关闭信号，正在优雅关闭...")
	cancel()
	time.Sleep(1 * time.Second)
	logger.Info("程序已关闭")
}

// 定期发送日志的函数
func sendLogsPeriodically(ctx context.Context, logger *syslog.Writer, interval time.Duration, maxCount int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sentCount := 0

	// 立即发送一次日志
	sendRandomLog(logger)
	sentCount++

	for {
		select {
		case <-ctx.Done():
			logger.Info("停止发送日志")
			return
		case <-ticker.C:
			if maxCount > 0 && sentCount >= maxCount {
				logger.Info("已达到指定日志条数，停止发送")
				return
			}
			sendRandomLog(logger)
			sentCount++
		}
	}
}

// 随机发送不同级别的日志
func sendRandomLog(logger *syslog.Writer) {
	// 获取当前时间戳
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// 随机选择日志级别
	logLevel := randInt(1, 4)

	switch logLevel {
	case 1:
		logger.Info(fmt.Sprintf("[%s] 系统运行正常，CPU使用率: %d%%", timestamp, randInt(20, 80)))
	case 2:
		logger.Warning(fmt.Sprintf("[%s] 内存使用率较高: %d%%，建议检查", timestamp, randInt(70, 95)))
	case 3:
		logger.Err(fmt.Sprintf("[%s] 检测到异常访问，IP: %s", timestamp, randomIP()))
	case 4:
		logger.Notice(fmt.Sprintf("[%s] 任务执行完成，耗时: %dms", timestamp, randInt(100, 2000)))
	}
}

// 生成随机整数
func randInt(min, max int) int {
	if min >= max {
		return min
	}
	return min + int(time.Now().UnixNano()%int64(max-min+1))
}

// 生成随机IP地址
func randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		randInt(1, 255),
		randInt(0, 255),
		randInt(0, 255),
		randInt(1, 254))
}
