// +build ignore
package main

import (
	"fmt"
)

func send(ch chan int) {
	ch <- 42 // HL
}

func main() {
	ch := make(chan int)
	go send(ch)
	fmt.Printf("Received %d\n", <-ch) // HL
}
