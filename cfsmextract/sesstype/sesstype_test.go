package sesstype

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/nickng/cfsm"
	"github.com/nickng/dingo-hunter/cfsmextract/utils"
	"golang.org/x/tools/go/ssa"
)

// Tests SendNode creation.
func TestSendNode(t *testing.T) {
	s := CreateSession()
	r := s.GetRole("main")
	c := s.MakeChan(utils.NewDef(utils.EmptyValue{T: nil}), r)
	n := NewSendNode(r, c, nil)
	if n.Kind() != SendOp {
		t.Errorf("Expecting node kind to be %s but got %s\n", SendOp, n.Kind())
	}
	if n.(*SendNode).nondet {
		t.Errorf("Expecting Send to be deterministic by default\n")
	}
	if len(n.Children()) != 0 {
		t.Errorf("Expecting node to have 0 children but got %d\n", len(n.Children()))
	}

	n2 := NewSelectSendNode(r, c, nil)
	if n2.Kind() != SendOp {
		t.Errorf("Expecting node kind to be %s but got %s\n", SendOp, n2.Kind())
	}
	if !n2.(*SendNode).nondet {
		t.Errorf("Expecting Select-Send to be non-deterministic by default\n")
	}
	if len(n2.Children()) != 0 {
		t.Errorf("Expecting node to have 0 children but got %d\n", len(n2.Children()))
	}

	if n2 != n.Append(n2) {
		t.Errorf("Appended node is not same as expected\n")
	}
	if len(n.Children()) != 1 {
		t.Errorf("Expecting node to have 1 children but got %d\n", len(n.Children()))
	}

}

// Tests RecvNode creation.
func TestRecvNode(t *testing.T) {
	s := CreateSession()
	r := s.GetRole("main")
	c := s.MakeChan(utils.NewDef(utils.EmptyValue{T: nil}), r)
	n := NewRecvNode(c, r, nil)
	if n.Kind() != RecvOp {
		t.Errorf("Expecting node kind to be %s but got %s\n", RecvOp, n.Kind())
	}
	if n.(*RecvNode).nondet {
		t.Errorf("Expecting Receive to be deterministic by default\n")
	}
	if len(n.Children()) != 0 {
		t.Errorf("Expecting node to have 0 children but got %d\n", len(n.Children()))
	}

	n2 := NewSelectRecvNode(c, r, nil)
	if n2.Kind() != RecvOp {
		t.Errorf("Expecting node kind to be %s but got %s\n", RecvOp, n2.Kind())
	}
	if !n2.(*RecvNode).nondet {
		t.Errorf("Expecting Select-Recv to be non-deterministic by default\n")
	}
	if len(n2.Children()) != 0 {
		t.Errorf("Expecting node to have 0 children but got %d\n", len(n2.Children()))
	}

	if n2 != n.Append(n2) {
		t.Errorf("Appended node is not same as expected\n")
	}
	if len(n.Children()) != 1 {
		t.Errorf("Expecting node to have 1 children but got %d\n", len(n.Children()))
	}

}

// Tests LabelNode and GotoNode creation.
func TestLabelGotoNode(t *testing.T) {
	l := NewLabelNode("Name")
	if l.Kind() != NoOp {
		t.Errorf("Expecting Goto node kind to be %s but got %s\n", NoOp, l.Kind())
	}
	if len(l.Children()) != 0 {
		t.Errorf("Expecting Label node to have 0 children but got %d\n", len(l.Children()))
	}

	g := NewGotoNode("Name")
	if g.Kind() != NoOp {
		t.Errorf("Expecting Goto node kind to be %s but got %s\n", NoOp, g.Kind())
	}
	if len(g.Children()) != 0 {
		t.Errorf("Expecting Goto node to have 0 children but got %d\n", len(g.Children()))
	}

	if g != l.Append(g) {
		t.Error("Appended node is not same as expected\n")
	}
	if len(l.Children()) != 1 {
		t.Errorf("Expecting Label node to have 1 children but got %d\n", len(l.Children()))
	}
}

// Tests NewChanNode creation.
func TestNewChanNode(t *testing.T) {
	s := CreateSession()
	r := s.GetRole("main")
	c := s.MakeChan(utils.NewDef(utils.EmptyValue{T: nil}), r)
	n := NewNewChanNode(c)
	if n.Kind() != NewChanOp {
		t.Errorf("Expecting node kind to be %s but got %s\n", NewChanOp, n.Kind())
	}
	if len(n.Children()) != 0 {
		t.Errorf("Expecting node to have 0 children but got %d\n", len(n.Children()))
	}
	n2 := NewNewChanNode(c)
	if n2 != n.Append(n2) {
		t.Errorf("Appended node is not same as expected\n")
	}
	if len(n.Children()) != 1 {
		t.Errorf("Expecting node to have 1 children but got %d\n", len(n.Children()))
	}
}

// Tests NewEndNode creation.
func TestEndNode(t *testing.T) {
	s := CreateSession()
	r := s.GetRole("main")
	c := s.MakeChan(utils.NewDef(utils.EmptyValue{T: nil}), r)
	n := NewEndNode(c)
	if n.Kind() != EndOp {
		t.Errorf("Expecting node kind to be %s but got %s\n", EndOp, n.Kind())
	}
	if len(n.Children()) != 0 {
		t.Errorf("Expecting node to have 0 children but got %d\n", len(n.Children()))
	}
	n2 := NewEndNode(c)
	if n2 != n.Append(n2) {
		t.Errorf("Appended node is not same as expected\n")
	}
	if len(n.Children()) != 1 {
		t.Errorf("Expecting node to have 1 children but got %d\n", len(n.Children()))
	}
}

type mockChan struct{}

func (mc mockChan) Name() string                  { return "MockChan" }
func (mc mockChan) String() string                { return "Mock Chan" }
func (mc mockChan) Type() types.Type              { return types.NewChan(types.SendRecv, types.NewStruct(nil, nil)) }
func (mc mockChan) Parent() *ssa.Function         { return nil }
func (mc mockChan) Referrers() *[]ssa.Instruction { return nil }
func (mc mockChan) Pos() token.Pos                { return token.NoPos }

func TestSelfLoop(t *testing.T) {
	s := CreateSession()
	r := s.GetRole("main")
	c := s.MakeChan(utils.NewDef(mockChan{}), r)

	n0 := NewLabelNode("BeforeReceive")
	n1 := NewRecvNode(c, r, types.NewStruct(nil, nil))
	n2 := NewGotoNode("BeforeReceive")
	n0.Append(n1)
	n1.Append(n2)

	ms := NewCFSMs(s)
	m := ms.Sys.NewMachine()
	ms.States[m] = make(map[string]*cfsm.State)
	ms.rootToMachine(r, n0, m)
	if want, got := 1, len(m.States()); want != got {
		t.Errorf("expecting %d states for self-loop but got %d", want, got)
	}
	if want, got := 1, len(m.States()[0].Transitions()); want != got {
		t.Errorf("expecting %d transitions for self-loop but got %d", want, got)
	}
	if from, to := m.States()[0], m.States()[0].Transitions()[0].State(); from != to {
		t.Errorf("expecting self-loop but got %s", m.String())
	}
}
