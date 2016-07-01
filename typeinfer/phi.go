package typeinfer

// Deal with Phi nodes.

import (
	"golang.org/x/tools/go/ssa"
)

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
