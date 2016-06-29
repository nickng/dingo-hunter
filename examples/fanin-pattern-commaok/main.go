package main

// fanin pattern, using for-range loop to consume values (syntactic sugar of
// loop over r, ok := <-ch)

import (
	"fmt"
)

func work(out chan<- int) {
	for {
		out <- 42
	}
}
func fanin(input1, input2 <-chan int) <-chan int {
	c := make(chan int)
	go func() {
		for {
			select {
			case s := <-input1:
				c <- s
			case s := <-input2:
				c <- s
			default:
				close(c)
				return
			}
		}
	}()
	return c
}

func main() {
	input1 := make(chan int)
	input2 := make(chan int)
	go work(input1)
	go work(input2)
	for c := range fanin(input1, input2) {
		fmt.Println(c)
	}
}
