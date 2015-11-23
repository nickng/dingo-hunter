package sesstype

import (
	"fmt"
)

func CountNodes(root Node) int {
	total := 1
	for _, child := range root.Children() {
		total += CountNodes(child)
	}
	return total
}

func SessionCountNodes(session *Session) map[string]int {
	m := make(map[string]int)
	for r, t := range session.Types {
		m[r.Name()] = CountNodes(t)
	}

	return m
}

func PrintNodeSummary(session *Session) {
	counts := SessionCountNodes(session)
	fmt.Printf("Total of nodes per role (%d roles)\n", len(counts))
	for role, n := range counts {
		fmt.Printf("\t%d\t: %s\n", n, role)
	}
}
