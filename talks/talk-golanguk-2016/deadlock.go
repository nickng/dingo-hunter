// +build OMIT

package main

import (
	"fmt"
	"time"
)

// Anonymous goroutine: Some long running work (e.g. http service)
func Work() {
	for i := 0; ; i++ {
		fmt.Println("Working #", i)
		time.Sleep(1 * time.Second)
	}
}

// START OMIT
func Sender(ch chan<- int) {
	ch <- 42
}
func Receiver(ch <-chan int, done chan<- int) {
	done <- <-ch
}

func main() {
	ch := make(chan int)
	done := make(chan int)
	go Sender(ch)
	go Receiver(ch, done)
	go Receiver(ch, done) // Who is ch receiving from? // HLoops

	fmt.Println("Done 1:", <-done)
	fmt.Println("Done 2:", <-done) // HLoops
}

// END OMIT
