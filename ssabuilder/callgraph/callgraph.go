// Package callgraph represents function call graph.
//
package callgraph // import "github.com/nickng/dingo-hunter/ssabuilder/callgraph"

import (
	"golang.org/x/tools/go/ssa"
)

type Node struct {
	Func     *ssa.Function
	Children []*Node
}

var (
	visitedFunc  map[*ssa.Function]bool
	visitedBlock map[*ssa.BasicBlock]bool
)

func Build(main *ssa.Function) *Node {
	root := &Node{
		Func:     main,
		Children: []*Node{},
	}
	visitedFunc = make(map[*ssa.Function]bool)
	visitedBlock = make(map[*ssa.BasicBlock]bool)
	visitedFunc[root.Func] = true
	visitBlock(root.Func.Blocks[0], root)
	return root
}

// FuncVisitor is an interface for analysing callgraph with a 'visit' function.
type FuncVisitor interface {
	Visit(f *ssa.Function)
}

// Traverse callgraph in depth-first order.
func (node *Node) Traverse(v FuncVisitor) {
	v.Visit(node.Func)
	for _, c := range node.Children {
		c.Traverse(v)
	}
}

func visitBlock(b *ssa.BasicBlock, node *Node) {
	if _, ok := visitedBlock[b]; ok {
		return
	}
	visitedBlock[b] = true
	for _, instr := range b.Instrs {
		switch instr := instr.(type) {
		case *ssa.Call:
			if f := instr.Common().StaticCallee(); f != nil {
				if _, ok := visitedFunc[f]; !ok {
					visitedFunc[f] = true
					node.Children = append(node.Children, &Node{
						Func:     f,
						Children: []*Node{},
					})
				}
			}
		case *ssa.Go:
			if f := instr.Common().StaticCallee(); f != nil {
				if _, ok := visitedFunc[f]; !ok {
					visitedFunc[f] = true
					node.Children = append(node.Children, &Node{
						Func:     f,
						Children: []*Node{},
					})
				}
			}
		case *ssa.If:
			visitBlock(instr.Block().Succs[0], node)
			visitBlock(instr.Block().Succs[1], node)
		case *ssa.Jump: // End of a block
			visitBlock(instr.Block().Succs[0], node)
		case *ssa.Return: // End of a function
			for _, child := range node.Children {
				if _, ok := visitedFunc[child.Func]; !ok {
					visitedFunc[child.Func] = true
					visitBlock(child.Func.Blocks[0], child)
				}
			}
		}
	}
}
