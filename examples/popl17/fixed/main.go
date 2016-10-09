package main

import (
	"fmt"
	"time"
)

func Work() {
	for {
		fmt.Println("Working")
		time.Sleep(1 * time.Second)
	}
}

func Send(ch chan<- int)                  { ch <- 42 }
func Recv(ch <-chan int, done chan<- int) { done <- <-ch }

func main() {
	ch, done := make(chan int), make(chan int)
	go Send(ch)
	go Recv(ch, done)
	go Work()

	<-done
}
