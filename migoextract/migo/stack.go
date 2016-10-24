package migo

import (
	"errors"
	"sync"
)

var (
	// ErrEmptyStack is the error message if the Statement stack is empty.
	ErrEmptyStack = errors.New("stack: empty")
)

// StmtsStack is a stack of []Statement.
//
// StmtsStack is mostly used for building nested control-flow of the MiGo
// language.
type StmtsStack struct {
	sync.Mutex
	s [][]Statement
}

// NewStmtsStack creates a new StmtsStack.
func NewStmtsStack() *StmtsStack {
	return &StmtsStack{s: [][]Statement{}}
}

// Push adds a new Statement to the top of stack.
func (s *StmtsStack) Push(stmt []Statement) {
	s.Lock()
	defer s.Unlock()
	s.s = append(s.s, stmt)
}

// Pop removes and returns a Statement from top of stack.
func (s *StmtsStack) Pop() ([]Statement, error) {
	s.Lock()
	defer s.Unlock()

	size := len(s.s)
	if size == 0 {
		return nil, ErrEmptyStack
	}
	stmt := s.s[size-1]
	s.s = s.s[:size-1]
	return stmt, nil
}

// Size returns the number of elements in stack.
func (s *StmtsStack) Size() int {
	return len(s.s)
}

// IsEmpty returns true if stack is empty.
func (s *StmtsStack) IsEmpty() bool {
	return len(s.s) == 0
}
