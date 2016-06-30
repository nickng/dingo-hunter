package main

import (
	"fmt"
)

func work(out chan<- int) {
	for {
		out <- 42
	}
}

func fanin(ch1, ch2 <-chan int) <-chan int {
	c := make(chan int)
	go func() {
		for {
			select {
			case s := <-ch1:
				c <- s
			case s := <-ch2:
				c <- s
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
	c := fanin(input1, input2)
	for {
		fmt.Println(<-c)
	}
}
