package main

// Visitors for relevant SSA constructs

import (
	"fmt"
	"go/token"
	"os"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types"

	"github.com/nickng/dingo-hunter/sesstype"
)

// Continuation modelled after go/ssa continuation
type nextAction int

const (
	cont nextAction = iota // Continue processing next Inst
	done                   // Finish processing block (because of a jump/return)
)

func visitBlock(blk *ssa.BasicBlock, fr *frame) {
	fmt.Fprintf(os.Stderr, "  -- Block %d (in:%d, out:%d)\n", blk.Index, len(blk.Preds), len(blk.Succs))

	blkLabel := fmt.Sprintf("%s#%d", blk.Parent().String(), blk.Index)
	// Make a label for other edges that enter this block
	if len(blk.Preds) > 1 {
		if _, found := fr.gortn.visited[blk]; found {
			fr.gortn.append(sesstype.MkGotoNode(blkLabel))
			return
		}
		fr.gortn.visited[blk] = sesstype.MkLabelNode(blkLabel)
		fr.gortn.append(fr.gortn.visited[blk])
	}

	for _, inst := range blk.Instrs {
		switch visitInst(inst, fr) {
		case cont:
		case done:
			return // Jump out of visitBlock
		}
	}
}

// visitFunc is called to traverse a function using given callee frame
// Returns a boolean representing whether or not there are code in the func.
func visitFunc(fn *ssa.Function, callee *frame) bool {
	fmt.Fprintf(os.Stderr, " -- Enter Function %s()\n", fn.String())
	if fn.Blocks == nil {
		fmt.Fprintf(os.Stderr, "  # Ignore builtin/external '"+fn.String()+"' with no Blocks\n")
		return false
	}

	visitBlock(fn.Blocks[0], callee)
	return true
}

func visitInst(inst ssa.Instruction, fr *frame) nextAction {
	switch inst := inst.(type) {
	case *ssa.MakeChan:
		visitMakeChan(inst, fr)

	case *ssa.Send:
		visitSend(inst, fr)

	case *ssa.UnOp:
		switch inst.Op {
		case token.ARROW:
			visitRecv(inst, fr)
		case token.MUL:
			visitValueof(inst, fr)
		default:
			fmt.Fprintf(os.Stderr, "  # "+red("%s")+"\n", inst.String())
		}

	case *ssa.Call:
		visitCall(inst, fr)

	case *ssa.Extract:
		visitExtract(inst, fr)

	case *ssa.Go:
		callgo(inst.Call, fr, inst.Pos())

	case *ssa.Return:
		fr.retvals = visitReturn(inst, fr)

	case *ssa.Store:
		visitStore(inst, fr)

	case *ssa.Alloc:
		visitAlloc(inst, fr)

	case *ssa.MakeClosure:
		visitMakeClosure(inst, fr)

	case *ssa.Select:
		visitSelect(inst, fr)

	case *ssa.ChangeType:
		visitChangeType(inst, fr)

	case *ssa.If:
		visitIf(inst, fr)
		return done

	case *ssa.Jump:
		visitJump(inst, fr)
		return done

	default:
		// Everything else not handled yet
		fmt.Fprintf(os.Stderr, "  # "+red("%s")+"\n", inst.String())
	}

	return cont
}

func visitExtract(e *ssa.Extract, fr *frame) {
	if tpl, ok := fr.env.tuples[e.Tuple]; ok {
		fmt.Fprintf(os.Stderr, "  (Extract Tuple %s #%d == %s)\n", e.Tuple.Name(), e.Index, tpl[e.Index].String())
	} else {
		// Check if value is an external tuple (return value)
		if extType, isExtern := fr.env.extern[e.Tuple]; isExtern {
			if extTpl, isTuple := extType.(*types.Tuple); isTuple {
				if extTpl.Len() < e.Index {
					panic("Cannot extract from tuple " + e.Tuple.Name() + "\n")
				}
				// if extracted value is a chan create a new channel for it
				if t, ok := extTpl.At(e.Index).Type().(*types.Chan); ok {
					fr.env.chans[e] = fr.env.session.MakeChan(e.Tuple.Name(), fr.gortn.role, t.Elem())
				}
			}
			fmt.Fprintf(os.Stderr, "  (Extract Tuple %s #%d == %s)\n", e.Tuple.Name(), e.Index, tpl[e.Index].String())
		} else {
			fmt.Fprintf(os.Stderr, "  # "+red("%s")+" of type %s\n", e.String(), e.Type().String())
		}
	}
}

func visitMakeClosure(inst *ssa.MakeClosure, frm *frame) {
	// TODO(nickng) Do call but copy current local variables
	return
}

// visitAlloc is for variable allocation (usually by 'new')
// Registers created here are pointers
func visitAlloc(inst *ssa.Alloc, fr *frame) {
	t := inst.Type().Underlying().(*types.Pointer).Elem()
	if _, ok := t.(*types.Chan); ok {
		ch := fr.env.session.MakeChan(inst.Name(), fr.gortn.role, t)
		fr.env.chans[inst] = ch // Ptr to channel
	}
	fmt.Fprintf(os.Stderr, "  # "+red("Alloc %s")+" of type %s \n", inst.String(), t.String())
}

