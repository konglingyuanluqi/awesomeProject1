package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
	"strconv"
	"sync"
	"time"
)

func main() {

	var filePath, raddr, network, tag string
	var loop bool

	flag.StringVar(&filePath, "file", "syslog.txt", "syslog file path")
	flag.StringVar(&raddr, "addr", "localhost:1515", "remote server and port,example:10.19.63.3:514")
	flag.StringVar(&network, "network", "udp", "udp/tcp")
	flag.StringVar(&tag, "tag", "golang", "program name")
	flag.BoolVar(&loop, "loop", false, "loop send")

	flag.Parse()

	if len(raddr) == 0 {
		log.Fatal("addr is null")
		return
	}

	sysLog, err := syslog.Dial(network, raddr, syslog.LOG_INFO, tag)
	if err != nil {

		log.Fatal(err)
	}

	fi, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	defer fi.Close()

	br := bufio.NewReader(fi)

	var list []string

	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		//fmt.Println(string(a))
		list = append(list, string(a))

	}
	start := time.Now()
	fmt.Println(start)

	if loop {
		var wg sync.WaitGroup

		for i := 1; i < 10; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()
				sum := 0
				for sum < 10000 {
					for _, a := range list {
						err = sysLog.Emerg(string(a))
						if err != nil {
							fmt.Printf("Error: %s\n", err)
						} else {
							sum = sum + 1
						}
					}
					fmt.Println(sum)
				}
				fmt.Println("send:"+strconv.Itoa(sum), " ", time.Now().Sub(start))
				fmt.Println(time.Now())

			}()

		}
		wg.Wait()
		//time.Sleep(100 * time.Second)
	} else {
		sum := 0
		for _, a := range list {
			err = sysLog.Emerg(string(a))
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			} else {
				sum = sum + 1
			}
		}

		fmt.Println("send:"+strconv.Itoa(sum), " ", time.Now().Sub(start))
		fmt.Println(time.Now())

	}

	return
}
