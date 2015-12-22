package main

import (
	"time"
)

type Interacter interface {
	Send(ch chan int)
	Recv(ch chan int)
}

type S struct{}

func (st S) Send(ch chan int) {
	ch <- 42
}

func (st S) Recv(ch chan int) {
	<-ch
}

func main() {
	x := S{}
	c := make(chan int)
	go x.Send(c)
	x.Recv(c)
	time.Sleep(1 * time.Second)
}
