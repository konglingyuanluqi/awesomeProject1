package main

import (
	"log"
	"net/http"
	_ "net/http/pprof" // 自动注册pprof路由
)

func main() {
	// 解析命令行参数
	config := ParseFlags()

	//// 启动一个HTTP服务器，用于pprof
	go func() {
		pprofPort := GetPprofPort()
		log.Println(http.ListenAndServe("localhost:"+pprofPort, nil))
	}()

	// 创建SyslogInput实例
	syslogInput := NewSyslogInput(config)

	// 开始捕获syslog消息
	syslogInput.SyslogDoCapture(&StopFlag{flag: false})
}
