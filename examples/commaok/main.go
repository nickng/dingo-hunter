package main

import (
	"fmt"
)

func main() {
	ch := make(chan int, 1)
	ch <- 1
	y := make(map[int]int)
	if x, ok := <-ch; ok {
		y[x] = 1
		if z, ok := y[1]; ok {
			fmt.Println(z)
		}
	}
}
