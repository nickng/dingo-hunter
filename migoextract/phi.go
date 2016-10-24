package migoextract

// Deal with Phi nodes.

import (
	"golang.org/x/tools/go/ssa"
)

// phiSelectEdge selects edge based on predecessor block and returns the edge index.
func phiSelectEdge(instr *ssa.Phi, infer *TypeInfer, ctx *Context) (edge int) {
	for i, pred := range instr.Block().Preds {
		if pred.Index == ctx.B.Pred {
			e, ok := ctx.F.locals[instr.Edges[i]]
			if !ok {
				switch t := instr.Edges[i].(type) {
				case *ssa.Const:
					ctx.F.locals[instr.Edges[i]] = &Const{t}
					e, edge = ctx.F.locals[instr.Edges[i]], pred.Index
					infer.Logger.Printf(ctx.F.Sprintf(PhiSymbol+"%s/%s = %s, selected const from block %d", instr.Name(), e, instr.String(), edge))
				case *ssa.Function:
					ctx.F.locals[instr.Edges[i]] = &Value{instr.Edges[i], ctx.F.InstanceID(), ctx.L.Index}
					e, edge = ctx.F.locals[instr.Edges[i]], pred.Index
					infer.Logger.Printf(ctx.F.Sprintf(PhiSymbol+"%s/%s = %s, selected function from block %d", instr.Name(), e, instr.String(), edge))
				case *ssa.Call:
					ctx.F.locals[instr.Edges[i]] = &Value{instr.Edges[i], ctx.F.InstanceID(), ctx.L.Index}
					e, edge = ctx.F.locals[instr.Edges[i]], pred.Index
					infer.Logger.Printf(ctx.F.Sprintf(PhiSymbol+"%s/%s = %s, selected call from block %d", instr.Name(), e, instr.String(), edge))
				case *ssa.UnOp:
					ctx.F.locals[instr.Edges[i]] = &Value{instr.Edges[i], ctx.F.InstanceID(), ctx.L.Index}
					e, edge = ctx.F.locals[instr.Edges[i]], pred.Index
					infer.Logger.Printf(ctx.F.Sprintf(PhiSymbol+"%s/%s = %s, selected UnOp from block %d", instr.Name(), e, instr.String(), edge))
				default:
					infer.Logger.Fatalf("phi: create instance Edge[%d]=%#v: %s", i, instr.Edges[i], ErrUnknownValue)
					return
				}
			}
			ctx.F.locals[instr], edge = e, pred.Index
			infer.Logger.Printf(ctx.F.Sprintf(PhiSymbol+"%s/%s = %s, selected from block %d", instr.Name(), e, instr.String(), edge))
			ctx.F.revlookup[instr.Name()] = instr.Edges[i].Name()
			if a, ok := ctx.F.arrays[e]; ok {
				ctx.F.arrays[ctx.F.locals[instr]] = a
			}
			if s, ok := ctx.F.structs[e]; ok {
				ctx.F.structs[ctx.F.locals[instr]] = s
			}
			return
		}
	}
	infer.Logger.Fatalf("phi: %d->%d: %s", ctx.B.Pred, instr.Block().Index, ErrPhiUnknownEdge)
	return
}
