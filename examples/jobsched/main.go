package main

import "fmt"

var i int

func worker(id int, jobQueue <-chan int) {
	for jobID := range jobQueue {
		fmt.Println(id, "Executing job", jobID)
	}
}

func morejob() bool {
	i++
	return i < 20
}

func main() {
	jobQueue := make(chan int, 10)
	go worker(1, jobQueue)
	go worker(2, jobQueue)

	for morejob() {
		jobQueue <- i
	}
	close(jobQueue)
}
