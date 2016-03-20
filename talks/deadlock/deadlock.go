// +build OMIT

package main

import (
	"fmt"
	"time"
)

var (
	result int
)

func Send(ch chan<- int)                     { ch <- 42 }             // Send
func RecvAck(ch <-chan int, done chan<- int) { v := <-ch; done <- v } // Recv then Send

func main() {
	ch, done := make(chan int), make(chan int)
	go Send(ch)
	go RecvAck(ch, done)
	go RecvAck(ch, done) // OOPS // HLoops

	// Anonymous goroutine: Some long running work (e.g. http service)
	go func() {
		for i := 0; i < 3; i++ {
			fmt.Println("Working #", i)
			time.Sleep(1 * time.Second)
		}
	}()
	result = <-done
	result = <-done // OOPS // HLoops
	fmt.Println("Result is ", result)
}
