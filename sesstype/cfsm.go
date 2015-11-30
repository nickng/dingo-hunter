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

// nodeToCFSM creates CFSM states from sesstype.Node. q0 is already written.
func nodeToCFSM(root Node, role Role, q0 string, initial bool) string {
	switch node := root.(type) {
	case *SendNode:
		toCFSM, ok := cfsmByName[node.dest.Name()]
		if !ok {
			panic(fmt.Sprintf("Sending to unknown channel: %s", node.dest.Name()))
		}

		sendType := encodeSymbols(node.dest.Type().String())
		qSend := encodeSymbols(genNewState(role.Name()))
		cfsm := fmt.Sprintf("%s %d ! %s ", q0, toCFSM, sendType)
		if !initial {
			cfsm = fmt.Sprintf("%s\n%s", q0, cfsm)
		}
		childCfsm := ""
		for _, child := range node.Children() {
			childCfsm += nodeToCFSM(child, role, qSend, false)
		}
		if childCfsm == "" {
			return fmt.Sprintf("%s %s\n", cfsm, qSend)
		}
		return fmt.Sprintf("%s %s", cfsm, childCfsm)

	case *RecvNode:
		fromCFSM, ok := cfsmByName[node.orig.Name()]
		if !ok {
			panic(fmt.Sprintf("Receiving from unknown channel: %s", node.orig.Name()))
		}

		recvType := encodeSymbols(node.orig.Type().String())
		qRecv := encodeSymbols(genNewState(role.Name()))
		cfsm := fmt.Sprintf("%s %d ? %s ", q0, fromCFSM, recvType)
		if !initial {
			cfsm = fmt.Sprintf("%s\n%s", q0, cfsm)
		}
		childCfsm := ""
		for _, child := range node.Children() {
			childCfsm += nodeToCFSM(child, role, qRecv, false)
		}
		if childCfsm == "" {
			return fmt.Sprintf("%s %s\n", cfsm, qRecv)
		}
		return fmt.Sprintf("%s %s", cfsm, childCfsm)

	case *EndNode:
		endCFSM, ok := cfsmByName[node.ch.Name()]
		if !ok {
			panic(fmt.Sprintf("Closing unknown channel: %s", node.ch.Name()))
		}

		qEnd := encodeSymbols(genNewState(role.Name()))
		cfsm := fmt.Sprintf("%s %d ! STOP %s", q0, endCFSM, qEnd)
		if !initial {
			cfsm = fmt.Sprintf("END %s\n%s", q0, cfsm)
		}
		childCfsm := ""
		for _, child := range node.Children() {
			childCfsm += nodeToCFSM(child, role, qEnd, false)
		}
		if childCfsm == "" {
			return fmt.Sprintf("%s %s\n", cfsm, qEnd)
		}
		return fmt.Sprintf("%s %s", cfsm, childCfsm)

	case *NewChanNode, *EmptyBodyNode:
		cfsm := ""
		for _, child := range node.Children() {
			cfsm += nodeToCFSM(child, role, q0, initial)
		}
		return cfsm

	case *LabelNode:
		labelJumpState[node.name] = q0
		cfsm := ""
		for _, child := range node.Children() {
			cfsm += nodeToCFSM(child, role, q0, initial)
		}
		return cfsm

	case *GotoNode:
		qJumpto := labelJumpState[node.name]
		cfsm := ""
		for _, child := range node.Children() {
			cfsm += nodeToCFSM(child, role, qJumpto, initial)
		}
		return cfsm

	default:
		panic(fmt.Sprintf("Unhandled node type: %T", node))
	}
}

func genCFSM(role Role, root Node) string {
	q0 := encodeSymbols(genNewState(role.Name()))
	cfsmBody := nodeToCFSM(root, role, q0, true)
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
