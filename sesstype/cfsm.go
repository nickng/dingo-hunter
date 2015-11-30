package sesstype

import (
	"fmt"
	"os"
)

var (
	cfsmStateCount = make(map[string]int)    // Contains next state number for each CFSM.
	cfsmByName     = make(map[string]int)    // Converts role names to CFSM state number.
	labelJumpState = make(map[string]string) // Convert label names to state names to jump to.
	totalCFSMs     = 0                       // Number of CFSMs.
	chanCFSMs      = 0                       // Number of CFSMs for channels.
)

func genNewState(roleName string) string {
	stateIdx := cfsmStateCount[roleName]
	cfsmStateCount[roleName]++
	return fmt.Sprintf("%sZZ%d", roleName, stateIdx)
}

func isAlphanum(r rune) bool {
	return ('0' <= r && r <= '9') || ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z')
}

// Encode non-alphanum symbols to empty.
func encodeSymbols(name string) string {
	outstr := ""
	for _, runeVal := range name {
		if isAlphanum(runeVal) {
			outstr += string(runeVal)
		} else {
			switch runeVal {
			case '{':
				outstr += "LBRACE"
			case '}':
				outstr += "RBRACE"
			case '.':
				outstr += "DOT"
			case '(':
				outstr += "LPAREN"
			case ')':
				outstr += "RPAREN"
			case '/':
				outstr += "SLASH"
			}
		}
		// Ignore other non alphanum
	}
	return outstr
}

// Create CFSM for channel.
func genChanCFSM(name string, typ string, begin int, end int) string {
	q0 := fmt.Sprintf("Chan%s", encodeSymbols(genNewState(name)))
	qTerm := fmt.Sprintf("Close%s", encodeSymbols(genNewState(name)))
	cfsm := ""
	for i := begin; i < end; i++ {
		q1 := encodeSymbols(genNewState(name))
		cfsm += fmt.Sprintf("%s %d ? %s %s\n", q0, i, encodeSymbols(typ), q1)
		for j := begin; j < end; j++ {
			if i != j {
				cfsm += fmt.Sprintf("%s %d ! %s %s\n", q1, j, encodeSymbols(typ), q0)
			}
		}
		cfsm += fmt.Sprintf("%s %d ? STOP %s\n", q0, i, qTerm)
		for j := begin; j < end; j++ {
			if i != j {
				cfsm += fmt.Sprintf("%s %d ! STOP %s\n", qTerm, j, qTerm)
			}
		}
	}

	return fmt.Sprintf(".outputs\n.state graph\n%s.marking %s\n.end\n\n", cfsm, q0)
}

