// Package sesstype encapsulates representation of session types
// As opposed to role-based session types, this representation is channel-based.
// In particular, sending and receiving both keep track of the role and
// channel involved.
package sesstype // import "github.com/nickng/dingo-hunter/sesstype"

import (
	"fmt"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types"
)

//go:generate stringer -type=op
type op int

// Chan is a typed channel in a session.
type Chan interface {
	Name() string
	Type() types.Type
	Role() Role
	Value() ssa.Value
}

type channel struct {
	value  ssa.Value
	creat  Role
	extern bool
}

// Return a name of channel.
func (ch channel) Name() string {
	fullname := fmt.Sprintf("%s.%s.%s", ch.creat.Name(), ch.value.Parent().String(), ch.value.Name())
	if ch.extern {
		return fullname + "*"
	}
	return fullname
}

// Return the payload type of channel.
func (ch channel) Type() types.Type {
	return ch.value.Type().(*types.Chan).Elem()
}
func (ch channel) Role() Role {
	return ch.creat
}
func (ch channel) Value() ssa.Value {
	return ch.value
}

// Role in a session (main or goroutine).
type Role interface {
	Name() string
}

type role struct {
	name string
}

func (r *role) Name() string { return r.name }

// Different operations/actions available in session.
const (
	NoOp op = iota
	NewChanOp
	SendOp
	RecvOp
	EndOp
)

// A Node in the session graph.
type Node interface {
	Kind() op               // For checking type without type switching
	Child(index int) Node   // Gets child at index
	Append(child Node) Node // Returns new child for chaining
	Children() []Node       // Returns whole slice
	String() string
}

// Session is a container of session graph nodes, also holds information about
// channels and roles in the current session.
type Session struct {
	types  map[Role]Node      // Root Node for each Role
	chans  map[ssa.Value]Chan // Actual channels are stored here
	roles  map[string]Role    // Actual roles are stored here
	labels map[string]int     // Count number of times label is used
}

// CreateSession initialises a new empty Session.
func CreateSession() *Session {
	return &Session{
		types: make(map[Role]Node),
		chans: make(map[ssa.Value]Chan),
		roles: make(map[string]Role),
	}
}

// GetType returns the local type of the session from the given role.
func (s *Session) GetType(role Role) Node { return s.types[role] }

// GetChan returns the channel variable given its name (e.g. register name).
func (s *Session) GetChan(v ssa.Value) Chan { return s.chans[v] }

// GetRole returns or create (if empty) a new session role using given name.
func (s *Session) GetRole(name string) Role { // Get or create role
	if _, found := s.roles[name]; !found {
		s.roles[name] = &role{name: name}
	}
	return s.roles[name]
}

// SetType sets the local type of the given role to root
func (s *Session) SetType(role Role, root Node) { s.types[role] = root }

// MakeChan creates and stores a new session channel created.
func (s *Session) MakeChan(v ssa.Value, r Role) Chan {
	s.chans[v] = &channel{
		value:  v,
		creat:  r,
		extern: false,
	}
	return s.chans[v]
}

// MakeExtChan creates and stores a new channel and mark as externally created.
func (s *Session) MakeExtChan(v ssa.Value, r Role) Chan {
	s.chans[v] = &channel{
		value:  v,
		creat:  r,
		extern: true,
	}
	return s.chans[v]
}

// NewChanNode represents creation of new channel
type NewChanNode struct {
	ch       Chan
	children []Node
}

func (nc *NewChanNode) Kind() op         { return NewChanOp }
func (nc *NewChanNode) Children() []Node { return nc.children }
func (nc *NewChanNode) Append(node Node) Node {
	nc.children = append(nc.children, node)
	return node
}
func (nc *NewChanNode) Child(index int) Node { return nc.children[index] }
func (nc *NewChanNode) String() string {
	return fmt.Sprintf("NewChan %s of type %s", nc.ch.Name(), nc.ch.Type())
}

// SendNode represents a send.
type SendNode struct {
	sndr     Role   // Sender
	dest     Chan   // Destination
	nondet   bool   // Is this non-deterministic?
	t        string // Datatype
	children []Node
}

func (s *SendNode) Kind() op         { return SendOp }
func (s *SendNode) Children() []Node { return s.children }
func (s *SendNode) Append(node Node) Node {
	s.children = append(s.children, node)
	return node
}
func (s *SendNode) Child(index int) Node { return s.children[index] }
func (s *SendNode) String() string {
	var nd string
	if s.nondet {
		nd = "nondet "
	}
	return fmt.Sprintf("Send %s ->{ chan: %s %s}", s.sndr.Name(), s.dest.Name(), nd)
}

// RecvNode represents a receive.
type RecvNode struct {
	orig     Chan   // Originates from
	rcvr     Role   // Received by
	nondet   bool   // Is this non-deterministic?
	t        string // Datatype
	children []Node
}

