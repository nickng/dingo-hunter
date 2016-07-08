package main

// fanin pattern, using for-range loop to consume values (syntactic sugar of
// loop over r, ok := <-ch)

import (
	"fmt"
	"time"
)

func work(out chan<- int) {
	for {
		out <- 42
	}
}

func fanin(ch1, ch2 <-chan int) <-chan int {
	c := make(chan int)
	go func(ch1, ch2 <-chan int, c chan<- int) {
		for {
			select {
			case s := <-ch1:
				c <- s
			case s := <-ch2:
				c <- s
			default:
				close(c)
				return
			}
		}
	}(ch1, ch2, c)
	return c
}

func main() {
	input1 := make(chan int)
	input2 := make(chan int)
	go work(input1)
	go work(input2)
	time.Sleep(1 * time.Second)
	for v := range fanin(input1, input2) {
		fmt.Println(v)
	}
}
