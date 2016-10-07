package main

// This example test different loop and is used for checking loop SSA
// generation.

import "fmt"

func main() {
	/*
			xs := []int{1, 2, 3}
			for _, s := range xs {
				fmt.Println(s)
			}

			xs2 := [3]int{1, 2, 3}
			for _, s := range xs2 {
				fmt.Println(s)
			}

			for i := 0; i < 3; i++ {
				fmt.Println("xs[i]", xs[i])
			}

			for {
				fmt.Println("looooopppp one") // This executes once
				break
			}

		loopcond := func(i int) bool { return false }

		for k := 0; loopcond(k); k++ {
			fmt.Println("Loop k: ", k)
		}
	*/
	for i := 0; i < 3; i++ {
		for j := 0; j < 2; j++ {
			fmt.Printf("Index (%d, %d) ", i, j)
			x := make(chan int)
			<-x
		}
		fmt.Printf("ASBCD")
	}
	/*
		x := []int{1, 2, 3, 4}
		for i := range x { // Range loop (safe)
			fmt.Println(i)
		}

		ch := make(chan int)
		go func(ch chan int) { ch <- 42; close(ch) }(ch)
		for v := range ch {
			fmt.Println(v)
		}

		for {
			fmt.Println("Infinite looooopppp")
		}
	*/
}
