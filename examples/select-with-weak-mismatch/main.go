package main

// This example tests how select works. Note that ch1 is never selected.

import (
	"fmt"
)

func main() {
	ch0 := make(chan int)
	ch1 := make(chan int)

	go func() {
		ch0 <- 42
	}()

	// Blocking
	select {
	case x := <-ch0:
		fmt.Printf("Result is %d\n", x)
	case ch1 <- 2: // This is a mismatch, no receive on ch1
	}
}
