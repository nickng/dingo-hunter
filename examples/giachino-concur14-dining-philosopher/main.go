package main

// Example from CONCUR 14 paper by Giachino et al.
// doi: 10.1007/978-3-662-44584-6_6

import (
	"fmt"
	"time"
)

// Creates philosophers.
func phils(n int, fork1, fork2 chan bool) {
	if n == 1 {
		go phil(fork1, fork2, n)
	} else {
		fork3 := make(chan bool)
		go phil(fork3, fork2, n)
		go func() { fork3 <- true }()
		phils(n-1, fork1, fork3)
	}
}

// philosophers (infinite recursive).
func phil(fork1, fork2 chan bool, id int) {
	x1 := <-fork1 // take fork
	x2 := <-fork2 // take fork
	fmt.Printf("phil %d got both fork\n", id)
	time.Sleep(1 * time.Second)
	go func() { fork1 <- x1 }() // return fork
	go func() { fork2 <- x2 }() // return fork
	phil(fork1, fork2, id)
}

func main() {
	fork1 := make(chan bool)
	fork2 := make(chan bool)
	phils(3, fork1, fork2)
	go func() { fork1 <- true }() // give it a fork
	go func() { fork2 <- true }() // give it a fork
	go phil(fork1, fork2, 0)      // deadlock if phil(fork2, fork1, 0)
	time.Sleep(10 * time.Second)
}
