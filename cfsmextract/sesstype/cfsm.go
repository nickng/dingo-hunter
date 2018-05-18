package sesstype

import (
	"fmt"
	"io"
	"log"

	"github.com/nickng/cfsm"
)

// STOP is the 'close' message.
const STOP = "STOP"

// CFSMs captures a CFSM system syserated from a Session.
type CFSMs struct {
	Sys    *cfsm.System
	Chans  map[Role]*cfsm.CFSM
	Roles  map[Role]*cfsm.CFSM
	States map[*cfsm.CFSM]map[string]*cfsm.State
}

func NewCFSMs(s *Session) *CFSMs {
	sys := &CFSMs{
		Sys:    cfsm.NewSystem(),
		Chans:  make(map[Role]*cfsm.CFSM),
		Roles:  make(map[Role]*cfsm.CFSM),
		States: make(map[*cfsm.CFSM]map[string]*cfsm.State),
	}
	for _, c := range s.Chans {
		m := sys.Sys.NewMachine()
		m.Comment = c.Name()
		sys.Chans[c] = m
		defer sys.chanToMachine(c, c.Type().String(), m)
	}
	for role, root := range s.Types {
		m := sys.Sys.NewMachine()
		m.Comment = role.Name()
		sys.Roles[role] = m
		sys.States[m] = make(map[string]*cfsm.State)
		sys.rootToMachine(role, root, m)
		if m.IsEmpty() {
			log.Println("Machine", m.ID, "is empty")
			sys.Sys.RemoveMachine(m.ID)
			delete(sys.Roles, role)
			delete(sys.States, m)
		}
	}
	return sys
}

// WriteTo implementers io.WriterTo interface.
func (sys *CFSMs) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write([]byte(sys.Sys.String()))
	return int64(n), err
}

// PrintSummary shows the statistics of the CFSM syseration.
func (sys *CFSMs) PrintSummary() {
	fmt.Printf("Total of %d CFSMs (%d are channels)\n",
		len(sys.Roles)+len(sys.Chans), len(sys.Chans))
	for r, m := range sys.Chans {
		fmt.Printf("\t%d\t= %s (channel)\n", m.ID, r.Name())
	}
	for r, m := range sys.Roles {
		fmt.Printf("\t%d\t= %s\n", m.ID, r.Name())
	}
}

func (sys *CFSMs) rootToMachine(role Role, root Node, m *cfsm.CFSM) {
	q0 := m.NewState()
	sys.nodeToMachine(role, root, q0, m)
	m.Start = q0
}

func (sys *CFSMs) nodeToMachine(role Role, node Node, q0 *cfsm.State, m *cfsm.CFSM) {
	switch node := node.(type) {
	case *SendNode:
		to, ok := sys.Chans[node.To()]
		if !ok {
			log.Fatal("Cannot Send to unknown channel", node.To().Name())
		}
		tr := cfsm.NewSend(to, node.To().Type().String())
		var qSent *cfsm.State
		if sys.isSelfLoop(m, q0, node) {
			qSent = q0
		} else {
			qSent = m.NewState()
			for _, c := range node.Children() {
				sys.nodeToMachine(role, c, qSent, m)
			}
		}
		tr.SetNext(qSent)
		q0.AddTransition(tr)

	case *RecvNode:
		from, ok := sys.Chans[node.From()]
		if !ok {
			log.Fatal("Cannot Recv from unknown channel", node.From().Name())
		}
		msg := node.From().Type().String()
		if node.Stop() {
			msg = STOP
		}
		tr := cfsm.NewRecv(from, msg)
		var qRcvd *cfsm.State
		if sys.isSelfLoop(m, q0, node) {
			qRcvd = q0
		} else {
			qRcvd = m.NewState()
			for _, c := range node.Children() {
				sys.nodeToMachine(role, c, qRcvd, m)
			}
		}
		tr.SetNext(qRcvd)
		q0.AddTransition(tr)

	case *EndNode:
		ch, ok := sys.Chans[node.Chan()]
		if !ok {
			log.Fatal("Cannot Close unknown channel", node.Chan().Name())
		}
		tr := cfsm.NewSend(ch, STOP)
		qEnd := m.NewState()
		for _, c := range node.Children() {
			sys.nodeToMachine(role, c, qEnd, m)
		}
		tr.SetNext(qEnd)
		q0.AddTransition(tr)

	case *NewChanNode, *EmptyBodyNode: // Skip
		for _, c := range node.Children() {
			sys.nodeToMachine(role, c, q0, m)
		}

	case *LabelNode:
		sys.States[m][node.Name()] = q0
		for _, c := range node.Children() {
			sys.nodeToMachine(role, c, q0, m)
		}

	case *GotoNode:
		qTarget := sys.States[m][node.Name()]
		for _, c := range node.Children() {
			sys.nodeToMachine(role, c, qTarget, m)
		}

	default:
		log.Fatalf("Unhandled node type %T", node)
	}
}

func (sys *CFSMs) chanToMachine(ch Role, T string, m *cfsm.CFSM) {
	q0 := m.NewState()
	qEnd := m.NewState()
	for _, machine := range sys.Roles {
		q1 := m.NewState()
		// q0 -- Recv --> q1
		tr0 := cfsm.NewRecv(machine, T)
		tr0.SetNext(q1)
		q0.AddTransition(tr0)
		// q1 -- Send --> q0
		for _, machine2 := range sys.Roles {
			if machine.ID != machine2.ID {
				tr1 := cfsm.NewSend(machine2, T)
				tr1.SetNext(q0)
				q1.AddTransition(tr1)
			}
		}
		// q0 -- STOP --> qEnd (same qEnd)
		tr2 := cfsm.NewRecv(machine, STOP)
		tr2.SetNext(qEnd)
		qEnd.AddTransition(tr2)
		// qEnd -- STOP --> qEnd
		for _, machine2 := range sys.Roles {
			if machine.ID != machine2.ID {
				tr3 := cfsm.NewSend(machine2, STOP)
				tr3.SetNext(qEnd)
				qEnd.AddTransition(tr3)
			}
		}
	}
	m.Start = q0
}

// isSelfLoop returns true if the action of node is a self-loop
// i.e. the state before and after the transition is the same.
func (sys *CFSMs) isSelfLoop(m *cfsm.CFSM, q0 *cfsm.State, node Node) bool {
	if len(node.Children()) == 1 {
		if gotoNode, ok := node.Child(0).(*GotoNode); ok {
			if loopback, ok := sys.States[m][gotoNode.Name()]; ok {
				return loopback == q0
			}
		}
	}
	return false
}
