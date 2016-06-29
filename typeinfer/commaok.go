package typeinfer

// CommaOK tests.

import (
	"golang.org/x/tools/go/ssa"
)

// CommaOK is a struct to capture different kinds of the
// _, ok := instr syntax where instr can be TypeAssert, map Lookup or recv UnOp
type CommaOk struct {
	Instr  ssa.Instruction // TypeAssert, Lookup (map access) or UnOp (recv).
	Result VarInstance     // Result tuple { recvVal:T , recvTest:bool }.
	OkCond VarInstance     // The comma-ok condition.
}

func isCommaOk(f *Function, inst VarInstance) bool {
	for _, commaOk := range f.commaok {
		if commaOk.OkCond == inst {
			return true
		}
	}
	return false
}
