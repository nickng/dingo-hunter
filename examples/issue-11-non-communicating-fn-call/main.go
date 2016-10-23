package main

// Issue #11, when a function is empty, all 'call' on that def in a migo file
// should be scrubbed.

func main() {
	x := make(chan bool)
	go func() { x <- true }()
	if true {
		<-x
	}
}
