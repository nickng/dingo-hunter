package main

import (
	"fmt"
)

func work1(out chan<- int) {
	for {
		out <- 42
	}
}
func work2(out chan<- int) {
	for {
		out <- 42
	}
}

func fanin(ch1, ch2, c chan int) {
	go func(ch1, ch2, c chan int) {
		for {
			select {
			case s := <-ch1:
				c <- s
			case s := <-ch2:
				c <- s
			}
		}
	}(ch1, ch2, c)
	for {
		fmt.Println(<-c)
	}
}

func main() {
	input1 := make(chan int)
	input2 := make(chan int)
	go work1(input1)
	go work2(input2)
	c := make(chan int)
	fanin(input1, input2, c)
}
