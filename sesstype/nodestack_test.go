package sesstype

import (
	"fmt"
	"testing"
)

func TestNewStack(t *testing.T) {
	ns := NewNodeStack()
	ns.Push(MkLabelNode("TEST"))
	ns.Push(MkLabelNode("TEST2"))
	ns.Push(MkLabelNode("TEST3"))
	fmt.Println(ns.String())
	l := ns.Top()
	if l.String() != "TEST3" {
		t.Fail()
	}
	ns.Pop()
	fmt.Println(ns.String())
	l2 := ns.Top()
	if l2.String() != "TEST2" {
		t.Fail()
	}
	ns.Pop()
	fmt.Println(ns.String())
	l3 := ns.Top()
	if l3.String() != "TEST" {
		t.Fail()
	}
	ns.Pop()
	fmt.Println(ns.String())
}
