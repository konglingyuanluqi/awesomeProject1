package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/mcuadros/go-syslog.v2"
)

func main() {
	// 创建带缓冲的通道以提高性能
	channel := make(syslog.LogPartsChannel, 1000)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC3164)
	server.SetHandler(handler)

	// 同时监听UDP和TCP端口，确保能接收到所有日志
	server.ListenUDP("localhost:1514")
	server.ListenTCP("localhost:1514")
	server.Boot()

	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 创建一个计数器
	var count int64

	// 启动一个goroutine来处理接收到的日志
	go func() {
		for logParts := range channel {
			count++

			// 解析并格式化日志内容
			timestamp, _ := logParts["timestamp"]
			content, _ := logParts["content"]
			priority, _ := logParts["priority"]
			version, _ := logParts["version"]
			appName, _ := logParts["app_name"]
			hostname, _ := logParts["hostname"]

			fmt.Printf("=== 日志 #%d ===", count)
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
		}
	}()

	// 启动统计协程，每10秒显示一次统计信息
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		lastCount := int64(0)
		for {
			select {
			case <-ticker.C:
				currentCount := count
				increment := currentCount - lastCount
				log.Printf("已接收日志总数: %d, 最近10秒新增: %d", currentCount, increment)
				lastCount = currentCount
			}
		}
	}()

	// 设置信号监听，用于优雅关闭程序
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("日志服务器已启动，监听端口 514，等待日志...")

	// 等待关闭信号
	<-sigChan
	log.Println("收到关闭信号，正在优雅关闭...")

	// 停止服务器
	server.Kill()

	// 等待一段时间让日志处理完成
	time.Sleep(1 * time.Second)

	log.Printf("服务器已关闭，共处理了 %d 条日志", count)
}
