// +build OMIT

package main

import (
	"fmt"
	"time"
)

func deepThought(replyCh chan int) {
	time.Sleep(75 * time.Millisecond)
	replyCh <- 42 // HLsendrecv
}

func main() {
	ch := make(chan int) // HLmakechan
	go deepThought(ch)   // HLdl
	answer := <-ch       // HLsendrecv
	fmt.Printf("The answer is %d\n", answer)
}

// END OMIT
