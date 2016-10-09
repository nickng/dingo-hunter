// Package migo is a library for working with the MiGo language/type.
//
// MiGo is a language/type that captures the core concurrency feature of Go.
package migo

import (
	"bytes"
	"fmt"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ssa"
)

var (
	nameFilter = strings.NewReplacer("(", "", ")", "", "*", "", "/", "_")
)

// Program is a set of Functions in a program.
type Program struct {
	Funcs   []*Function // Function definitions.
	visited map[*Function]int
}

// NewProgram creates a new empty Program.
func NewProgram() *Program {
	return &Program{Funcs: []*Function{}}
}

// AddFunction adds a Function to Program.
//
// If Function already exists this does nothing.
func (p *Program) AddFunction(f *Function) {
	for _, fun := range p.Funcs {
		if fun.Name == f.Name {
			return
		}
	}
	p.Funcs = append(p.Funcs, f)
}

// Function gets a Function in a Program by name.
//
// Returns the function and a bool indicating whether lookup was successful.
func (p *Program) Function(name string) (*Function, bool) {
	for _, f := range p.Funcs {
		if f.Name == name {
			return f, true
		}
	}
	return nil, false
}

func (p *Program) findEmptyFuncMain(f *Function) {
	known := make(map[string]bool)
	p.findEmptyFunc(f, known)
	f.HasComm = true
}

func (p *Program) findEmptyFunc(f *Function, known map[string]bool) {
	if _, ok := known[f.Name]; ok {
		return
	}
	known[f.Name] = f.HasComm
	for _, stmt := range f.Stmts {
		switch stmt := stmt.(type) {
		case *CallStatement:
			if child, ok := p.Function(stmt.Name); ok {
				if hasComm, ok := known[child.Name]; ok {
					f.HasComm = hasComm
				} else {
					p.findEmptyFunc(child, known)
					f.HasComm = f.HasComm || child.HasComm
				}
				known[f.Name] = f.HasComm
			}
		case *SpawnStatement:
			if child, ok := p.Function(stmt.Name); ok {
				if hasComm, ok := known[child.Name]; ok {
					f.HasComm = hasComm
				} else {
					p.findEmptyFunc(child, known)
					f.HasComm = f.HasComm || child.HasComm
				}
				known[f.Name] = f.HasComm
			}
		}
	}
}

// CleanUp removes empty functions.
func (p *Program) CleanUp() {
	// First remove all empty functions
	fns := []*Function{}
	validFns := make(map[string]bool) // Stores what functions are valid
	for i, f := range p.Funcs {
		if f.HasComm {
			fns = append(fns, p.Funcs[i])
			validFns[f.Name] = true
		}
	}
	p.Funcs = fns
	for _, f := range p.Funcs {
		stmts := []Statement{}
		for i, s := range f.Stmts {
			switch stmt := s.(type) {
			case *CallStatement:
				if _, ok := validFns[stmt.Name]; ok {
					stmts = append(stmts, f.Stmts[i])
				}
			case *SpawnStatement:
				if _, ok := validFns[stmt.Name]; ok {
					stmts = append(stmts, f.Stmts[i])
				}
			default:
				stmts = append(stmts, f.Stmts[i])
			}
		}
		f.Stmts = stmts
	}
}

func (p *Program) String() string {
	for _, f := range p.Funcs {
		if f.Name == "main.main" {
			p.findEmptyFuncMain(f)
		}
	}
	p.CleanUp()
	var buf bytes.Buffer
	for _, f := range p.Funcs {
		if !f.IsEmpty() {
			buf.WriteString(f.String())
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
	Name    string       // Name of the function.
	Params  []*Parameter // Parameters (map from local variable name to Parameter).
	Stmts   []Statement  // Function body (slice of statements).
	HasComm bool         // Does the function has communication statement?

	stack  *StmtsStack // Stack for working with nested conditionals.
	pos    token.Pos   // Position of the function in Go source code.
	varIdx int         // Next fresh variable index.
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

// SimpleName returns a filtered name of a function.
func (f *Function) SimpleName() string {
	return nameFilter.Replace(f.Name)
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
SET_HASCOMM:
	for _, s := range stmts {
		switch s.(type) {
		case *SendStatement, *RecvStatement, *CloseStatement, *SelectStatement, *NewChanStatement:
			f.HasComm = true
			break SET_HASCOMM
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
	buf.WriteString(fmt.Sprintf("def %s(%s):\n", f.SimpleName(), CalleeParameterString(f.Params)))
	for _, stmt := range f.Stmts {
		buf.WriteString(fmt.Sprintf("    %s;\n", stmt))
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

// SimpleName returns a filtered name.
func (s *CallStatement) SimpleName() string {
	return nameFilter.Replace(s.Name)
}

func (s *CallStatement) String() string {
	return fmt.Sprintf("call %s(%s)", s.SimpleName(), CallerParameterString(s.Params))
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

// SimpleName returns a filtered name.
func (s *SpawnStatement) SimpleName() string {
	return nameFilter.Replace(s.Name)
}

func (s *SpawnStatement) String() string {
	return fmt.Sprintf("spawn %s(%s)", s.SimpleName(), CallerParameterString(s.Params))
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
	return fmt.Sprintf("let %s = newchan %s, %d", s.Name.Name(), nameFilter.Replace(s.Chan), s.Size)
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
	buf.WriteString("select")
	for _, c := range s.Cases {
		buf.WriteString("\n      case ")
		for _, stmt := range c {
			buf.WriteString(fmt.Sprintf("%s; ", stmt.String()))
		}
	}
	buf.WriteString("\n    endselect")
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
