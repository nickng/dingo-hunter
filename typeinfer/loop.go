package typeinfer

// Loop related functions and utilities.

import (
	"fmt"
	"go/constant"
	"go/token"

	"golang.org/x/tools/go/ssa"
)

// Loop captures information about loop.
//
// A Loop context exists within a function, inside the scope of a for loop.
// Nested loops should be captured externally.
type Loop struct {
	Parent *Function // Enclosing function.
	Bound  LoopBound // Loop bound type.
	State  LoopState // Loop/Body/Done.

	IndexVar  ssa.Value // Variable holding the index (phi).
	CondVar   ssa.Value // Variable holding the cond expression.
	Index     int64     // Current index value.
	Start     int64     // Lower bound of index.
	Step      int64     // Increment (can be negative).
	End       int64     // Upper bound of index.
	LoopBlock int       // Block number of loop (with for.loop label).
}

// SetInit sets the loop index initial value (int).
func (l *Loop) SetInit(index ssa.Value, init int64) {
	l.IndexVar = index
	l.Start = init
	l.Index = init
}

// SetStep sets the loop index step value (int).
func (l *Loop) SetStep(step int64) {
	l.Step = step
}

// SetCond sets the loop exit condition (int).
func (l *Loop) SetCond(cond ssa.Value, max int64) {
	l.CondVar = cond
	l.End = max
}

// Next performs an index increment (e.g. i++) if possible.
func (l *Loop) Next() {
	if l.Bound == Static {
		l.Index += l.Step
	}
}

// HasNext returns true if the loop should continue.
func (l *Loop) HasNext() bool {
	if l.Bound == Static {
		return l.Start <= l.Index && l.Index <= l.End
	}
	return false
}

func (l *Loop) String() string {
	if l.Bound != Unknown && l.State != NonLoop {
		return fmt.Sprintf("%s: bound %s [%d..%d..%d] Step:%d", l.State, l.Bound, l.Start, l.Index, l.End, l.Step)
	}
	return fmt.Sprintf("%s: bound %s", l.State, l.Bound)
}

// loopSetIndex handles loop indices (initial value and increment)
func loopSetIndex(instr *ssa.Phi, infer *TypeInfer, ctx *Context) {
	if i, ok := instr.Edges[0].(*ssa.Const); ok && !i.IsNil() && i.Value.Kind() == constant.Int {
		ctx.L.SetInit(instr, i.Int64())
		infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"%s <= i", fmtLoopHL(ctx.L.Start)))
	}
	if bin, ok := instr.Edges[1].(*ssa.BinOp); ok {
		switch bin.Op {
		case token.ADD:
			if i, ok := bin.Y.(*ssa.Const); ok && i.Value.Kind() == constant.Int {
				ctx.L.SetStep(i.Int64())
			}
			infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"i += %s", fmtLoopHL(ctx.L.Step)))
		case token.SUB:
			if i, ok := bin.Y.(*ssa.Const); ok && i.Value.Kind() == constant.Int {
				ctx.L.SetStep(-i.Int64())
			}
			infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"i -= %s", fmtLoopHL(ctx.L.Step)))
		default:
			infer.Logger.Printf("loop index expression not supported %s", bin)
		}
	}
}

// loopDetectBounds detects static bounds (or set as dynamic bounds) from based
// on loop state machine.
func loopDetectBounds(instr *ssa.Phi, infer *TypeInfer, ctx *Context) {
	switch ctx.L.State {
	case Enter:
		switch ctx.L.Bound {
		case Unknown:
			phiSelectEdge(instr, infer, ctx)
			loopSetIndex(instr, infer, ctx)
		case Static:
			switch ctx.L.Bound {
			case Static:
				phiSelectEdge(instr, infer, ctx)
				if instr == ctx.L.IndexVar {
					ctx.L.Next()
					infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"Increment %s by %s to %s", ctx.L.IndexVar.Name(), fmtLoopHL(ctx.L.Step), fmtLoopHL(ctx.L.Index)))
				}
			default:
				visitSkip(instr, infer, ctx)
			}
		case Dynamic:
			phiSelectEdge(instr, infer, ctx)
			infer.Logger.Printf(ctx.F.Sprintf(PhiSymbol+"(dynamic bound) %s", instr.String()))
		}
	default:
		phiSelectEdge(instr, infer, ctx)
	}
}

// loopStateTransition updates loop transitions based on the state machine.
//
// ... NonLoop --> Enter --> Body --> Exit ...
//                       <--
func loopStateTransition(blk *ssa.BasicBlock, infer *TypeInfer, f *Function, l **Loop) {
	switch (*l).State {
	case NonLoop:
		if blk.Comment == "for.loop" {
			(*l).State = Enter
			(*l).Bound = Unknown
			(*l).LoopBlock = blk.Index
		}
		if blk.Comment == "for.body" {
			(*l).State = Body
			(*l).Bound = Unknown
			(*l).LoopBlock = blk.Index
		}
	case Enter:
		if blk.Comment == "for.body" {
			(*l).State = Body
		}
		if blk.Comment == "for.done" {
			(*l).State = Exit
			top, err := f.loopstack.Pop()
			if err != nil {
				return
			}
			*l = top
		}
	case Body:
		if blk.Comment == "for.loop" {
			if (*l).LoopBlock == blk.Index {
				// Back to loop init, but we don't need to find loop bounds
				(*l).State = Enter
			} else {
				if (*l).IndexVar != nil {
					infer.Logger.Print(f.Sprintf(LoopSymbol+"enter NESTED loop (%s)", (*l).IndexVar.Name()))
				} else {
					infer.Logger.Print(f.Sprintf(LoopSymbol + "enter NESTED loop"))
				}
				f.loopstack.Push(*l)
				*l = &Loop{Parent: f, Bound: Unknown, State: Enter, LoopBlock: blk.Index}
			}
		}
		if blk.Comment == "for.done" {
			(*l).State = Exit
		}
	case Exit:
		(*l).State = NonLoop
		(*l).Bound = Unknown
	}
}
