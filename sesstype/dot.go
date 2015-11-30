package sesstype

import (
	"fmt"
	"os"

	"github.com/awalterschulze/gographviz"
)

var (
	count                        = 0
	labelNodes map[string]string = make(map[string]string)
)

func GenDot(sess *Session) {
	graph := gographviz.NewEscape()
	graph.SetDir(true)
	graph.SetName("G")

	for role, root := range sess.Types {
		sg := gographviz.NewSubGraph("\"cluster_" + role.Name() + "\"")
		if root != nil {
			DotVisitNode(root, graph, sg, nil)
		}
		graph.AddSubGraph(graph.Name, sg.Name, nil)
	}

	f, err := os.OpenFile("output.dot", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer f.Close()
	if err != nil {
		panic(err)
	}

	_, err = f.WriteString(graph.String())
	if err != nil {
		panic(err)
	}
}

func DotCreateNode(node Node) *gographviz.Node {
	switch node := node.(type) {
	case *LabelNode:
		defer func() { count++ }()
		labelNodes[node.name] = fmt.Sprintf("label%d", count)
		dotNode := gographviz.Node{
			Name: labelNodes[node.name],
			Attrs: map[string]string{
				"label": fmt.Sprintf("\"%s\"", node.String()),
				"shape": "plaintext,",
			},
		}
		return &dotNode

	case *NewChanNode:
		defer func() { count++ }()
		return &gographviz.Node{
			Name: fmt.Sprintf("%s%d", node.Kind(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("Channel %s Type:%s", node.ch.Name(), node.ch.Type()),
				"shape": "rect",
				"color": "red",
			},
		}

	case *SendNode:
		defer func() { count++ }()
		style := "solid"
		desc := ""
		if node.nondet {
			style = "dashed"
			desc = " nondet"
		}
		return &gographviz.Node{
			Name: fmt.Sprintf("%s%d", node.Kind(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("Send %s%s", node.dest.Name(), desc),
				"shape": "rect",
				"style": style,
			},
		}

	case *RecvNode:
		defer func() { count++ }()
		style := "solid"
		desc := ""
		if node.nondet {
			style = "dashed"
			desc = " nondet"
		}
		return &gographviz.Node{
			Name: fmt.Sprintf("%s%d", node.Kind(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("Recv %s%s", node.orig.Name(), desc),
				"shape": "rect",
				"style": style,
			},
		}

	case *GotoNode:
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
func DotVisitNode(node Node, graph *gographviz.Escape, subgraph *gographviz.SubGraph, parent *gographviz.Node) *gographviz.Node {
	dotNode := DotCreateNode(node)

	if dotNode == nil { // GotoNode
		gtn := node.(*GotoNode)
		graph.AddEdge(parent.Name, labelNodes[gtn.name], true, nil)
		for _, child := range node.Children() {
			DotVisitNode(child, graph, subgraph, parent)
		}

		return parent // GotoNode's children are children of parent. So return parent.
	}

	graph.AddNode(subgraph.Name, dotNode.Name, dotNode.Attrs)
	if parent != nil { // Parent is not toplevel
		graph.AddEdge(parent.Name, dotNode.Name, true, nil)
	}
	for _, child := range node.Children() {
		DotVisitNode(child, graph, subgraph, dotNode)
	}

	if dotNode == nil {
		panic("Cannot return nil dotNode")
	}

	return dotNode
}
