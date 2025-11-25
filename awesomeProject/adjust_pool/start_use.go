package main

import (
	"fmt"
	"github.com/panjf2000/ants/v2"
	"log"
)

func main() {
	pool, err := ants.NewPool(100,
		ants.WithPanicHandler(func(i interface{}) {
			fmt.Printf("panic: %v\n", i)
		}))
	if err != nil {
		log.Fatalln(err)
	}
	defer pool.Release()

	//	打印携程参数
	printInfo(pool)
	//加入任务
	for i := 0; i < 100; i++ {
		err := pool.Submit(func() {
			fmt.Println("hello world")
		})
		if err != nil {
			fmt.Println(err)
		}
	}

	//	动态扩容
	pool.Tune(200)
	printInfo(pool)
	//	动态缩容
	pool.Tune(50)
	printInfo(pool)
}

func printInfo(pool *ants.Pool) {
	fmt.Println("===== 协程池状态信息 =====")
	fmt.Printf("当前运行中的协程数: %d\n", pool.Running())
	fmt.Printf("空闲协程数: %d\n", pool.Free())
	fmt.Printf("协程池总容量: %d\n", pool.Cap())
	fmt.Printf("协程池使用率: %.2f%%\n", float64(pool.Running())/float64(pool.Cap())*100)
	fmt.Printf("协程池空闲率: %.2f%%\n", float64(pool.Free())/float64(pool.Cap())*100)
	fmt.Println("=========================")
}
