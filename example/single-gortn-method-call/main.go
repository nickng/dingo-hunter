package main

var x = 1323

type T struct {
	x int
	y chan int
}

func (t *T) setup(y int) {
	t.x = y
	t.y = make(chan int)
}

func main() {
	var t T
	x := 12
	t.setup(x)
	t.y <- 42 // Nobody to receive!
}
