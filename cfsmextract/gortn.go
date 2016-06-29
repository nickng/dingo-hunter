package cfsmextract

import (
	"github.com/nickng/dingo-hunter/cfsmextract/sesstype"
	"golang.org/x/tools/go/ssa"
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
