// +build OMIT

package main

import (
	"fmt"
	"math/rand"
	"time"
)

func calc(ch chan int) {
	rand.Seed(time.Now().Unix())
	val := rand.Intn(5)
	time.Sleep(time.Duration(val) * time.Microsecond)
	ch <- val
}

func main() {
	ch1, ch2 := make(chan int), make(chan int)
	go calc(ch1)
	go calc(ch2)
	select {
	case ans := <-ch1:
		fmt.Println("Answer from ch1: ", ans)
	case ans := <-ch2:
		fmt.Println("Answer from ch2: ", ans)
	}
}
