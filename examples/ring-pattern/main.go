package main

import "fmt"

func main() {
	chOne := make(chan int)
	var chIn, chOut chan int
	chIn = chOne
	for i := 0; i < 10; i++ {
		chOut = make(chan int)
		go func(in, out chan int) {
			out <- (<-in + 1)
		}(chIn, chOut)
		chIn = chOut
	}
	chOne <- 0
	fmt.Println(<-chOut)
}
