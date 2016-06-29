package ssabuilder

// Channel helper functions.
// Most of the functions in this file are modified from golan.org/x/tools/oracle

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type ChanOpType int

const (
	ChanMake ChanOpType = iota
	ChanSend
	ChanRecv
	ChanClose
)

// ChanOp abstracts an ssa.Send, ssa.Unop(ARROW) or a SelectState.
type ChanOp struct {
	Value ssa.Value
	Type  ChanOpType
	Pos   token.Pos
}

// chanOps extract all channel operations from an instruction.
func chanOps(instr ssa.Instruction) []ChanOp {
	var ops []ChanOp
	switch instr := instr.(type) {
	case *ssa.Send:
		ops = append(ops, ChanOp{instr.Chan, ChanSend, instr.Pos()})
	case *ssa.UnOp:
		if instr.Op == token.ARROW {
			ops = append(ops, ChanOp{instr.X, ChanRecv, instr.Pos()})
		}
	case *ssa.Select:
		for _, st := range instr.States {
			switch st.Dir {
			case types.SendOnly:
				ops = append(ops, ChanOp{st.Chan, ChanSend, st.Pos})
			case types.RecvOnly:
				ops = append(ops, ChanOp{st.Chan, ChanRecv, st.Pos})
			}
		}
	case ssa.CallInstruction:
		common := instr.Common()
		if b, ok := common.Value.(*ssa.Builtin); ok && b.Name() == "close" {
			ops = append(ops, ChanOp{common.Args[0], ChanClose, common.Pos()})
		}
	}
	return ops
}

// progChanOps extract all channels from a program.
func progChanOps(prog *ssa.Program) []ChanOp {
	var ops []ChanOp // all sends/receives of opposite direction

	// Look at all channel operations in the whole ssa.Program.
	allFuncs := ssautil.AllFunctions(prog)
	for fn := range allFuncs {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				for _, op := range chanOps(instr) {
					ops = append(ops, op)
				}
			}
		}
	}
	return ops
}

// purgeChanOps removes channels that are of different type as queryOp, i.e.
// channel we are looking for.
func purgeChanOps(ops []ChanOp, ch ssa.Value) []ChanOp {
	i := 0
	for _, op := range ops {
		if types.Identical(op.Value.Type().Underlying().(*types.Chan).Elem(), ch.Type().Underlying().(*types.Chan).Elem()) {
			ops[i] = op
			i++
		}
	}
	ops = ops[:i]
	return ops
}
