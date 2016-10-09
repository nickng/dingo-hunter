package main

// Example from CONCUR 14 paper by Giachino et al.
// doi: 10.1007/978-3-662-44584-6_6

import (
	"fmt"
	"time"
)

func Fork(fork chan int) {
	for {
		fork <- 1
		<-fork
	}
}

// philosophers (infinite recursive).
func phil(fork1, fork2 chan int, id int) {
	var x1, x2 int
	for {
		select {
		case x1 = <-fork1:
			select {
			case x2 = <-fork2:
				fmt.Printf("phil %d got both fork\n", id)
				fork1 <- x1
				fork2 <- x2
			default:
				fork1 <- x1
			}
		case x1 = <-fork2:
			select {
			case x2 = <-fork1:
				fmt.Printf("phil %d got both fork\n", id)
				fork2 <- x1
				fork1 <- x2
			default:
				fork2 <- x1
			}
		}
	}
}

func main() {
	fork1 := make(chan int)
	fork2 := make(chan int)
	fork3 := make(chan int)
	go phil(fork1, fork2, 0) // deadlock if phil(fork2, fork1, 0)
	go phil(fork2, fork3, 1)
	go phil(fork3, fork1, 2)
	go Fork(fork1)
	go Fork(fork2)
	go Fork(fork3)
	time.Sleep(10 * time.Second)
}