// nodeToCFSM creates a transition from state q0 which took prefix to the
// subtree rooted at root.
func nodeToCFSM(root Node, role Role, q0 string, transition string) string {
	if q0 == "" {
		panic("q0 cannot be empty")
	}

	cfsm := ""

	switch node := root.(type) {
	case *SendNode:
		var qSend, send string
		to, ok := cfsmByName[node.dest.Name()]
		if !ok {
			panic(fmt.Sprintf("Sending to unknown channel: %s", node.dest.Name()))
		}

		if transition != "" {
			qSend = encodeSymbols(genNewState(role.Name()))
			cfsm += fmt.Sprintf("%s %s %s\n", q0, transition, qSend)
		} else { // First transition
			qSend = q0 // next parent is q0
		}
		send = fmt.Sprintf("%d ! %s", to, encodeSymbols(node.dest.Type().String()))

		if len(root.Children()) == 0 {
			return fmt.Sprintf("%s %s %s\n", qSend, send, encodeSymbols(genNewState(role.Name())))
		}

		for _, child := range root.Children() {
			cfsm += nodeToCFSM(child, role, qSend, send)
		}
		return cfsm

	case *RecvNode:
		var qRecv, recv string
		from, ok := cfsmByName[node.orig.Name()]
		if !ok {
			panic(fmt.Sprintf("Receiving from unknown channel: %s", node.orig.Name()))
		}

		if transition != "" {
			qRecv = encodeSymbols(genNewState(role.Name()))
			cfsm += fmt.Sprintf("%s %s %s\n", q0, transition, qRecv)
		} else { // First transition
			qRecv = q0 // next parent is q0
		}
		recv = fmt.Sprintf("%d ? %s", from, encodeSymbols(node.orig.Type().String()))

		if len(root.Children()) == 0 {
			return fmt.Sprintf("%s %s %s\n", qRecv, recv, encodeSymbols(genNewState(role.Name())))
		}

		for _, child := range root.Children() {
			cfsm += nodeToCFSM(child, role, qRecv, recv)
		}
		return cfsm

	case *EndNode:
		var qEnd, end string
		ch, ok := cfsmByName[node.ch.Name()]
		if !ok {
			panic(fmt.Sprintf("Closing unknown channel: %s", node.ch.Name()))
		}

		if transition != "" {
			qEnd = encodeSymbols(genNewState(role.Name()))
			cfsm += fmt.Sprintf("%s %s %s\n", q0, transition, qEnd)
		} else { // First transition
			qEnd = q0
		}
		end = fmt.Sprintf("%d ! STOP", ch)

		if len(root.Children()) == 0 {
			return fmt.Sprintf("%s %s %s\n", qEnd, end, encodeSymbols(genNewState(role.Name())))
		}

		for _, child := range root.Children() {
			cfsm += nodeToCFSM(child, role, qEnd, end)
		}
		return cfsm

	case *NewChanNode:
		if len(root.Children()) == 0 {
			if transition != "" {
				return fmt.Sprintf("%s %s %s\n", q0, transition, encodeSymbols(genNewState(role.Name())))
			}
		}

		for _, child := range root.Children() {
			cfsm += nodeToCFSM(child, role, q0, transition)
		}
		return cfsm

	case *LabelNode:
		labelJumpState[node.name] = encodeSymbols(genNewState(role.Name()))

		if len(root.Children()) == 0 {
			if transition != "" {
				return fmt.Sprintf("%s %s %s\n", q0, transition, labelJumpState[node.name])
			}
			return ""
		}

		for _, child := range root.Children() {
			cfsm += nodeToCFSM(child, role, q0, transition)
		}
		return cfsm

	case *GotoNode:
		qGoto, ok := labelJumpState[node.name]
		if !ok {
			//panic(fmt.Sprintf("Jump to unknown state: %s", node.name))
			return ""
		}

		if transition != "" {
			cfsm += fmt.Sprintf("%s %s %s\n", q0, transition, qGoto)
			for _, child := range root.Children() {
				cfsm += nodeToCFSM(child, role, qGoto, transition)
			}
			return cfsm
		}

		for _, child := range root.Children() {
			cfsm += nodeToCFSM(child, role, q0, transition)
		}

		return cfsm

	case *EmptyBodyNode:
		qNext := q0
		if len(root.Children()) == 0 {
			if transition != "" {
				qNext = encodeSymbols(genNewState(role.Name()))
				cfsm += fmt.Sprintf("%s %s %s\n", q0, transition, qNext)
			}
		}
		// Passthrough
		for _, child := range root.Children() {
			cfsm += nodeToCFSM(child, role, qNext, transition)
		}
		return cfsm

	default:
		panic(fmt.Sprintf("Unhandled node type: %T", node))
	}
}

func genCFSM(role Role, root Node) string {
	q0 := encodeSymbols(genNewState(role.Name()))
	cfsmBody := nodeToCFSM(root, role, q0, "")
	if cfsmBody == "" {
		return ""
	}
	return fmt.Sprintf(".outputs\n.state graph\n%s.marking %s\n.end\n\n", cfsmBody, q0)
}

// Initialise the CFSM counts.
func initCFSMs(s *Session) {
	for _, c := range s.Chans {
		cfsmByName[c.Name()] = totalCFSMs
		chanCFSMs++
		totalCFSMs++
	}

	for r := range s.Types {
		cfsmByName[r.Name()] = totalCFSMs
		totalCFSMs++
	}
}

func PrintCFSMSummary() {
	fmt.Printf("Total of %d CFSMs (%d are channels)\n", totalCFSMs, chanCFSMs)
	for cfsmName, cfsmIndex := range cfsmByName {
		if cfsmIndex < chanCFSMs {
			fmt.Printf("\t%d\t= %s (channel)\n", cfsmIndex, cfsmName)
		} else {
			fmt.Printf("\t%d\t= %s\n", cfsmIndex, cfsmName)
		}
	}
}

// GenAllCFSMs generates CFSMs for all roles in the session, plus the static
// CFSMs for the channels.
func GenAllCFSMs(s *Session) {
	initCFSMs(s)

	allCFSMs := ""
	goroutineCFSMs := ""
	chanCFSMs := ""
	nonEmptyCFSMs := 0

	for r, root := range s.Types {
		cfsm := genCFSM(r, root)
		fmt.Fprintf(os.Stderr, "Generate %s CFSM\n", r.Name())
		if cfsm == "" {
			fmt.Fprintf(os.Stderr, "  ^ Empty\n")
		}
		if cfsm != "" {
			nonEmptyCFSMs++
			goroutineCFSMs += cfsm
		}
	}

	for _, c := range s.Chans {
		chanCFSMs += genChanCFSM(c.Name(), c.Type().String(), len(s.Chans), len(s.Chans)+nonEmptyCFSMs)
	}

	allCFSMs = chanCFSMs + goroutineCFSMs

	f, err := os.OpenFile("output_cfsms", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer f.Close()

	if err != nil {
		panic(err)
	}

	_, err = f.WriteString(allCFSMs)
	if err != nil {
		panic(err)
	}
}
