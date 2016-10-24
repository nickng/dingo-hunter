package migo_test

import (
	"testing"

	"github.com/nickng/dingo-hunter/migoextract/migo"
)

func TestStmtsStack(t *testing.T) {
	s := migo.NewStmtsStack()
	blk1 := []migo.Statement{&migo.SendStatement{Chan: "ch"}}
	blk2 := []migo.Statement{&migo.TauStatement{}}
	s.Push(blk1)
	if s.Size() != 1 {
		t.Errorf("push: failed (Size=%d, expects=1)", s.Size())
	}
	s.Push(blk2)
	if s.Size() != 2 {
		t.Errorf("push: failed (Size=%d, expects=2)", s.Size())
	}

	i := 1
	for !s.IsEmpty() {
		stmts, err := s.Pop()
		if err != nil {
			t.Error(err)
		}
		if i == 1 && stmts[0] != blk2[0] { // XXX Compare pointer
			t.Error("pop: Expecting stack to return blk2")
		}
		if i == 2 && stmts[0] != blk1[0] { // XXX Compare pointer
			t.Error("pop: Expecting stack to return blk1")
		}
		i--
	}
}
