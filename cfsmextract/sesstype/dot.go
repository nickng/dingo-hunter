package sesstype

import (
	"fmt"
	"io"
	"log"

	"github.com/awalterschulze/gographviz"
)

// GraphvizDot reprents a new graphviz dot graph.
type GraphvizDot struct {
	Graph      *gographviz.Escape
	Count      int
	LabelNodes map[string]string
}

// NewGraphvizDot creates a new graphviz dot graph from a session.
func NewGraphvizDot(s *Session) *GraphvizDot {
	dot := &GraphvizDot{
		Graph:      gographviz.NewEscape(),
		Count:      0,
		LabelNodes: make(map[string]string),
	}
	dot.Graph.SetDir(true)
	dot.Graph.SetName("G")

	for role, root := range s.Types {
		sg := gographviz.NewSubGraph("\"cluster_" + role.Name() + "\"")
		if root != nil {
			dot.visitNode(root, sg, nil)
		}
		dot.Graph.AddSubGraph(dot.Graph.Name, sg.Name, nil)
	}
	return dot
}

// WriteTo implements io.WriterTo interface.
func (dot *GraphvizDot) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write([]byte(dot.Graph.String()))
	return int64(n), err
}

func (dot *GraphvizDot) nodeToDotNode(node Node) *gographviz.Node {
	switch node := node.(type) {
	case *LabelNode:
		defer func() { dot.Count++ }()
		dot.LabelNodes[node.Name()] = fmt.Sprintf("label%d", dot.Count)
		attrs, err := gographviz.NewAttrs(map[string]string{
			"label": fmt.Sprintf("\"%s\"", node.String()),
			"shape": "plaintext,",
		})
		if err != nil {
			log.Fatal(err)
		}
		dotNode := gographviz.Node{
			Name:  dot.LabelNodes[node.Name()],
			Attrs: attrs,
		}
		return &dotNode

	case *NewChanNode:
		defer func() { dot.Count++ }()
		attrs, err := gographviz.NewAttrs(map[string]string{
			"label": fmt.Sprintf("Channel %s Type:%s", node.Chan().Name(), node.Chan().Type()),
			"shape": "rect",
			"color": "red",
		})
		if err != nil {
			log.Fatal(err)
		}
		return &gographviz.Node{
			Name:  fmt.Sprintf("%s%d", node.Kind(), dot.Count),
			Attrs: attrs,
		}

	case *SendNode:
		defer func() { dot.Count++ }()
		style := "solid"
		desc := ""
		if node.IsNondet() {
			style = "dashed"
			desc = " nondet"
		}
		attrs, err := gographviz.NewAttrs(map[string]string{
			"label": fmt.Sprintf("Send %s%s", node.To().Name(), desc),
			"shape": "rect",
			"style": style,
		})
		if err != nil {
			log.Fatal(err)
		}
		return &gographviz.Node{
			Name:  fmt.Sprintf("%s%d", node.Kind(), dot.Count),
			Attrs: attrs,
		}

	case *RecvNode:
		defer func() { dot.Count++ }()
		style := "solid"
		desc := ""
		if node.IsNondet() {
			style = "dashed"
			desc = " nondet"
		}
		attrs, err := gographviz.NewAttrs(map[string]string{
			"label": fmt.Sprintf("Recv %s%s", node.From().Name(), desc),
			"shape": "rect",
			"style": style,
		})
		if err != nil {
			log.Fatal(err)
		}
		return &gographviz.Node{
			Name:  fmt.Sprintf("%s%d", node.Kind(), dot.Count),
			Attrs: attrs,
		}

	case *GotoNode:
		return nil // No new node to create

	default:
		defer func() { dot.Count++ }()
		attrs, err := gographviz.NewAttrs(map[string]string{
			"label": fmt.Sprintf("\"%s\"", node.String()),
			"shape": "rect",
		})
		if err != nil {
			log.Fatal(err)
		}
		dotNode := gographviz.Node{
			Name:  fmt.Sprintf("%s%d", node.Kind(), dot.Count),
			Attrs: attrs,
		}
		return &dotNode
	}
}

// visitNode Creates a dot Node and from it create a subgraph of children.
// Returns head of the subgraph.
func (dot *GraphvizDot) visitNode(node Node, subgraph *gographviz.SubGraph, parent *gographviz.Node) *gographviz.Node {
	dotNode := dot.nodeToDotNode(node)

	if dotNode == nil { // GotoNode
		gtn := node.(*GotoNode)
		dot.Graph.AddEdge(parent.Name, dot.LabelNodes[gtn.Name()], true, nil)
		for _, child := range node.Children() {
			dot.visitNode(child, subgraph, parent)
		}
		return parent // GotoNode's children are children of parent. So return parent.
	}

	attrs := make(map[string]string)
	for k, v := range dotNode.Attrs {
		attrs[string(k)] = v
	}
	dot.Graph.AddNode(subgraph.Name, dotNode.Name, attrs)
	if parent != nil { // Parent is not toplevel
		dot.Graph.AddEdge(parent.Name, dotNode.Name, true, nil)
	}
	for _, child := range node.Children() {
		dot.visitNode(child, subgraph, dotNode)
	}

	if dotNode == nil {
		panic("Cannot return nil dotNode")
	}

	return dotNode
}
