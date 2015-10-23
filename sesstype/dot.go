package sesstype

import (
	"fmt"
	"github.com/awalterschulze/gographviz"
	"os"
)

var (
	count                        = 0
	labelNodes map[string]string = make(map[string]string)
)

func GenDot(sess *Session) {
	graph := gographviz.NewEscape()
	graph.SetDir(true)
	graph.SetName("G")

	for role, root := range sess.types {
		sg := gographviz.NewSubGraph("cluster_" + role.Name())
		if root != nil {
			visitNode(root, graph, sg)
		}
		graph.AddSubGraph(graph.Name, sg.Name, map[string]string{"label": role.Name()})
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

func CreateNode(node Node) *gographviz.Node {
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
			Name: fmt.Sprintf("%s%d", node.Kind().String(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("New channel %s", node.ch.Name()),
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
			Name: fmt.Sprintf("%s%d", node.Kind().String(), count),
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
			Name: fmt.Sprintf("%s%d", node.Kind().String(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("Receive %s%s", node.orig.Name(), desc),
				"shape": "rect",
				"style": style,
			},
		}

	case *GotoNode:
		return nil

	default:
		defer func() { count++ }()
		dotNode := gographviz.Node{
			Name: fmt.Sprintf("%s%d", node.Kind().String(), count),
			Attrs: map[string]string{
				"label": fmt.Sprintf("%s", node.String()),
				"shape": "rect",
			},
		}
		return &dotNode
	}
}

func visitNode(node Node, graph *gographviz.Escape, subgraph *gographviz.SubGraph) *gographviz.Node {
	gNode := CreateNode(node)
	if gNode != nil {
		if subgraph.Name == "cluster_main" && len(graph.Nodes.Nodes) == 0 {
			gNode.Attrs.Add("style", "bold")
		}
		graph.AddNode(subgraph.Name, gNode.Name, gNode.Attrs)
	}

	for _, child := range node.Children() {
		cNode := visitNode(child, graph, subgraph)
		if cNode != nil {
			graph.AddEdge(gNode.Name, cNode.Name, true, nil)
		} else {
			// Goto Node
			graph.AddEdge(gNode.Name, labelNodes[child.(*GotoNode).name], true, nil)
		}
	}

	return gNode
}
