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
func loopSetIndex(instr *ssa.Phi, infer *TypeInfer, f *Function, b *Block, l *Loop) {
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

// loopDetectBounds detects static bounds (or set as dynamic bounds) from based
// on loop state machine.
func loopDetectBounds(instr *ssa.Phi, infer *TypeInfer, f *Function, b *Block, l *Loop) {
	switch l.State {
	case Enter:
		switch l.Bound {
		case Unknown:
			phiSelectEdge(instr, infer, f, b, l)
			loopSetIndex(instr, infer, f, b, l)
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
