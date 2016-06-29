package main

// Semaphores.
// Emulating semaphores with buffered channels.
// http://www.golangpatterns.info/concurrency/semaphores

type empty struct{}
type Semaphore chan empty

func NewSemaphore(cap int) Semaphore {
	return make(Semaphore, cap)
}

// acquire n resources
func (s Semaphore) P(n int) {
	e := empty{}
	for i := 0; i < n; i++ {
		s <- e
	}

}

// release n resources
func (s Semaphore) V(n int) {
	for i := 0; i < n; i++ {
		<-s
	}

}

func main() {
}
