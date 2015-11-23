package utils

import (
	"fmt"

	"golang.org/x/tools/go/ssa"
)

var (
	VarVers = make(map[ssa.Value]int)
)

// Variable definitions
type VarDef struct {
	Var ssa.Value
	Ver int
}

// NewVarDef creates a new variable definition from an ssa.Value
func NewVarDef(v ssa.Value) *VarDef {
	if v == nil {
		panic("NewVarDef: Cannot create new VarDef with nil")
	}
	if ver, ok := VarVers[v]; ok {
		VarVers[v]++
		return &VarDef{
			Var: v,
			Ver: ver + 1,
		}
	}
	VarVers[v] = 0
	return &VarDef{
		Var: v,
		Ver: 0,
	}
}

func (vd *VarDef) String() string {
	if vd == nil || vd.Var == nil {
		return "VarDef:nil"
	}
	if vd.Var.Parent() != nil {
		return fmt.Sprintf("%s.\033[4m%s\033[0m@%d", vd.Var.Parent().String(), vd.Var.Name(), vd.Ver)
	}
	return fmt.Sprintf("???.\033[4m%s\033[0m@%d", vd.Var.Name(), vd.Ver)
}