func (r *RecvNode) Kind() op         { return RecvOp }
func (r *RecvNode) Children() []Node { return r.children }
func (r *RecvNode) Append(node Node) Node {
	r.children = append(r.children, node)
	return node
}
func (r *RecvNode) Child(index int) Node { return r.children[index] }
func (r *RecvNode) String() string {
	var nd string
	if r.nondet {
		nd = "nondet "
	}

	return fmt.Sprintf("Recv { chan: %s %s}-> %s", r.orig.Name(), nd, r.rcvr.Name())
}

// LabelNode makes a placeholder for loop/jumping
type LabelNode struct {
	name     string
	children []Node
}

func (l *LabelNode) Kind() op         { return NoOp }
func (l *LabelNode) Children() []Node { return l.children }
func (l *LabelNode) Append(node Node) Node {
	l.children = append(l.children, node)
	return node
}
func (l *LabelNode) Child(index int) Node { return l.children[index] }
func (l *LabelNode) String() string       { return fmt.Sprintf("%s", l.name) }

// GotoNode represents a jump (edge to existing LabelNode)
type GotoNode struct {
	name     string
	children []Node
}

func (g *GotoNode) Kind() op         { return NoOp }
func (g *GotoNode) Children() []Node { return g.children }
func (g *GotoNode) Append(node Node) Node {
	g.children = append(g.children, node)
	return node
}
func (g *GotoNode) Child(index int) Node { return g.children[index] }
func (g *GotoNode) String() string       { return fmt.Sprintf("Goto %s", g.name) }

type EndNode struct {
	ch       Chan
	children []Node
}

func (e *EndNode) Kind() op         { return EndOp }
func (e *EndNode) Children() []Node { return e.children }
func (e *EndNode) Append(node Node) Node {
	e.children = append(e.children, node)
	return node
}
func (e *EndNode) Child(index int) Node { return e.children[index] }
func (e *EndNode) String() string       { return fmt.Sprintf("End %s", e.ch.Name()) }

type EmptyBodyNode struct {
	children []Node
}

func (e *EmptyBodyNode) Kind() op         { return NoOp }
func (e *EmptyBodyNode) Children() []Node { return e.children }
func (e *EmptyBodyNode) Append(node Node) Node {
	e.children = append(e.children, node)
	return node
}
func (e *EmptyBodyNode) Child(index int) Node { return e.children[index] }
func (e *EmptyBodyNode) String() string       { return "(Empty)" }

// MkNewChanNode makes a NewChanNode.
func MkNewChanNode(ch Chan) Node {
	return &NewChanNode{ch: ch, children: []Node{}}
}

// MkSendNode makes a SendNode.
func MkSendNode(sndr Role, dest Chan) Node {
	return &SendNode{
		sndr:     sndr,
		dest:     dest,
		nondet:   false,
		children: []Node{},
	}
}

// MkSelectSendNode makes a SendNode in a select (non-deterministic).
func MkSelectSendNode(sndr Role, dest Chan) Node {
	return &SendNode{
		sndr:     sndr,
		dest:     dest,
		nondet:   true,
		children: []Node{},
	}
}

// MkRecvNode makes a RecvNode.
func MkRecvNode(orig Chan, rcvr Role) Node {
	return &RecvNode{
		orig:     orig,
		rcvr:     rcvr,
		nondet:   false,
		children: []Node{},
	}
}

// MkSelectRecvNode makes a RecvNode in a select (non-deterministic).
func MkSelectRecvNode(orig Chan, rcvr Role) Node {
	return &RecvNode{
		orig:     orig,
		rcvr:     rcvr,
		nondet:   true,
		children: []Node{},
	}
}

// MkLabelNode makes a LabelNode.
func MkLabelNode(name string) Node {
	return &LabelNode{
		name:     name,
		children: []Node{},
	}
}

// MkGotoNode makes a GotoNode.
func MkGotoNode(name string) Node {
	return &GotoNode{
		name:     name,
		children: []Node{},
	}
}

// MkEndNode makse an EndNode.
func MkEndNode(ch Chan) Node {
	return &EndNode{
		ch: ch,
	}
}

// String displays session details.
func (s *Session) String() string {
	str := "# Channels\n"
	for _, ch := range s.chans {
		str += fmt.Sprintf("%s ", ch.Name())
	}
	str += "\n# Role\n"
	for _, r := range s.roles {
		str += fmt.Sprintf("%s ", r.Name())
	}
	str += "\n# Session\n"
	for role, session := range s.types {
		str += fmt.Sprintf("  %s: %s", role.Name(), StringRecursive(session))
		str += "\n"
	}
	return str
}

func StringRecursive(node Node) string {
	str := ""
	if node == nil {
		return str
	}

	str += node.String() + "; "
	switch len(node.Children()) {
	case 0:
	case 1:
		str += StringRecursive(node.Children()[0])
	default:
		str += fmt.Sprintf("children: %d &{", len(node.Children()))
		for i, child := range node.Children() {
			if i > 0 {
				str += ","
			}
			str += StringRecursive(child)
		}
		str += "}"
	}
	return str
}
