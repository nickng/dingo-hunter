package typeinfer

import (
	"golang.org/x/tools/go/ssa"
)

// detectLoop updates loop transitions based on the state machine.
//
// ... NonLoop --> Enter --> Body --> Exit ...
//                       <--
func detectLoop(blk *ssa.BasicBlock, infer *TypeInfer, f *Function, l **Loop) {
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
				infer.Logger.Printf(LoopSymbol+"enter NESTED loop (%s)", (*l).IndexVar.Name())
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
