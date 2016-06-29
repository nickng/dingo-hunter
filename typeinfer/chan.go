package typeinfer

// Utility functions to work with channels.

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

func getChan(val ssa.Value, infer *TypeInfer) ssa.Value {
	if _, ok := val.Type().(*types.Chan); ok {
		switch instr := val.(type) {
		case *ssa.ChangeType:
			return getChan(instr.X, infer)
		case *ssa.Parameter:
			return val // Maybe lookup from parent
		case *ssa.MakeChan:
			return val
		case *ssa.Phi:
			infer.Logger.Print("Phi:", val.String())
			return val //
		}
	}
	infer.Logger.Print("Don't know where this chan comes from:", val.String())
	return val
}
