package main

import "fmt"

// Example from CONCUR 14 paper by Giachino et al.
// doi: 10.1007/978-3-662-44584-6_6

func fact(n int, r, s chan int) {
	if n == 0 {
		m := <-r
		s <- m
		return
	}
	t := make(chan int)
	go fact(n-1, t, s)
	m := <-r
	t <- m * n
}

func main() {
	accumulated, result := make(chan int), make(chan int)
	go fact(3, accumulated, result)
	accumulated <- 1
	fmt.Println(<-result)
}
