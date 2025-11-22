package main

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	sysloghook "github.com/sirupsen/logrus/hooks/syslog"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := logrus.New()

	logger.SetLevel(logrus.InfoLevel)

	hook, err := sysloghook.NewSyslogHook("",
		"",
		syslog.LOG_INFO|syslog.LOG_USER,
		"",
	)

	if err != nil {
		log.Fatalf("无法创建 Syslog Hook: %v", err)
	}
	logger.Hooks.Add(hook)

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号监听，用于优雅关闭程序
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动日志发送协程
	go sendLogsPeriodically(ctx, logger, 3*time.Second)

	// 等待信号
	<-sigChan
	logger.Info("收到关闭信号，正在优雅关闭...")
	cancel()
	time.Sleep(1 * time.Second)
	logger.Info("程序已关闭")
}

// 定期发送日志的函数
func sendLogsPeriodically(ctx context.Context, logger *logrus.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 立即发送一次日志
	sendRandomLog(logger)

	for {
		select {
		case <-ctx.Done():
			logger.Info("停止发送日志")
			return
		case <-ticker.C:
			sendRandomLog(logger)
		}
	}
}

// 随机发送不同级别的日志
func sendRandomLog(logger *logrus.Logger) {
	// 获取当前时间戳
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// 随机选择日志级别
	logLevel := randInt(1, 4)

	switch logLevel {
	case 1:
		logger.Infof("[%s] 系统运行正常，CPU使用率: %d%%", timestamp, randInt(20, 80))
	case 2:
		logger.Warnf("[%s] 内存使用率较高: %d%%，建议检查", timestamp, randInt(70, 95))
	case 3:
		logger.Errorf("[%s] 检测到异常访问，IP: %s", timestamp, randomIP())
	case 4:
		logger.Infof("[%s] 任务执行完成，耗时: %dms", timestamp, randInt(100, 2000))
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
