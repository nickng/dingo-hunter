// Package migo is a library for working with the MiGo language/type.
//
// MiGo is a language/type that captures the core concurrency feature of Go.
package migo

import (
	"bytes"
	"fmt"
	"go/token"

	"golang.org/x/tools/go/ssa"
)

// Program is a set of Functions in a program.
type Program struct {
	defs []*Function
}

// NewProgram creates a new empty Program.
func NewProgram() *Program { return &Program{[]*Function{}} }

// AddFunction adds a Function to Program.
//
// If Function already exists this does nothing.
func (p *Program) AddFunction(f *Function) {
	for _, def := range p.defs {
		if def.Name == f.Name {
			return
		}
	}
	p.defs = append(p.defs, f)
}

// Function gets a Function in a Program by name.
//
// Returns the function and a bool indicating whether lookup was successful.
func (p *Program) Function(name string) (*Function, bool) {
	for _, def := range p.defs {
		if def.Name == name {
			return def, true
		}
	}
	return nil, false
}

func (p *Program) String() string {
	var buf bytes.Buffer
	for _, d := range p.defs {
		if !d.IsEmpty() {
			buf.WriteString(d.String())
		}
	}
	return buf.String()
}

// Parameter is a translation from caller environment to callee.
type Parameter struct {
	Caller ssa.Value
	Callee ssa.Value
}

func (p *Parameter) String() string {
	return fmt.Sprintf("[%s → %s]", p.Caller.Name(), p.Callee.Name())
}

// CalleeParameterString converts a slice of *Parameter to parameter string.
func CalleeParameterString(params []*Parameter) string {
	var buf bytes.Buffer
	for i, p := range params {
		if i == 0 {
			buf.WriteString(p.Callee.Name())
		} else {
			buf.WriteString(fmt.Sprintf(", %s", p.Callee.Name()))
		}
	}
	return buf.String()
}

// CallerParameterString converts a slice of *Parameter to parameter string.
func CallerParameterString(params []*Parameter) string {
	var buf bytes.Buffer
	for i, p := range params {
		if i == 0 {
			buf.WriteString(p.Caller.Name())
		} else {
			buf.WriteString(fmt.Sprintf(", %s", p.Caller.Name()))
		}
	}
	return buf.String()
}

// Function is a block of Statements sharing the same parameters.
type Function struct {
	Name   string       // Name of the function.
	Params []*Parameter // Parameters (map from local variable name to Parameter).
	pos    token.Pos    // Position of the function in Go source code.
	Stmts  []Statement  // Function body (slice of statements).
	stack  *StmtsStack  // Stack for working with nested conditionals.
	varIdx int          // Next fresh variable index.
}

// NewFunction creates a new Function using the given name.
func NewFunction(name string) *Function {
	return &Function{
		Name:   name,
		Params: []*Parameter{},
		Stmts:  []Statement{},
		stack:  NewStmtsStack(),
	}
}

// AddParams adds Parameters to Function.
//
// If Parameter already exists this does nothing.
func (f *Function) AddParams(params ...*Parameter) {
	for _, param := range params {
		found := false
		for _, p := range f.Params {
			if p.Callee == param.Callee || p.Caller == param.Caller {
				found = true
			}
		}
		if !found {
			f.Params = append(f.Params, param)
		}
	}
}

// GetParamByCalleeValue is for looking up params from the body of a Function.
func (f *Function) GetParamByCalleeValue(v ssa.Value) (*Parameter, error) {
	for _, p := range f.Params {
		if p.Callee == v {
			return p, nil
		}
	}
	return nil, fmt.Errorf("Parameter not found")
}

// AddStmts add Statement(s) to a Function.
func (f *Function) AddStmts(stmts ...Statement) {
	numStmts := len(f.Stmts)
	if numStmts > 1 {
		if _, ok := f.Stmts[numStmts-1].(*TauStatement); ok {
			f.Stmts = append(f.Stmts[:numStmts], stmts...)
			return
		}
	}
	f.Stmts = append(f.Stmts, stmts...)
}

