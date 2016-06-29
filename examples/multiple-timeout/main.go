// Command multiple-timeout is an example which uses multiple branches of
// time.After.
//
// The main purpose is to test if the extracted local type can distinguish
// between two "externally created" channels (in time.After), and are
// initialised separately in the local graph.
package main

import "time"

func main() {
	ch := make(chan int, 1)
	go func(ch chan int) { time.Sleep(10 * time.Second); ch <- 42 }(ch)

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
	case <-time.After(4 * time.Second):
	}
}
