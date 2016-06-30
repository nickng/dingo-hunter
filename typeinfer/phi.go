package typeinfer

import (
	"go/constant"
	"go/token"

	"golang.org/x/tools/go/ssa"
)

// setLoopIndex handles loop indices (initial value and increment)
func setLoopIndex(instr *ssa.Phi, infer *TypeInfer, f *Function, b *Block, l *Loop) {
	if i, ok := instr.Edges[0].(*ssa.Const); ok && !i.IsNil() && i.Value.Kind() == constant.Int {
		l.SetInit(instr, i.Int64())
		infer.Logger.Printf(f.Sprintf(LoopSymbol+"%s <= i", fmtLoopHL(l.Start)))
	}
	if bin, ok := instr.Edges[1].(*ssa.BinOp); ok {
		switch bin.Op {
		case token.ADD:
			if i, ok := bin.Y.(*ssa.Const); ok && i.Value.Kind() == constant.Int {
				l.SetStep(i.Int64())
			}
			infer.Logger.Printf(f.Sprintf(LoopSymbol+"i += %s", fmtLoopHL(l.Step)))
		case token.SUB:
			if i, ok := bin.Y.(*ssa.Const); ok && i.Value.Kind() == constant.Int {
				l.SetStep(-i.Int64())
			}
			infer.Logger.Printf(f.Sprintf(LoopSymbol+"i -= %s", fmtLoopHL(l.Step)))
		default:
			infer.Logger.Printf("loop index expression not supported %s", bin)
		}
	}
}

func phiDetectLoop(instr *ssa.Phi, infer *TypeInfer, f *Function, b *Block, l *Loop) {
	switch l.State {
	case Enter:
		switch l.Bound {
		case Unknown:
			phiSelectEdge(instr, infer, f, b, l)
			setLoopIndex(instr, infer, f, b, l)
		case Static:
			switch l.Bound {
			case Static:
				phiSelectEdge(instr, infer, f, b, l)
				if instr == l.IndexVar {
					l.Next()
					infer.Logger.Printf(f.Sprintf(LoopSymbol+"Increment %s by %s to %s", l.IndexVar.Name(), fmtLoopHL(l.Step), fmtLoopHL(l.Index)))
				}
			default:
				visitSkip(instr, infer, f, b, l)
			}
		case Dynamic:
			phiSelectEdge(instr, infer, f, b, l)
			infer.Logger.Printf(f.Sprintf(PhiSymbol+"(dynamic bound) %s", instr.String()))
		}
	default:
		phiSelectEdge(instr, infer, f, b, l)
	}
}

// phiSelectEdge selects edge based on predecessor block and returns the edge index.
func phiSelectEdge(instr *ssa.Phi, infer *TypeInfer, f *Function, b *Block, l *Loop) int {
	edge := 0
	for i, pred := range instr.Block().Preds {
		if pred.Index == b.Pred {
			e, ok := f.locals[instr.Edges[i]]
			if !ok {
				c, ok := instr.Edges[i].(*ssa.Const)
				if !ok {
					infer.Logger.Fatalf("phi: create instance Edge[%d]=%s: %s", i, instr.Edges[i].String()+instr.Edges[i].Name(), ErrUnknownValue)
				}
				f.locals[instr], edge = &ConstInstance{c}, pred.Index
				infer.Logger.Printf(f.Sprintf(PhiSymbol+"%s/%s = %s, selected const from block %d", instr.Name(), f.locals[instr], instr.String(), edge))
				if a, ok := f.arrays[e]; ok {
					f.arrays[f.locals[instr]] = a
				}
				if s, ok := f.structs[e]; ok {
					f.structs[f.locals[instr]] = s
				}
				return edge
			}
			f.locals[instr], edge = e, pred.Index
			infer.Logger.Printf(f.Sprintf(PhiSymbol+"%s/%s = %s, selected from block %d", instr.Name(), f.locals[instr], instr.String(), edge))
			if a, ok := f.arrays[e]; ok {
				f.arrays[f.locals[instr]] = a
			}
			if s, ok := f.structs[e]; ok {
				f.structs[f.locals[instr]] = s
			}
			return edge
		}
	}
	infer.Logger.Fatalf("phi: %d->%d: %s", b.Pred, instr.Block().Index, ErrPhiUnknownEdge)
	return edge
}
