// Command parallel-recursive-fibonacci is a recursive fibonacci which spawns a
// new goroutine per fib call.
package main

import "fmt"

func main() {
	ch := make(chan int)
	go fib(10, ch)
	fmt.Println(<-ch)
}

func fib(n int, ch chan<- int) {
	if n <= 1 {
		ch <- n
		return
	}
	ch1 := make(chan int)
	ch2 := make(chan int)
	go fib(n-1, ch1)
	go fib(n-2, ch2)
	ch <- <-ch1 + <-ch2
}
