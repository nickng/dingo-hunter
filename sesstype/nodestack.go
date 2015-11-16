package sesstype

type NodeStack struct {
	nodes []Node
	count int
}

func (s *NodeStack) Push(node Node) {
	s.nodes = append(s.nodes[:s.count], node)
	s.count++
}

func (s *NodeStack) Pop() {
	if s.count <= 0 {
		panic("Cannot pop empty stack")
	}
	s.count--
}

func (s *NodeStack) Top() Node {
	if s.count <= 0 {
		return nil
	}
	return s.nodes[s.count-1]
}

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

func NewNodeStack() *NodeStack {
	return &NodeStack{}
}

