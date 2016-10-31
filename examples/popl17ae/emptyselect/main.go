package main

func s(ch chan int) {
	ch <- 5
}

func main() {
	ch := make(chan int, 2)
	select {
	case <-ch:
	default:
	}
	s(ch)
}
