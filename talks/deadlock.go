// +build OMIT

package main

import (
	"fmt"
	"time"
)

func Send(ch chan<- int)                  { ch <- 42 }
func Recv(ch <-chan int, done chan<- int) { done <- <-ch }

func main() {
	ch, done := make(chan int), make(chan int)
	go Send(ch)
	go Recv(ch, done)
	go Recv(ch, done) // HL

	// Some long running work (e.g. http service)
	go func() {
		for i := 0; i < 3; i++ {
			fmt.Println("Working")
			time.Sleep(1 * time.Second)
		}
	}()
	<-done
	<-done
}
