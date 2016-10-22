package main

// Issue #10, when generating migo from programs that uses close, the channel
// name used is incorrect.

func main() {
	x := make(chan bool)
	close(x)
}
