// +build OMIT

package main

import (
	"fmt"
	"time"
)

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
	// Unreleated worker goroutine, keeps deadlock 'alive'
	go func() { // HLwork
		for i := 0; i < 2; i++ { // HLwork
			fmt.Println("Working #", i)        // HLwork
			time.Sleep(500 * time.Millisecond) // HLwork
		} // HLwork
		fmt.Println("-------- Worker finished --------") // HLwork
	}() // HLwork

	fmt.Println("Done 1:", <-done)
	fmt.Println("Done 2:", <-done) // HLoops
}
