// Command parallel-twoprocess-fibonacci is an improved version of parallel
// fibonacci which limits to only spawning 2 goroutines.
package main

import "fmt"

func fib(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func fibParallel(n int, ch chan<- int) {
	ch <- fib(n)
}

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)
	n := 10
	go fibParallel(n-1, ch1)
	go fibParallel(n-2, ch2)

	fmt.Println(<-ch1 + <-ch2)
}
