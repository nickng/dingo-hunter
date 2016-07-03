package typeinfer

// Deal with Phi nodes.

import (
	"golang.org/x/tools/go/ssa"
)

// phiSelectEdge selects edge based on predecessor block and returns the edge index.
func phiSelectEdge(instr *ssa.Phi, infer *TypeInfer, f *Function, b *Block, l *Loop) (edge int) {
	for i, pred := range instr.Block().Preds {
		if pred.Index == b.Pred {
			e, ok := f.locals[instr.Edges[i]]
			if !ok {
				switch t := instr.Edges[i].(type) {
				case *ssa.Const:
					f.locals[instr.Edges[i]] = &ConstInstance{t}
					e, edge = f.locals[instr.Edges[i]], pred.Index
					infer.Logger.Printf(f.Sprintf(PhiSymbol+"%s/%s = %s, selected const from block %d", instr.Name(), e, instr.String(), edge))
				case *ssa.Function:
					f.locals[instr.Edges[i]] = &Instance{instr.Edges[i], f.InstanceID(), l.Index}
					e, edge = f.locals[instr.Edges[i]], pred.Index
					infer.Logger.Printf(f.Sprintf(PhiSymbol+"%s/%s = %s, selected function from block %d", instr.Name(), e, instr.String(), edge))
				case *ssa.Call:
					f.locals[instr.Edges[i]] = &Instance{instr.Edges[i], f.InstanceID(), l.Index}
					e, edge = f.locals[instr.Edges[i]], pred.Index
					infer.Logger.Printf(f.Sprintf(PhiSymbol+"%s/%s = %s, selected call from block %d", instr.Name(), e, instr.String(), edge))
				case *ssa.UnOp:
					f.locals[instr.Edges[i]] = &Instance{instr.Edges[i], f.InstanceID(), l.Index}
					e, edge = f.locals[instr.Edges[i]], pred.Index
					infer.Logger.Printf(f.Sprintf(PhiSymbol+"%s/%s = %s, selected UnOp from block %d", instr.Name(), e, instr.String(), edge))
				default:
					infer.Logger.Fatalf("phi: create instance Edge[%d]=%#v: %s", i, instr.Edges[i], ErrUnknownValue)
					return
				}
			}
			f.locals[instr], edge = e, pred.Index
			infer.Logger.Printf(f.Sprintf(PhiSymbol+"%s/%s = %s, selected from block %d", instr.Name(), e, instr.String(), edge))
			if a, ok := f.arrays[e]; ok {
				f.arrays[f.locals[instr]] = a
			}
			if s, ok := f.structs[e]; ok {
				f.structs[f.locals[instr]] = s
			}
			return
		}
	}
	infer.Logger.Fatalf("phi: %d->%d: %s", b.Pred, instr.Block().Index, ErrPhiUnknownEdge)
	return
}
