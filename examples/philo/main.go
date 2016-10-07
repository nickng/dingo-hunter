package main

// philio test case from Stadtmuller, Thieman

import (
	"fmt"
)

func philo(id int, forks chan int) {
	for {
		<-forks
		<-forks
		fmt.Printf("%d eats\n", id)
		forks <- 1
		forks <- 1
	}
}

func main() {
	forks := make(chan int)
	go func() { forks <- 1 }()
	go func() { forks <- 1 }()
	go func() { forks <- 1 }()
	go philo(1, forks)
	go philo(2, forks)
	philo(3, forks)
}
