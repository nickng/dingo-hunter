package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

func S(out chan int) {
	out <- 42
	fmt.Println("Sent 42")
}

func R(in chan int) {
	fmt.Printf("Received %d\n", <-in)
}

// Natural branch
func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)

	flag.Parse()
	fmt.Printf("NArg=%d\n", flag.NArg())
	if flag.NArg() > 0 {
		s := flag.Arg(0)
		i, err := strconv.Atoi(s)
		if err != nil {
			os.Exit(2)
		}

		if i > 0 {
			fmt.Println("Branch one")
			go R(ch2)
			go S(ch2)
		} else {
			fmt.Println("Branch two")
			go R(ch1)
			go S(ch1)
		}
		time.Sleep(1 * time.Second)
	}
}
