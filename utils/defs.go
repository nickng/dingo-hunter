package utils

import (
	"fmt"

	"golang.org/x/tools/go/ssa"
)

var (
	VarVers = make(map[ssa.Value]int)
)

// Variable definitions
type Definition struct {
	Var ssa.Value
	Ver int
}

// NewVarDef creates a new variable definition from an ssa.Value
func NewDef(v ssa.Value) *Definition {
	if v == nil {
		panic("NewVarDef: Cannot create new VarDef with nil")
	}
	if ver, ok := VarVers[v]; ok {
		VarVers[v]++
		return &Definition{
			Var: v,
			Ver: ver + 1,
		}
	}
	VarVers[v] = 0
	return &Definition{
		Var: v,
		Ver: 0,
	}
}

func (vd *Definition) String() string {
	if vd == nil || vd.Var == nil {
		return "Undefined"
	}
	if vd.Var.Parent() != nil {
		return fmt.Sprintf("%s.%s@%d", vd.Var.Parent().String(), vd.Var.Name(), vd.Ver)
	}
	return fmt.Sprintf("???.%s@%d", vd.Var.Name(), vd.Ver)
}
