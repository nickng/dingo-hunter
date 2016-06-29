package main

// Simplest way of disabling the deadlock detector.

import _ "net"

func main() {
	ch := make(chan int)
	<-ch
}
