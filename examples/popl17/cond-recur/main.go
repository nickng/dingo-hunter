// Command conditional-recur has a recursion with conditional on one goroutine
// and another receiving until a done message is received.
package main

import "fmt"

func x(ch chan int, done chan struct{}) {
	i := 0
	for {
		if i < 3 {
			ch <- i
			fmt.Println("Sent", i)
			i++
		} else {
			done <- struct{}{}
			return
		}
	}
}

func main() {
	done := make(chan struct{})
	ch := make(chan int)
	go x(ch, done)
FINISH:
	for {
		select {
		case x := <-ch:
			fmt.Println(x)
		case <-done:
			break FINISH
		}
	}
}
