package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
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
	go sendLogsPeriodically(ctx, logger, 3*time.Second)

	// 等待信号
	<-sigChan
	logger.Info("收到关闭信号，正在优雅关闭...")
	cancel()
	time.Sleep(1 * time.Second)
	logger.Info("程序已关闭")

}

// 读取日志文件
func readSyslog() {
	// 打开 Syslog 日志文件（不同系统路径可能不同）
	// Ubuntu/Debian: /var/log/syslog
	// CentOS/RHEL: /var/log/messages
	file, err := os.Open("/var/log/syslog")
	if err != nil {
		fmt.Println("无法打开日志文件:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// 读取日志文件内容
	for scanner.Scan() {
		line := scanner.Text()

		// 可以在这里添加过滤逻辑
		if strings.Contains(line, "日志") { // 过滤包含 "myapp" 标签的日志
			fmt.Println(line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("读取日志文件错误:", err)
	}
}

// 定期发送日志的函数
func sendLogsPeriodically(ctx context.Context, logger *syslog.Writer, interval time.Duration) {
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
