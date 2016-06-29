package typeinfer

import (
	"golang.org/x/tools/go/ssa"
)

// Elems are maps from array indices (variable) to VarInstances of elements.
type Elems map[ssa.Value]VarInstance
