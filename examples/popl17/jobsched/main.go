package main

import (
	"fmt"
	"time"
)

var i int

func worker(id int, jobQueue <-chan int, done <-chan struct{}) {
	for {
		select {
		case jobID := <-jobQueue:
			fmt.Println(id, "Executing job", jobID)
		case <-done:
			fmt.Println(id, "Quits")
			return
		}
	}
}

func morejob() bool {
	i++
	return i < 20
}

func producer(q chan int, done chan struct{}) {
	for morejob() {
		q <- 42
	}
	close(done)
}

func main() {
	jobQueue := make(chan int)
	done := make(chan struct{})
	go worker(1, jobQueue, done)
	go worker(2, jobQueue, done)
	producer(jobQueue, done)
	time.Sleep(1 * time.Second)
}
