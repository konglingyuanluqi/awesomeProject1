package main

import (
	"fmt"
	"gopkg.in/mcuadros/go-syslog.v2"
	"time"
)

func main() {
	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC3164)
	server.SetHandler(handler)
	server.ListenUDP("localhost:1514")

	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			fmt.Println(logParts)
		}
	}(channel)

	server.Boot()

	server.Wait()

	time.Sleep(time.Minute * 10)
}
