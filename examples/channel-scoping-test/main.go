// The reentry example targets multiple use of a function with channel created
// inside. The channel is anchored in the function, so multiple calls of the
// function will use different version of the channel(s). Combined with loop
// indices assumptions this will be inaccurate.

package main

func main() {
	ch := makenew()
	for i := 0; i < 2; i++ {
		ch2 := makenew()
		ch2 <- 22
	}
	ch <- 42
}

func makenew() chan int {
	return make(chan int, 1)
}
