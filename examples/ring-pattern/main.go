package main

import "fmt"

func numprocs() int {
	return 10
}

func adder(in <-chan int, out chan<- int) {
	for {
		out <- (<-in + 1)
	}
}

func main() {
	chOne := make(chan int)
	chOut := chOne
	chIn := chOne
	for i := 0; i < numprocs(); i++ {
		chOut = make(chan int)
		go adder(chIn, chOut)
		chIn = chOut
	}
	chOne <- 0
	fmt.Println(<-chOut)
}
