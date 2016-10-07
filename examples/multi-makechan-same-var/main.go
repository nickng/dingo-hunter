// createchan is an example which reuses a channel for different operations in
// an expanded form.

package main

func createChan() chan int {
	return make(chan int, 1)
}

func main() {
	ch := createChan()
	ch <- 42

	ch = createChan()
	ch <- 3
}
