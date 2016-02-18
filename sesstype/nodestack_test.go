package sesstype

import (
	"testing"
)

func TestNewStack(t *testing.T) {
	ns := NewNodeStack()
	ns.Push(NewLabelNode("TEST"))
	ns.Push(NewLabelNode("TEST2"))
	ns.Push(NewLabelNode("TEST3"))
	l := ns.Top()
	if l.String() != "TEST3" {
		t.Fail()
	}
	ns.Pop()
	l2 := ns.Top()
	if l2.String() != "TEST2" {
		t.Fail()
	}
	ns.Pop()
	l3 := ns.Top()
	if l3.String() != "TEST" {
		t.Fail()
	}
	ns.Pop()
}
