package main

import "fmt"

var ch chan int

func f() {
	fmt.Println("blah")
	g()
}

func g() {
	<-ch
	fmt.Println("blah-g")
}

func main() {
	fmt.Println("before")
	ch = make(chan int)
	f()
	g()
	x := func() <-chan int { return make(chan int) }
	select {
	case <-x():
	}
	fmt.Println("after")
}
