package main

import (
	"fmt"
)

func main() {

	ch1 := make(chan int)
	ch2 := make(chan int)
	ch3 := make(chan int)

	select {
	case x := <-ch1:
		fmt.Println("Received x", x)
	case ch2 <- 43:
		fmt.Println("ok sent")
	case <-ch3:
	default:
		fmt.Println("asdfsdafsad")
	}
	ch1 <- 32
	fmt.Println("asdfsadfs")
}
