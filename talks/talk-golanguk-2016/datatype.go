// +build OMIT

package main

import (
	"fmt"
)

func main() {
	x := 42
	fmt.Printf("Type of x is %T", x)
	x = "hello" // incompatible!
}
