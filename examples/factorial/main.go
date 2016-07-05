package main

import "fmt"

func main() {
	ch := make(chan int)
	go fact(5, ch)
	fmt.Println(<-ch)
}

func fact(n int, results chan<- int) {
	if n <= 1 {
		results <- n
		return
	}
	ch := make(chan int)
	go fact(n-1, ch)
	results <- n * <-ch
}
