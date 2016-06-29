package typeinfer

import (
	"sync"
)

// LoopStack is a stack of ssa.BasicBlock
type LoopStack struct {
	sync.Mutex
	s []*Loop
}

// NewLoopStack creates a new LoopStack.
func NewLoopStack() *LoopStack {
	return &LoopStack{s: []*Loop{}}
}

// Push adds a new LoopContext to the top of stack.
func (s *LoopStack) Push(l *Loop) {
	s.Lock()
	defer s.Unlock()
	s.s = append(s.s, l)
}

// Pop removes a BasicBlock from top of stack.
func (s *LoopStack) Pop() (*Loop, error) {
	s.Lock()
	defer s.Unlock()

	size := len(s.s)
	if size == 0 {
		return nil, ErrEmptyStack
	}
	l := s.s[size-1]
	s.s = s.s[:size-1]
	return l, nil
}

// IsEmpty returns true if stack is empty.
func (s *LoopStack) IsEmpty() bool {
	return len(s.s) == 0
}
