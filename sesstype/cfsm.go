package sesstype

import (
	"fmt"
	"os"
)

var (
	cfsmStateCount     = make(map[string]int)      // Contains next state number for each CFSM.
	cfsmByName         = make(map[string]int)      // Converts role names to CFSM state number.
	labelJumpState     = make(map[string]string)   // Convert label names to state names to jump to.
	statePendingLabels = make(map[string][]string) // List of labels waiting for the next state write.
	totalCFSMs         = 0                         // Number of CFSMs.
	chanCFSMs          = 0                         // Number of CFSMs for channels.
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
func genChanCFSM(name string, typ string) string {
	q0 := "CHAN" + encodeSymbols(genNewState(name))
	qTerm := "CHANCLOSE" + encodeSymbols(genNewState(name))
	cfsm := ".outputs\n.state graph\n"

	for i := chanCFSMs; i < totalCFSMs; i++ {
		// Received not Sent intermediate state.
		state := encodeSymbols(genNewState(name))
		cfsm += fmt.Sprintf("%s %d ? %s %s\n", q0, i, encodeSymbols(typ), state)
		for j := chanCFSMs; j < totalCFSMs; j++ {
			if i != j {
				cfsm += fmt.Sprintf("%s %d ! %s %s\n", state, j, encodeSymbols(typ), q0)
			}
		}
		// close(channel) is a special termination state.
		cfsm += fmt.Sprintf("%s %d ? STOP %s\n", q0, i, qTerm)
	}

	cfsm += fmt.Sprintf(".marking %s\n.end\n\n", q0)
	return cfsm
}

// nodeToCFSM creates a transition from state q0 which took prefix to the
// subtree rooted at root.
func nodeToCFSM(root Node, role Role, q0 string, prefix string) string {
	if q0 == "" {
		panic("q0 cannot be empty")
	}

	var state, action string
	cfsm := ""

	switch node := root.(type) {
	case *SendNode:
		to, ok := cfsmByName[node.dest.Name()]
		if !ok {
			panic(fmt.Sprintf("Sending to unknown channel: %s", node.dest.Name()))
		}

		if prefix != "" { // Write transition
			state = encodeSymbols(genNewState(role.Name()))
			// Update all labels waiting for a state to anchor on
			if labels, isLabel := statePendingLabels[q0]; isLabel {
				for _, label := range labels {
					labelJumpState[label] = state
				}
			}
			cfsm += fmt.Sprintf("%s %s %s\n", q0, encodeSymbols(prefix), state)
		} else { // No prefix (first action)
			state = q0
		}
		action = fmt.Sprintf("%d ! %s", to, "STYPE"+encodeSymbols(node.t.String()))

	case *RecvNode:
		from, ok := cfsmByName[node.orig.Name()]
		if !ok {
			panic(fmt.Sprintf("Receiving from unknown channel: %s", node.orig.Name()))
		}

		if prefix != "" { // Write transition
			state = encodeSymbols(genNewState(role.Name()))
			// Update all labels waiting for a state to anchor on
			if labels, isLabel := statePendingLabels[q0]; isLabel {
				for _, label := range labels {
					labelJumpState[label] = state
				}
			}
			cfsm += fmt.Sprintf("%s %s %s\n", q0, prefix, state)
		} else { // No prefix (first action)
			state = q0
		}
		action = fmt.Sprintf("%d ? %s", from, "TYPE"+encodeSymbols(node.t.String()))

	case *EmptyBodyNode:
		if len(root.Children()) > 0 { // Passthrough
			state = q0
			action = prefix
		} else { // Empty branch
			if prefix != "" {
				state = encodeSymbols(genNewState(role.Name()))
				cfsm += fmt.Sprintf("%s %s %s\n", q0, prefix, state)
			}
			return cfsm
		}

	case *NewChanNode: // Just skip
		state = q0
		action = prefix

	case *LabelNode:
		statePendingLabels[q0] = append(statePendingLabels[q0], node.name)
		state = q0
		action = prefix

	case *GotoNode:
		if st, ok := labelJumpState[node.name]; ok {
			state = st
			action = prefix
			if prefix != "" {
				cfsm += fmt.Sprintf("%s %s %s\n", q0, prefix, st)
			}
		}
		return cfsm // GoTo has no children (return early to skip empty nodes)

	case *EndNode:
		// Close a channel.
		state = encodeSymbols(genNewState(role.Name()))
		action = fmt.Sprintf("%s ! STOP", cfsmByName[node.ch.Name()])
		cfsm += fmt.Sprintf("%s %s %s\n", q0, prefix, state)

	default:
		panic(fmt.Sprintf("Unhandled node type: %T", node))
	}

	if len(root.Children()) == 0 {
		stateLast := encodeSymbols(genNewState(role.Name()))
		if action != "" { // Only if there are actions
			if labels, isLabel := statePendingLabels[state]; isLabel {
				for _, label := range labels {
					labelJumpState[label] = stateLast
				}
			}
			cfsm += fmt.Sprintf("%s %s %s\n", state, action, stateLast)
		}
		// Otherwise there is no communication
	} else {
		for _, child := range root.Children() {
			cfsm += nodeToCFSM(child, role, state, action)
		}
	}

	return cfsm
}

func genCFSM(role Role, root Node) string {
	q0 := encodeSymbols(genNewState(role.Name()))
	cfsm := ".outputs\n.state graph\n"
	cfsm += nodeToCFSM(root, role, q0, "")
	cfsm += fmt.Sprintf(".marking %s\n.end\n\n", q0)
	return cfsm
}

// Initialise the CFSM counts.
func initCFSMs(s *Session) {
	for _, c := range s.chans {
		cfsmByName[c.Name()] = totalCFSMs
		chanCFSMs++
		totalCFSMs++
	}

	for r := range s.types {
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
	for _, c := range s.chans {
		allCFSMs += genChanCFSM(c.Name(), c.Type().String())
	}

	for r, root := range s.types {
		allCFSMs += genCFSM(r, root)
	}

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
