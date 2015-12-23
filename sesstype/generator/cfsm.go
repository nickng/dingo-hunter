package generator

import (
	"fmt"
	"os"

	"github.com/nickng/dingo-hunter/sesstype"
)

var (
	stateCount     = make(map[sesstype.Role]int)    // Contains next state number for each CFSM.
	labelJumpState = make(map[string]string)        // Convert label names to state names to jump to.
	finalState     = make(map[sesstype.Role]string) // Final states for each role.
	cfsmByName     = make(map[sesstype.Role]int)    // Converts role names to CFSM state number.
	chanByIndex    = make([]sesstype.Chan, 0)       // Index -> Chan
	cfsmCount      = 0
	chanCount      = 0
)

func genNewState(role sesstype.Role) string {
	stateIdx := stateCount[role]
	stateCount[role]++
	return fmt.Sprintf("q%d%d", cfsmByName[role], stateIdx)
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
func genChanCFSM(role sesstype.Role, typ string, begin int, end int) string {
	q0 := fmt.Sprintf("%s", encodeSymbols(genNewState(role)))
	qTerm := fmt.Sprintf("%s", encodeSymbols(genNewState(role)))
	cfsm := ""
	for i := begin; i < end; i++ {
		q1 := encodeSymbols(genNewState(role))
		cfsm += fmt.Sprintf("%s %d ? %s %s\n", q0, i, encodeSymbols(typ), q1)
		for j := begin; j < end; j++ {
			if i != j {
				cfsm += fmt.Sprintf("%s %d ! %s %s\n", q1, j, encodeSymbols(typ), q0)
			}
		}
		cfsm += fmt.Sprintf("%s %d ? STOP %s\n", q0, i, qTerm)
		for j := begin; j < end; j++ {
			if i != j {
				//cfsm += fmt.Sprintf("%s %d ! STOP %s\n", qTerm, j, qTerm)
				cfsm += fmt.Sprintf("%s %d ! %s %s\n", qTerm, j, encodeSymbols(typ), qTerm)
			}
		}
	}

	return fmt.Sprintf("-- channel %s\n.outputs\n.state graph\n%s.marking %s\n.end\n\n", role.Name(), cfsm, q0)
}

// nodeToCFSM creates CFSM states from sesstype.Node. q0 is already written.
func nodeToCFSM(root sesstype.Node, role sesstype.Role, q0 string, initial bool) string {
	switch node := root.(type) {
	case *sesstype.SendNode:
		toCFSM, ok := cfsmByName[node.To()]
		if !ok {
			panic(fmt.Sprintf("Sending to unknown channel: %s", node.To().Name()))
		}

		sendType := encodeSymbols(node.To().Type().String())
		qSend := encodeSymbols(genNewState(role))
		cfsm := fmt.Sprintf("%s %d ! %s ", q0, toCFSM, sendType)
		if !initial {
			cfsm = fmt.Sprintf("%s\n%s", q0, cfsm)
		}
		childrenCfsm := ""
		childInit := false
		for _, child := range node.Children() {
			childCfsm := nodeToCFSM(child, role, qSend, childInit)
			childInit = (childCfsm != "")
			childrenCfsm += childCfsm
		}
		if childrenCfsm == "" {
			if _, ok := finalState[role]; !ok {
				finalState[role] = qSend
			}
			return fmt.Sprintf("%s%s\n", cfsm, finalState[role])
		}
		return fmt.Sprintf("%s %s", cfsm, childrenCfsm)

	case *sesstype.RecvNode:
		fromCFSM, ok := cfsmByName[node.From()]
		if !ok {
			panic(fmt.Sprintf("Receiving from unknown channel: %s", node.From().Name()))
		}

		recvType := encodeSymbols(node.From().Type().String())
		qRecv := encodeSymbols(genNewState(role))
		cfsm := fmt.Sprintf("%s %d ? %s ", q0, fromCFSM, recvType)
		if !initial {
			cfsm = fmt.Sprintf("%s\n%s", q0, cfsm)
		}
		childrenCfsm, childInit := "", false
		for _, child := range node.Children() {
			childCfsm := nodeToCFSM(child, role, qRecv, childInit)
			childInit = (childCfsm != "")
			childrenCfsm += childCfsm
		}
		if childrenCfsm == "" {
			if _, ok := finalState[role]; !ok {
				finalState[role] = qRecv
			}
			return fmt.Sprintf("%s%s\n", cfsm, finalState[role])
		}
		return fmt.Sprintf("%s %s", cfsm, childrenCfsm)

	case *sesstype.EndNode:
		endCFSM, ok := cfsmByName[node.Chan()]
		if !ok {
			panic(fmt.Sprintf("Closing unknown channel: %s", node.Chan().Name()))
		}

		qEnd := encodeSymbols(genNewState(role))
		cfsm := fmt.Sprintf("%s %d ! STOP ", q0, endCFSM)
		if !initial {
			cfsm = fmt.Sprintf("%s\n%s", q0, cfsm)
		}
		childrenCfsm, childInit := "", false
		for _, child := range node.Children() {
			childCfsm := nodeToCFSM(child, role, qEnd, childInit)
			childInit = (childCfsm != "")
			childrenCfsm += childCfsm
		}
		if childrenCfsm == "" {
			if _, ok := finalState[role]; !ok {
				finalState[role] = qEnd
			}
			return fmt.Sprintf("%s%s\n", cfsm, finalState[role])
		}
		return fmt.Sprintf("%s %s", cfsm, childrenCfsm)

	case *sesstype.NewChanNode, *sesstype.EmptyBodyNode:
		cfsm, childInit := "", initial
		for _, child := range node.Children() {
			childCfsm := nodeToCFSM(child, role, q0, childInit)
			childInit = (childCfsm != "" || initial)
			cfsm += childCfsm
		}
		return cfsm

	case *sesstype.LabelNode:
		labelJumpState[node.Name()] = q0
		cfsm, childInit := "", initial
		//cfsm += fmt.Sprintf("Label{label:%s,state:%s}\n", node.Name(), q0)
		for _, child := range node.Children() {
			childCfsm := nodeToCFSM(child, role, q0, childInit)
			childInit = (childCfsm != "" || initial)
			cfsm += childCfsm
		}
		return cfsm

	case *sesstype.GotoNode:
		qJumpto := labelJumpState[node.Name()]
		cfsm, childInit := "", initial
		//cfsm += fmt.Sprintf("Goto{label:%s,state:%s}\n", node.Name(), qJumpto)
		for _, child := range node.Children() {
			// qJumpto written, so initial again
			childCfsm := nodeToCFSM(child, role, qJumpto, childInit)
			childInit = (childCfsm != "" || initial)
			cfsm += childCfsm
		}
		if cfsm == "" && !initial {
			return fmt.Sprintf("%s\n", qJumpto)
		}
		return cfsm

	default:
		panic(fmt.Sprintf("Unhandled node type: %T", node))
	}
}

func genCFSM(role sesstype.Role, root sesstype.Node) string {
	q0 := encodeSymbols(genNewState(role))
	cfsmBody := nodeToCFSM(root, role, q0, true)
	if cfsmBody == "" {
		return ""
	}
	return fmt.Sprintf("-- role %s\n.outputs\n.state graph\n%s.marking %s\n.end\n\n", role.Name(), cfsmBody, q0)
}

func PrintCFSMSummary() {
	fmt.Printf("Total of %d CFSMs (%d are channels)\n", cfsmCount, chanCount)
	for role, index := range cfsmByName {
		if index < chanCount {
			fmt.Printf("\t%d\t= %s (channel)\n", index, role.Name())
		} else {
			fmt.Printf("\t%d\t= %s\n", index, role.Name())
		}
	}
}

// GenAllCFSMs generates CFSMs for all roles in the session, plus the static
// CFSMs for the channels.
func getCFSMs(s *sesstype.Session) string {
	for _, c := range s.Chans {
		cfsmByName[c] = cfsmCount // For role CFSMs
		chanByIndex = append(chanByIndex, c)
		cfsmCount++
	}

	chanCount = cfsmCount
	roleCFSMs := ""
	for role, root := range s.Types {
		machine := genCFSM(role, root)
		fmt.Fprintf(os.Stderr, "Generate %s CFSM\n", role.Name())
		if machine != "" {
			cfsmByName[role] = cfsmCount
			roleCFSMs += fmt.Sprintf("-- %d\n", cfsmByName[role])
			roleCFSMs += machine
			cfsmCount++
		} else {
			fmt.Fprintf(os.Stderr, "  ^ Empty\n")
		}
	}

	chanCFSMs := ""
	for _, ch := range chanByIndex {
		chanCFSMs += fmt.Sprintf("-- %d\n", cfsmByName[ch])
		chanCFSMs += genChanCFSM(ch, ch.Type().String(), chanCount, chanCount+cfsmCount-chanCount)
	}

	return chanCFSMs + roleCFSMs
}
