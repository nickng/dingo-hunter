// Command nodet-for-select is a for-select pattern between two compatible
// recursive select.
package main

import "fmt"

func sel1(ch1, ch2 chan int, done chan struct{}) {
	select {
	case <-ch1:
		fmt.Println("sel1: recv")
		done <- struct{}{}
	case ch2 <- 1:
		fmt.Println("sel1: send")
		sel1(ch1, ch2, done)
	}
}

func sel2(ch1, ch2 chan int, done chan struct{}) {
	select {
	case <-ch2:
		fmt.Println("sel2: recv")
		sel2(ch1, ch2, done)
	case ch1 <- 2:
		fmt.Println("sel2: send")
		done <- struct{}{}
	}
}

func main() {
	done := make(chan struct{})
	a := make(chan int)
	b := make(chan int)
	go sel1(a, b, done)
	go sel2(a, b, done)

	<-done
	<-done
}
