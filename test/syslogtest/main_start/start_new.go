package main

import (
	"log"
	"net/http"
	_ "net/http/pprof" // 自动注册pprof路由
	"syslogtest/practice"
)

func main() {
	// 解析命令行参数
	config := practice.ParseFlags()

	//// 启动一个HTTP服务器，用于pprof
	go func() {
		pprofPort := practice.GetPprofPort()
		log.Println(http.ListenAndServe("localhost:"+pprofPort, nil))
	}()

	// 创建SyslogInput实例
	syslogInput := practice.NewSyslogInput(config)

	// 开始捕获syslog消息
	syslogInput.SyslogDoCapture()
}
