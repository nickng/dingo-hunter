package sesstype // import "github.com/nickng/dingo-hunter/sesstype"

// NodeStack is a stack for sesstype.Node
type NodeStack struct {
	nodes []Node
	count int
}

// Push pushes a sesstype.Node to the stack.
func (s *NodeStack) Push(node Node) {
	s.nodes = append(s.nodes[:s.count], node)
	s.count++
}

// Pop removes a sesstype.Node from the stack.
func (s *NodeStack) Pop() {
	if s.count <= 0 {
		panic("Cannot pop empty stack")
	}
	s.count--
}

// Top returns the sesstype.Node at the top of the stack.
func (s *NodeStack) Top() Node {
	if s.count <= 0 {
		return nil
	}
	return s.nodes[s.count-1]
}

// Size returns number of sesstype.Node on the stack.
func (s *NodeStack) Size() int {
	return s.count
}

// String returns a String representing the stack.
func (s *NodeStack) String() string {
	str := "["
	for i := s.count - 1; i >= 0; i-- {
		str += s.nodes[i].String()
		if i != 0 {
			str += ", "
		}
	}
	str += "]"
	return str
}

// NewNodeStack returns a new NodeStack instance.
func NewNodeStack() *NodeStack {
	return &NodeStack{}
}
