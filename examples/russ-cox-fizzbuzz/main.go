package main

import "fmt"

func main() {
	c := generate()
	c = filter(c, 3, "Fizz")
	c = filter(c, 5, "Buzz")
	for i := 1; i <= 100; i++ {
		if s := <-c; s != "" {
			fmt.Println(s)
		} else {
			fmt.Println(i)
		}
	}
}

func generate() <-chan string {
	c := make(chan string)
	go func() {
		for {
			c <- ""
		}
	}()
	return c
}

func filter(c <-chan string, n int, label string) <-chan string {
	out := make(chan string)
	go func() {
		for {
			for i := 0; i < n-1; i++ {
				out <- <-c
			}
			out <- <-c + label
		}
	}()
	return out
}

