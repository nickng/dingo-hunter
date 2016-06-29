// The loop-chan example shows what will happen when a channel is created in a
// loop (and stored in slices). Pointer analysis identifies the channel
// operations with channels created inside the loop, but the loop indices are
// ignored and the analysis must make an assumption that the channels inside the
// slices are accessed in order.

package main

func main() {
	chans := make([]chan int, 5)
	for i := range chans {
		chans[i] = make(chan int, 1)
	}

	for _, ch := range chans {
		ch <- 42
	}

	for _, ch := range chans {
		<-ch
	}
}
