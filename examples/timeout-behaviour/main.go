package main

import (
	"fmt"
	"time"
)

func main() {
	done := make(chan struct{})
	ch := make(chan int)
	go func(ch chan int, done chan struct{}) {
		time.Sleep(1 * time.Second)
		ch <- 42
		fmt.Println("Sent")
		done <- struct{}{}
	}(ch, done)
	select {
	case v := <-ch:
		fmt.Println("received value of", v)
	case <-time.After(1 * time.Second):
		fmt.Println("Timeout: spawn goroutine to cleanup")
		fmt.Println("value received after cleanup:", <-ch)
	case <-time.After(1 * time.Second):
		fmt.Println("Timeout2: spawn goroutine to cleanup")
		fmt.Println("value received after cleanup:", <-ch)
	}
	<-done
	fmt.Println("All Done")
}
