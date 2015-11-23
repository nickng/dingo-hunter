package main

import (
	"golang.org/x/tools/go/ssa"

	"github.com/nickng/dingo-hunter/sesstype"
)

type goroutine struct {
	role    sesstype.Role
	root    sesstype.Node
	leaf    *sesstype.Node
	visited map[*ssa.BasicBlock]sesstype.Node
}

// Append a session type node to current goroutine.
func (gortn *goroutine) AddNode(node sesstype.Node) {
	if gortn.leaf == nil {
		panic("AddNode: leaf cannot be nil")
	}

	newLeaf := (*gortn.leaf).Append(node)
	gortn.leaf = &newLeaf
}