// IsEmpty returns true if the Function body is empty.
func (f *Function) IsEmpty() bool { return len(f.Stmts) == 0 }

// PutAway pushes current statements to stack.
func (f *Function) PutAway() {
	f.stack.Push(f.Stmts)
	f.Stmts = []Statement{}
}

// Restore pops current statements from stack.
func (f *Function) Restore() ([]Statement, error) { return f.stack.Pop() }

func (f *Function) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("def %s(%s):\n", f.Name, CalleeParameterString(f.Params)))
	for _, stmt := range f.Stmts {
		buf.WriteString(fmt.Sprintf(" %s;\n", stmt))
	}
	return buf.String()
}

// Statement is a generic statement.
type Statement interface {
	String() string
}

// CallStatement captures function calls or block jumps in the SSA.
type CallStatement struct {
	Name   string
	Params []*Parameter
}

func (s *CallStatement) String() string {
	return fmt.Sprintf("call %s(%s)", s.Name, CallerParameterString(s.Params))
}

// AddParams add parameter(s) to a Function call.
func (s *CallStatement) AddParams(params ...*Parameter) {
	for _, param := range params {
		found := false
		for _, p := range s.Params {
			if p == param {
				found = true
			}
		}
		if !found {
			s.Params = append(s.Params, param)
		}
	}
}

// CloseStatement closes a channel.
type CloseStatement struct {
	Chan string // Channel name
}

func (s *CloseStatement) String() string {
	return fmt.Sprintf("close %s", s.Chan)
}

// SpawnStatement captures spawning of goroutines.
type SpawnStatement struct {
	Name   string
	Params []*Parameter
}

func (s *SpawnStatement) String() string {
	return fmt.Sprintf("spawn %s(%s)", s.Name, CallerParameterString(s.Params))
}

// AddParams add parameter(s) to a goroutine spawning Function call.
func (s *SpawnStatement) AddParams(params ...*Parameter) {
	for _, param := range params {
		found := false
		for _, p := range s.Params {
			if p == param {
				found = true
			}
		}
		if !found {
			s.Params = append(s.Params, param)
		}
	}
}

// NewChanStatement creates and names a newly created channel.
type NewChanStatement struct {
	Name ssa.Value
	Chan string
	Size int64
}

func (s *NewChanStatement) String() string {
	return fmt.Sprintf("let %s = newchan %s, %d", s.Name.Name(), s.Chan, s.Size)
}

// IfStatement is a conditional statement.
//
// IfStatements always have both Then and Else.
type IfStatement struct {
	Then []Statement
	Else []Statement
}

func (s *IfStatement) String() string {
	var buf bytes.Buffer
	buf.WriteString("if ")
	for _, t := range s.Then {
		buf.WriteString(fmt.Sprintf("%s; ", t.String()))
	}
	buf.WriteString("else ")
	for _, f := range s.Else {
		buf.WriteString(fmt.Sprintf("%s; ", f.String()))
	}
	buf.WriteString("endif")
	return buf.String()
}

// SelectStatement is non-deterministic choice
type SelectStatement struct{ Cases [][]Statement }

func (s *SelectStatement) String() string {
	var buf bytes.Buffer
	buf.WriteString("select  ")
	for _, c := range s.Cases {
		buf.WriteString("case ")
		for _, stmt := range c {
			buf.WriteString(fmt.Sprintf("%s; ", stmt.String()))
		}
	}
	buf.WriteString(" endselect")
	return buf.String()
}

// TauStatement is inaction.
type TauStatement struct{}

func (s *TauStatement) String() string { return "tau" }

// SendStatement sends to Chan.
type SendStatement struct{ Chan string }

func (s *SendStatement) String() string { return fmt.Sprintf("send %s", s.Chan) }

// RecvStatement receives from Chan
type RecvStatement struct{ Chan string }

func (s *RecvStatement) String() string { return fmt.Sprintf("recv %s", s.Chan) }
