package main

import (
	"go/token"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types"
)

var (
	_ ssa.Value = (*EmptyValue)(nil) // Make sure it implements ssa.Value.
)

// EmptyValue is a ssa.Value placeholder for values we don't care.
type EmptyValue struct {
	T types.Type
}

func (v EmptyValue) Name() string                  { return "(Nothingness)" }
func (v EmptyValue) String() string                { return "(Empty Value)" }
func (v EmptyValue) Type() types.Type              { return v.T }
func (v EmptyValue) Parent() *ssa.Function         { return nil }
func (v EmptyValue) Referrers() *[]ssa.Instruction { return nil }
func (v EmptyValue) Pos() token.Pos                { return token.NoPos }