func visitValueof(inst *ssa.UnOp, fr *frame) {
	ptr := inst.X
	val := inst
	if ch, found := fr.env.chans[fr.get(ptr)]; found {
		fr.env.chans[val] = ch
	}
	fr.locals[val] = ptr
}

func visitSelect(s *ssa.Select, fr *frame) {
	parentNode := fr.gortn.end
	for _, state := range s.States {
		if ch, ok := fr.env.chans[fr.get(state.Chan)]; ok {
			switch state.Dir {
			case types.SendOnly:
				fr.gortn.end = parentNode.Append(sesstype.MkSelectSendNode(fr.gortn.role, ch))
				fmt.Fprintf(os.Stderr, "  select "+orange("%s")+"\n", fr.gortn.end.String())
				// TODO(nickng) continuation in this state
			case types.RecvOnly:
				fr.gortn.end = parentNode.Append(sesstype.MkSelectRecvNode(ch, fr.gortn.role))
				fmt.Fprintf(os.Stderr, "  select "+orange("%s")+"\n", fr.gortn.end.String())
				// TODO(nickng) continuation in this state
			default:
				panic("Cannot handle 'select' with SendRecv channels")
			}
		} else {
			panic("Channel " + state.Chan.Name() + " not found!\n")
		}
	}
}

func visitReturn(ret *ssa.Return, fr *frame) []ssa.Value {
	fmt.Printf(" -- Return from %s\n", fr.fn.String())
	//fr.gortn.append(sesstype.MkEndNode())
	return ret.Results
}

// Handles function call.
// Wrapper for calling visitFunc and performing argument translation.
func visitCall(c *ssa.Call, caller *frame) {
	call(c, caller)
}

func visitIf(inst *ssa.If, fr *frame) {
	if len(inst.Block().Succs) != 2 {
		panic("Cannot handle If with more or less than 2 successor blocks!")
	}

	ifparent := fr.gortn.end
	visitBlock(inst.Block().Succs[0], fr)

	fr.gortn.end = ifparent
	visitBlock(inst.Block().Succs[1], fr)

	// This is end of the block so continuation should not matter
}

func visitMakeChan(mc *ssa.MakeChan, fr *frame) {
	ch := fr.env.session.MakeChan(mc.Name(), fr.gortn.role, mc.Type())
	fr.env.chans[mc] = ch // Ptr to channel
	fr.gortn.append(sesstype.MkNewChanNode(ch))
}

func visitSend(send *ssa.Send, fr *frame) {
	if ch, ok := fr.env.chans[fr.get(send.Chan)]; ok {
		fr.gortn.append(sesstype.MkSendNode(fr.gortn.role, ch))
		fmt.Fprintf(os.Stderr, "  "+orange("%s")+"\n", fr.gortn.end.String())
	} else {
		fmt.Fprintf(os.Stderr, "Send%s: '%s' is not a channel", loc(fr.fn.Prog.Fset, send.Pos()), send.Chan.Name())
	}
}

func visitRecv(recv *ssa.UnOp, fr *frame) {
	if ch, ok := fr.env.chans[fr.get(recv.X)]; ok {
		fr.gortn.append(sesstype.MkRecvNode(ch, fr.gortn.role))
		fmt.Fprintf(os.Stderr, "  "+orange("%s")+"\n", fr.gortn.end.String())
	} else {
		fmt.Fprintf(os.Stderr, "Recv%s: '%s' is not a channel", loc(fr.fn.Prog.Fset, recv.Pos()), recv.X.Name())
	}
}

// visitClose for the close() builtin primitive.
func visitClose(ch sesstype.Chan,  fr *frame) {
	fmt.Fprintf(os.Stderr, " -- Enter close()\n")

	fr.gortn.append(sesstype.MkSendNode(fr.gortn.role, ch))
	fr.gortn.append(sesstype.MkEndNode())
}

func visitJump(inst *ssa.Jump, fr *frame) {
	fmt.Fprintf(os.Stderr, " -jump-> Block %d\n", inst.Block().Succs[0].Index)
	if len(inst.Block().Succs) != 1 {
		panic("Cannot Jump with multiple successors!")
	}
	visitBlock(inst.Block().Succs[0], fr)
}

func visitStore(inst *ssa.Store, fr *frame) {
	source := inst.Val
	dstPtr := inst.Addr
	fr.locals[dstPtr] = source
	if ch, found := fr.env.chans[fr.get(source)]; found {
		fr.env.chans[dstPtr] = ch
		fmt.Fprintf(os.Stderr, "  & store *%s -> channel %s\n", dstPtr.Name(), ch.Name())
	} else {
		fmt.Fprintf(os.Stderr, "  # "+red("store *%s = %s")+" of type %s\n", dstPtr.Name(), source.Name(), source.Type().String())
	}
}

func visitChangeType(inst *ssa.ChangeType, fr *frame) {
	if _, ok := inst.Type().(*types.Chan); ok {
		if ch, found := fr.env.chans[fr.get(inst.X)]; found {
			fr.env.chans[inst] = ch
			fmt.Fprintf(os.Stderr, "  & changetype from %s to %s (channel %s)\n", inst.X.Name(), inst.Name(), ch.Name())
		} else {
			panic("Channel " + inst.X.Name() + " not found!\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "  # "+red("%s")+"\n", inst.String())
	}
}
