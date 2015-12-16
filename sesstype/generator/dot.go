package generator

import (
	"fmt"

	"github.com/awalterschulze/gographviz"
	"github.com/nickng/dingo-hunter/sesstype"
)

var (
	count                        = 0
	labelNodes map[string]string = make(map[string]string)
)

func getDot(s *sesstype.Session) string {
	graph := gographviz.NewEscape()
	graph.SetDir(true)
	graph.SetName("G")

	for role, root := range s.Types {
		sg := gographviz.NewSubGraph("\"cluster_" + role.Name() + "\"")
		if root != nil {
			visitNode(root, graph, sg, nil)
		}
		graph.AddSubGraph(graph.Name, sg.Name, nil)
	}

	return graph.String()
}

func nodeToDotNode(node sesstype.Node) *gographviz.Node {
	switch node := node.(type) {
	case *sesstype.LabelNode:
		defer func() { count++ }()
		labelNodes[node.Name()] = fmt.Sprintf("label%d", count)
		dotNode := gographviz.Node{
			Name: labelNodes[node.Name()],
			Attrs: map[string]string{
				"label": fmt.Sprintf("\"%s\"", node.String()),
				"shape": "plaintext,",
			},
		}
		return &dotNode

	case *sesstype.NewChanNode:
		defer func() { count++ }()
		return &gographviz.Node{
			Name: fmt.Sprintf("%s%d", node.Kind(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("Channel %s Type:%s", node.Chan().Name(), node.Chan().Type()),
				"shape": "rect",
				"color": "red",
			},
		}

	case *sesstype.SendNode:
		defer func() { count++ }()
		style := "solid"
		desc := ""
		if node.IsNondet() {
			style = "dashed"
			desc = " nondet"
		}
		return &gographviz.Node{
			Name: fmt.Sprintf("%s%d", node.Kind(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("Send %s%s", node.To().Name(), desc),
				"shape": "rect",
				"style": style,
			},
		}

	case *sesstype.RecvNode:
		defer func() { count++ }()
		style := "solid"
		desc := ""
		if node.IsNondet() {
			style = "dashed"
			desc = " nondet"
		}
		return &gographviz.Node{
			Name: fmt.Sprintf("%s%d", node.Kind(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("Recv %s%s", node.From().Name(), desc),
				"shape": "rect",
				"style": style,
			},
		}

	case *sesstype.GotoNode:
		return nil // No new node to create

	default:
		defer func() { count++ }()
		dotNode := gographviz.Node{
			Name: fmt.Sprintf("%s%d", node.Kind(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("\"%s\"", node.String()),
				"shape": "rect",
			},
		}
		return &dotNode
	}
}

// visitNode Creates a dot Node and from it create a subgraph of children.
// Returns head of the subgraph.
func visitNode(node sesstype.Node, graph *gographviz.Escape, subgraph *gographviz.SubGraph, parent *gographviz.Node) *gographviz.Node {
	dotNode := nodeToDotNode(node)

	if dotNode == nil { // GotoNode
		gtn := node.(*sesstype.GotoNode)
		graph.AddEdge(parent.Name, labelNodes[gtn.Name()], true, nil)
		for _, child := range node.Children() {
			visitNode(child, graph, subgraph, parent)
		}
		return parent // GotoNode's children are children of parent. So return parent.
	}

	graph.AddNode(subgraph.Name, dotNode.Name, dotNode.Attrs)
	if parent != nil { // Parent is not toplevel
		graph.AddEdge(parent.Name, dotNode.Name, true, nil)
	}
	for _, child := range node.Children() {
		visitNode(child, graph, subgraph, dotNode)
	}

	if dotNode == nil {
		panic("Cannot return nil dotNode")
	}

	return dotNode
}
