package main

import (
	"fmt"
	"go/token"
	"go/types"
	"os"

	"github.com/nickng/dingo-hunter/sesstype"
	"github.com/nickng/dingo-hunter/utils"
	"golang.org/x/tools/go/ssa"
)

func visitBlock(blk *ssa.BasicBlock, fr *frame) {
	if len(blk.Preds) > 1 {
		blkLabel := fmt.Sprintf("%s#%d", blk.Parent().String(), blk.Index)

		if _, found := fr.gortn.visited[blk]; found {
			fr.gortn.AddNode(sesstype.NewGotoNode(blkLabel))
			return
		}
		// Make a label for other edges that enter this block
		label := sesstype.NewLabelNode(blkLabel)
		fr.gortn.AddNode(label)
		fr.gortn.visited[blk] = label // XXX visited is initialised by append if lblNode is head of tree
	}

	for _, inst := range blk.Instrs {
		visitInst(inst, fr)
	}
}

// visitFunc is called to traverse a function using given callee frame
// Returns a boolean representing whether or not there are code in the func.
func visitFunc(fn *ssa.Function, callee *frame) bool {
	if fn.Blocks == nil {
		//fmt.Fprintf(os.Stderr, "  # Ignore builtin/external '"+fn.String()+"' with no Blocks\n")
		return false
	}

	visitBlock(fn.Blocks[0], callee)
	return true
}

func visitInst(inst ssa.Instruction, fr *frame) {
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
			visitDeref(inst, fr)
		default:
			fmt.Fprintf(os.Stderr, "   # unhandled %s = %s\n", red(inst.Name()), red(inst.String()))
		}

	case *ssa.Call:
		visitCall(inst, fr)

	case *ssa.Extract:
		visitExtract(inst, fr)

	case *ssa.Go:
		fr.callGo(inst)

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

	case *ssa.ChangeInterface:
		visitChangeInterface(inst, fr)

	case *ssa.If:
		visitIf(inst, fr)

	case *ssa.Jump:
		visitJump(inst, fr)

	case *ssa.BinOp:
		visitBinOp(inst, fr)

	case *ssa.Slice:
		visitSlice(inst, fr)

	case *ssa.MakeSlice:
		visitMakeSlice(inst, fr)

	case *ssa.FieldAddr:
		visitFieldAddr(inst, fr)

	case *ssa.Field:
		visitField(inst, fr)

	case *ssa.IndexAddr:
		visitIndexAddr(inst, fr)

	case *ssa.Index:
		visitIndex(inst, fr)

	case *ssa.Defer:
		visitDefer(inst, fr)

	case *ssa.RunDefers:
		visitRunDefers(inst, fr)

	case *ssa.Phi:
		visitPhi(inst, fr)

	case *ssa.TypeAssert:
		visitTypeAssert(inst, fr)

	case *ssa.MakeInterface:
		visitMakeInterface(inst, fr)

	default:
		// Everything else not handled yet
		if v, ok := inst.(ssa.Value); ok {
			fmt.Fprintf(os.Stderr, "   # unhandled %s = %s\n", red(v.Name()), red(v.String()))
		} else {
			fmt.Fprintf(os.Stderr, "   # unhandled %s\n", red(inst.String()))
		}
	}
}

func visitExtract(e *ssa.Extract, fr *frame) {
	if recvCh, ok := fr.recvok[e.Tuple]; ok && e.Index == 1 { // 1 = ok (bool)
		fmt.Fprintf(os.Stderr, "  EXTRACT for %s\n", recvCh.Name())
		//fr.locals[e] = e
		fr.env.recvTest[e] = recvCh
		return
	}
	if tpl, ok := fr.tuples[e.Tuple]; ok {
		fmt.Fprintf(os.Stderr, "   %s = extract %s[#%d] == %s\n", reg(e), e.Tuple.Name(), e.Index, tpl[e.Index].String())
		fr.locals[e] = tpl[e.Index]
	} else {
		// Check if we are extracting select index
		if _, ok := fr.env.selNode[e.Tuple]; ok && e.Index == 0 {
			fmt.Fprintf(os.Stderr, "   | %s = select %s index\n", e.Name(), e.Tuple.Name())
			fr.env.selIdx[e] = e.Tuple
			return
		}
		// Check if value is an external tuple (return value)
		if extType, isExtern := fr.env.extern[e.Tuple]; isExtern {
			if extTpl, isTuple := extType.(*types.Tuple); isTuple {
				if extTpl.Len() < e.Index {
					panic(fmt.Sprintf("Extract: Cannot extract from tuple %s\n", e.Tuple.Name()))
				}
				// if extracted value is a chan create a new channel for it
				if _, ok := extTpl.At(e.Index).Type().(*types.Chan); ok {
					panic("Extract: Undefined channel")
				}
			}
			if e.Index < len(tpl) {
				fmt.Fprintf(os.Stderr, "  extract %s[#%d] == %s\n", e.Tuple.Name(), e.Index, tpl[e.Index].String())
			} else {
				fmt.Fprintf(os.Stderr, "  extract %s[#%d/%d]\n", e.Tuple.Name(), e.Index, len(tpl))
			}
		} else {
			fmt.Fprintf(os.Stderr, "   # %s = %s of type %s\n", e.Name(), red(e.String()), e.Type().String())
			switch derefAll(e.Type()).Underlying().(type) {
			case *types.Array:
				vd := utils.NewDef(e)
				fr.locals[e] = vd
				fr.arrays[vd] = make(Elems)
				fmt.Fprintf(os.Stderr, "     ^ local array (used as definition)\n")
			case *types.Struct:
				vd := utils.NewDef(e)
				fr.locals[e] = vd
				fr.structs[vd] = make(Fields)
				fmt.Fprintf(os.Stderr, "     ^ local struct (used as definition)\n")
			}
		}
	}
}

func visitMakeClosure(inst *ssa.MakeClosure, fr *frame) {
	fr.env.closures[inst] = make([]*utils.Definition, 0)
	for _, binding := range inst.Bindings {
		fr.env.closures[inst] = append(fr.env.closures[inst], fr.locals[binding])
	}
}

// visitAlloc is for variable allocation (usually by 'new')
// Everything allocated here are pointers
func visitAlloc(inst *ssa.Alloc, fr *frame) {
	locn := loc(fr, inst.Pos())
	allocType := inst.Type().(*types.Pointer).Elem()
	if allocType == nil {
		panic("Alloc: Cannot Alloc for non-pointer type")
	}
	var val ssa.Value = inst

	switch t := allocType.Underlying().(type) {
	case *types.Array:
		vd := utils.NewDef(val)
		fr.locals[val] = vd
		if inst.Heap {
			fr.env.arrays[vd] = make(Elems)
			fmt.Fprintf(os.Stderr, "   %s = Alloc (array@heap) of type %s (%d elems) at %s\n", cyan(reg(inst)), inst.Type().String(), t.Len(), locn)
		} else {
			fr.arrays[vd] = make(Elems)
			fmt.Fprintf(os.Stderr, "   %s = Alloc (array@local) of type %s (%d elems) at %s\n", cyan(reg(inst)), inst.Type().String(), t.Len(), locn)
		}

	case *types.Chan:
		// VD will be created in MakeChan so no need to allocate here.
		fmt.Fprintf(os.Stderr, "   %s = Alloc (chan) of type %s at %s\n", cyan(reg(inst)), inst.Type().String(), locn)

	case *types.Struct:
		vd := utils.NewDef(val)
		fr.locals[val] = vd
		if inst.Heap {
			fr.env.structs[vd] = make(Fields, t.NumFields())
			fmt.Fprintf(os.Stderr, "   %s = Alloc (struct@heap) of type %s (%d fields) at %s\n", cyan(reg(inst)), inst.Type().String(), t.NumFields(), locn)
		} else {
			fr.structs[vd] = make(Fields, t.NumFields())
			fmt.Fprintf(os.Stderr, "   %s = Alloc (struct@local) of type %s (%d fields) at %s\n", cyan(reg(inst)), inst.Type().String(), t.NumFields(), locn)
		}

	default:
		fmt.Fprintf(os.Stderr, "   # %s = "+red("Alloc %s")+" of type %s\n", inst.Name(), inst.String(), t.String())
	}
}

func visitDeref(inst *ssa.UnOp, fr *frame) {
	ptr := inst.X
	val := inst

	if _, ok := ptr.(*ssa.Global); ok {
		fr.locals[ptr] = fr.env.globals[ptr]
		fmt.Fprintf(os.Stderr, "   %s = *%s (global) of type %s\n", cyan(reg(val)), ptr.Name(), ptr.Type().String())
		fmt.Fprintf(os.Stderr, "    ^ i.e. %s\n", fr.locals[ptr].String())

		switch deref(fr.locals[ptr].Var.Type()).(type) {
		case *types.Array, *types.Slice:
			if _, ok := fr.env.arrays[fr.env.globals[ptr]]; !ok {
				fr.env.arrays[fr.env.globals[ptr]] = make(Elems)
			}

		case *types.Struct:
			if _, ok := fr.env.structs[fr.env.globals[ptr]]; !ok {
				fr.env.structs[fr.env.globals[ptr]] = make(Fields)
			}
		}
	}

	switch vd, kind := fr.get(ptr); kind {
	case Array, LocalArray:
		fr.locals[val] = vd
		fmt.Fprintf(os.Stderr, "   %s = *%s (array)\n", cyan(reg(val)), ptr.Name())

	case Struct, LocalStruct:
		fr.locals[val] = vd
		fmt.Fprintf(os.Stderr, "   %s = *%s (struct)\n", cyan(reg(val)), ptr.Name())

	case Chan:
		fr.locals[val] = vd
		fmt.Fprintf(os.Stderr, "   %s = *%s (previously initalised Chan)\n", cyan(reg(val)), ptr.Name())

	case Nothing:
		fmt.Fprintf(os.Stderr, "   # %s = *%s (not found)\n", red(inst.String()), red(inst.X.String()))
		if _, ok := val.Type().Underlying().(*types.Chan); ok {
			fmt.Fprintf(os.Stderr, "     ^ channel (not allocated, must be initialised by MakeChan)")
		}

	default:
		fmt.Fprintf(os.Stderr, "   # %s = *%s/%s (not found, type=%s)\n", red(inst.String()), red(inst.X.String()), reg(inst.X), inst.Type().String())
	}
}

func visitSelect(s *ssa.Select, fr *frame) {
	if fr.gortn.leaf == nil {
		panic("Select: Session head Node cannot be nil")
	}

	fr.env.selNode[s] = struct {
		parent   *sesstype.Node
		blocking bool
	}{
		fr.gortn.leaf,
		s.Blocking,
	}
	for _, state := range s.States {
		locn := loc(fr, state.Chan.Pos())
		switch vd, kind := fr.get(state.Chan); kind {
		case Chan:
			ch := fr.env.chans[vd]
			fmt.Fprintf(os.Stderr, "   select "+orange("%s")+" (%d states)\n", vd.String(), len(s.States))
			switch state.Dir {
			case types.SendOnly:
				fr.gortn.leaf = fr.env.selNode[s].parent
				fr.gortn.AddNode(sesstype.NewSelectSendNode(fr.gortn.role, *ch, state.Chan.Type()))
				fmt.Fprintf(os.Stderr, "    %s\n", orange((*fr.gortn.leaf).String()))

			case types.RecvOnly:
				fr.gortn.leaf = fr.env.selNode[s].parent
				fr.gortn.AddNode(sesstype.NewSelectRecvNode(*ch, fr.gortn.role, state.Chan.Type()))
				fmt.Fprintf(os.Stderr, "    %s\n", orange((*fr.gortn.leaf).String()))

			default:
				panic("Select: Cannot handle with SendRecv channels")
			}

		case Nothing:
			fr.printCallStack()
			panic(fmt.Sprintf("Select: Channel %s at %s is undefined", reg(state.Chan), locn))

		default:
			fr.printCallStack()
			panic(fmt.Sprintf("Select: Channel %s at %s is of wrong kind", reg(state.Chan), locn))
		}
	}
	if !s.Blocking { // Default state exists
		fr.gortn.leaf = fr.env.selNode[s].parent
		fr.gortn.AddNode(&sesstype.EmptyBodyNode{})
		fmt.Fprintf(os.Stderr, "    Default: %s\n", orange((*fr.gortn.leaf).String()))
	}
}

func visitReturn(ret *ssa.Return, fr *frame) []*utils.Definition {
	var vds []*utils.Definition
	for _, result := range ret.Results {
		vds = append(vds, fr.locals[result])
	}
	return vds
}

// Handles function call.
// Wrapper for calling visitFunc and performing argument translation.
func visitCall(c *ssa.Call, caller *frame) {
	caller.call(c)
}

func visitIf(inst *ssa.If, fr *frame) {
	if len(inst.Block().Succs) != 2 {
		panic("If: Cannot handle If with more or less than 2 successor blocks!")
	}

	ifparent := fr.gortn.leaf
	if ifparent == nil {
		panic("If: Parent is nil")
	}

	if ch, isRecvTest := fr.env.recvTest[inst.Cond]; isRecvTest {
		fmt.Fprintf(os.Stderr, "  @ Switch to recvtest true\n")
		fr.gortn.leaf = ifparent
		fr.gortn.AddNode(sesstype.NewRecvNode(*ch, fr.gortn.role, ch.Type()))
		fmt.Fprintf(os.Stderr, "  %s\n", orange((*fr.gortn.leaf).String()))
		visitBlock(inst.Block().Succs[0], fr)

		fmt.Fprintf(os.Stderr, "  @ Switch to recvtest false\n")
		fr.gortn.leaf = ifparent
		fr.gortn.AddNode(sesstype.NewRecvStopNode(*ch, fr.gortn.role, ch.Type()))
		fmt.Fprintf(os.Stderr, "  %s\n", orange((*fr.gortn.leaf).String()))
		visitBlock(inst.Block().Succs[1], fr)
	} else if selTest, isSelTest := fr.env.selTest[inst.Cond]; isSelTest {
		// Check if this is a select-test-jump, if so handle separately.
		fmt.Fprintf(os.Stderr, "  @ Switch to select branch #%d\n", selTest.idx)
		if selParent, ok := fr.env.selNode[selTest.tpl]; ok {
			fr.gortn.leaf = ifparent
			*fr.gortn.leaf = (*selParent.parent).Child(selTest.idx)
			visitBlock(inst.Block().Succs[0], fr)

			if !selParent.blocking && len((*selParent.parent).Children()) > selTest.idx+1 {
				*fr.gortn.leaf = (*selParent.parent).Child(selTest.idx + 1)
			}
			visitBlock(inst.Block().Succs[1], fr)
		} else {
			panic("Select without corresponding sesstype.Node")
		}
	} else {
		fr.env.ifparent.Push(*fr.gortn.leaf)

		parent := fr.env.ifparent.Top()
		fr.gortn.leaf = &parent
		fr.gortn.AddNode(&sesstype.EmptyBodyNode{})
		visitBlock(inst.Block().Succs[0], fr)

		parent = fr.env.ifparent.Top()
		fr.gortn.leaf = &parent
		fr.gortn.AddNode(&sesstype.EmptyBodyNode{})
		visitBlock(inst.Block().Succs[1], fr)

		fr.env.ifparent.Pop()
	}
	// This is end of the block so no continuation
}

func visitMakeChan(inst *ssa.MakeChan, caller *frame) {
	locn := loc(caller, inst.Pos())
	role := caller.gortn.role

	vd := utils.NewDef(inst) // Unique identifier for inst
	ch := caller.env.session.MakeChan(vd, role)

	caller.env.chans[vd] = &ch
	caller.gortn.AddNode(sesstype.NewNewChanNode(ch))
	caller.locals[inst] = vd
	fmt.Fprintf(os.Stderr, "   New channel %s { type: %s } by %s at %s\n", green(ch.Name()), ch.Type(), vd.String(), locn)
	fmt.Fprintf(os.Stderr, "               ^ in role %s\n", role.Name())
}

func visitSend(send *ssa.Send, fr *frame) {
	locn := loc(fr, send.Chan.Pos())
	if vd, kind := fr.get(send.Chan); kind == Chan {
		ch := fr.env.chans[vd]
		fr.gortn.AddNode(sesstype.NewSendNode(fr.gortn.role, *ch, send.Chan.Type()))
		fmt.Fprintf(os.Stderr, "  %s\n", orange((*fr.gortn.leaf).String()))
	} else if kind == Nothing {
		fr.locals[send.Chan] = utils.NewDef(send.Chan)
		ch := fr.env.session.MakeExtChan(fr.locals[send.Chan], fr.gortn.role)
		fr.env.chans[fr.locals[send.Chan]] = &ch
		fr.gortn.AddNode(sesstype.NewSendNode(fr.gortn.role, ch, send.Chan.Type()))
		fmt.Fprintf(os.Stderr, "  %s\n", orange((*fr.gortn.leaf).String()))
		fmt.Fprintf(os.Stderr, "   ^ Send: Channel %s at %s is external\n", reg(send.Chan), locn)
	} else {
		fr.printCallStack()
		panic(fmt.Sprintf("Send: Channel %s at %s is of wrong kind", reg(send.Chan), locn))
	}
}

func visitRecv(recv *ssa.UnOp, fr *frame) {
	locn := loc(fr, recv.X.Pos())
	if vd, kind := fr.get(recv.X); kind == Chan {
		ch := fr.env.chans[vd]
		if recv.CommaOk {
			// ReceiveOK test
			fr.recvok[recv] = ch
			// TODO(nickng) technically this should do receive (both branches)
		} else {
			// Normal receive
			fr.gortn.AddNode(sesstype.NewRecvNode(*ch, fr.gortn.role, recv.X.Type()))
			fmt.Fprintf(os.Stderr, "  %s\n", orange((*fr.gortn.leaf).String()))
		}
	} else if kind == Nothing {
		fr.locals[recv.X] = utils.NewDef(recv.X)
		ch := fr.env.session.MakeExtChan(fr.locals[recv.X], fr.gortn.role)
		fr.env.chans[fr.locals[recv.X]] = &ch
		fr.gortn.AddNode(sesstype.NewRecvNode(ch, fr.gortn.role, recv.X.Type()))
		fmt.Fprintf(os.Stderr, "  %s\n", orange((*fr.gortn.leaf).String()))
		fmt.Fprintf(os.Stderr, "   ^ Recv: Channel %s at %s is external\n", reg(recv.X), locn)
	} else {
		fr.printCallStack()
		panic(fmt.Sprintf("Recv: Channel %s at %s is of wrong kind", reg(recv.X), locn))
	}
}

// visitClose for the close() builtin primitive.
func visitClose(ch sesstype.Chan, fr *frame) {
	fr.gortn.AddNode(sesstype.NewEndNode(ch))
}

func visitJump(inst *ssa.Jump, fr *frame) {
	//fmt.Fprintf(os.Stderr, " -jump-> Block %d\n", inst.Block().Succs[0].Index)
	if len(inst.Block().Succs) != 1 {
		panic("Cannot Jump with multiple successors!")
	}
	visitBlock(inst.Block().Succs[0], fr)
}

func visitStore(inst *ssa.Store, fr *frame) {
	source := inst.Val
	dstPtr := inst.Addr // from Alloc or field/elem access

	if _, ok := dstPtr.(*ssa.Global); ok {
		vdOld, _ := fr.env.globals[dstPtr]
		switch vd, kind := fr.get(source); kind {
		case Array:
			fr.env.globals[dstPtr] = vd
			fr.updateDefs(vdOld, vd)
			fmt.Fprintf(os.Stderr, "   # store (global) *%s = %s of type %s\n", dstPtr.String(), source.Name(), source.Type().String())

		case Struct:
			fr.env.globals[dstPtr] = vd
			fr.updateDefs(vdOld, vd)
			fmt.Fprintf(os.Stderr, "   # store (global) *%s = %s of type %s\n", reg(dstPtr), reg(source), source.Type().String())

		default:
			fmt.Fprintf(os.Stderr, "   # store (global) *%s = %s of type %s\n", red(reg(dstPtr)), reg(source), source.Type().String())
		}
	} else {
		vdOld, _ := fr.get(dstPtr)
		switch vd, kind := fr.get(source); kind {
		case Array:
			// Pre: fr.locals[source] points to vd
			// Pre: fr.locals[dstPtr] points to (empty/outdated) vdOld
			// Post: fr.locals[source] unchanged
			// Post: fr.locals[dstPtr] points to vd
			fr.locals[dstPtr] = vd   // was vdOld
			fr.updateDefs(vdOld, vd) // Update all references to vdOld to vd
			fmt.Fprintf(os.Stderr, "   # store array *%s = %s of type %s\n", cyan(reg(dstPtr)), reg(source), source.Type().String())

		case LocalArray:
			fr.locals[dstPtr] = vd
			fr.updateDefs(vdOld, vd)
			fmt.Fprintf(os.Stderr, "   store larray *%s = %s of type %s\n", cyan(reg(dstPtr)), reg(source), source.Type().String())

		case Chan:
			fr.locals[dstPtr] = vd
			fr.updateDefs(vdOld, vd)
			fmt.Fprintf(os.Stderr, "   store chan *%s = %s of type %s\n", cyan(reg(dstPtr)), reg(source), source.Type().String())

		case Struct:
			fr.locals[dstPtr] = vd
			fr.updateDefs(vdOld, vd)
			fmt.Fprintf(os.Stderr, "   store struct *%s = %s of type %s\n", cyan(reg(dstPtr)), reg(source), source.Type().String())

		case LocalStruct:
			fr.locals[dstPtr] = vd
			fr.updateDefs(vdOld, vd)
			fmt.Fprintf(os.Stderr, "   store lstruct *%s = %s of type %s\n", cyan(reg(dstPtr)), reg(source), source.Type().String())

		case Untracked:
			fr.locals[dstPtr] = vd
			fmt.Fprintf(os.Stderr, "   store update *%s = %s of type %s\n", cyan(reg(dstPtr)), reg(source), source.Type().String())

		case Nothing:
			fmt.Fprintf(os.Stderr, "   # store *%s = %s of type %s\n", red(reg(dstPtr)), reg(source), source.Type().String())

		default:
			fr.locals[dstPtr] = vd
			fmt.Fprintf(os.Stderr, "   store *%s = %s of type %s\n", cyan(reg(dstPtr)), reg(source), source.Type().String())
		}

	}
}

func visitChangeType(inst *ssa.ChangeType, fr *frame) {
	switch vd, kind := fr.get(inst.X); kind {
	case Chan:
		fr.locals[inst] = vd // ChangeType from <-chan and chan<-
		ch := fr.env.chans[vd]
		fmt.Fprintf(os.Stderr, "   & changetype from %s to %s (channel %s)\n", green(reg(inst.X)), reg(inst), ch.Name())
		fmt.Fprintf(os.Stderr, "                      ^ origin\n")

	case Nothing:
		fmt.Fprintf(os.Stderr, "   # changetype %s = %s %s\n", inst.Name(), inst.X.Name(), inst.String())
		fmt.Fprintf(os.Stderr, "          ^ unknown kind\n")

	default:
		fr.locals[inst] = vd
		fmt.Fprintf(os.Stderr, "   # changetype %s = %s\n", red(inst.Name()), inst.String())
	}
}

func visitChangeInterface(inst *ssa.ChangeInterface, fr *frame) {
	fr.locals[inst] = fr.locals[inst.X]
	fmt.Fprintf(os.Stderr, "   # changeinterface %s = %s\n", reg(inst), inst.String())
}

func visitBinOp(inst *ssa.BinOp, fr *frame) {
	switch inst.Op {
	case token.EQL:
		if selTuple, isSelTuple := fr.env.selIdx[inst.X]; isSelTuple {
			branchID := int(inst.Y.(*ssa.Const).Int64())
			fr.env.selTest[inst] = struct {
				idx int
				tpl ssa.Value
			}{
				branchID, selTuple,
			}
		} else {
			fmt.Fprintf(os.Stderr, "   # %s = "+red("%s")+"\n", inst.Name(), inst.String())
		}
	default:
		fmt.Fprintf(os.Stderr, "   # %s = "+red("%s")+"\n", inst.Name(), inst.String())
	}
}

func visitMakeInterface(inst *ssa.MakeInterface, fr *frame) {
	switch vd, kind := fr.get(inst.X); kind {
	case Struct, LocalStruct:
		fmt.Fprintf(os.Stderr, "   %s <-(struct/iface)- %s %s = %s\n", cyan(reg(inst)), reg(inst.X), inst.String(), vd.String())
		fr.locals[inst] = vd

	case Array, LocalArray:
		fmt.Fprintf(os.Stderr, "   %s <-(array/iface)- %s %s = %s\n", cyan(reg(inst)), reg(inst.X), inst.String(), vd.String())
		fr.locals[inst] = vd

	default:
		fmt.Fprintf(os.Stderr, "   # %s <- %s\n", red(reg(inst)), inst.String())
	}
}

func visitSlice(inst *ssa.Slice, fr *frame) {
	fr.env.arrays[utils.NewDef(inst)] = make(Elems)
}

func visitMakeSlice(inst *ssa.MakeSlice, fr *frame) {
	fr.env.arrays[utils.NewDef(inst)] = make(Elems)
}

func visitFieldAddr(inst *ssa.FieldAddr, fr *frame) {
	field := inst
	struc := inst.X
	index := inst.Field

	if stype, ok := deref(struc.Type()).Underlying().(*types.Struct); ok {
		switch vd, kind := fr.get(struc); kind {
		case Struct:
			fmt.Fprintf(os.Stderr, "   %s = %s(=%s)->[%d] of type %s\n", cyan(reg(field)), struc.Name(), vd.String(), index, field.Type().String())
			if fr.env.structs[vd][index] == nil { // First use
				vdField := utils.NewDef(field)
				fr.env.structs[vd][index] = vdField
				fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as field definition\n", field.Name())
				// If field is struct
				if fieldType, ok := deref(field.Type()).Underlying().(*types.Struct); ok {
					fr.env.structs[vdField] = make(Fields, fieldType.NumFields())
					fmt.Fprintf(os.Stderr, "     ^ field %s is a struct (allocating)\n", field.Name())
				}
			} else if fr.env.structs[vd][index].Var != field { // Previously defined
				fmt.Fprintf(os.Stderr, "     ^ field %s previously defined as %s\n", field.Name(), reg(fr.env.structs[vd][index].Var))
			} // else Accessed before (and unchanged)
			fr.locals[field] = fr.env.structs[vd][index]

		case LocalStruct:
			fmt.Fprintf(os.Stderr, "   %s = %s(=%s)->[%d] (local) of type %s\n", cyan(reg(field)), struc.Name(), vd.String(), index, field.Type().String())
			if fr.structs[vd][index] == nil { // First use
				vdField := utils.NewDef(field)
				fr.structs[vd][index] = vdField
				fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as field definition\n", field.Name())
				// If field is struct
				if fieldType, ok := deref(field.Type()).Underlying().(*types.Struct); ok {
					fr.structs[vdField] = make(Fields, fieldType.NumFields())
					fmt.Fprintf(os.Stderr, "     ^ field %s is a struct (allocating locally)\n", field.Name())
				}
			} else if fr.structs[vd][index].Var != field { // Previously defined
				fmt.Fprintf(os.Stderr, "     ^ field %s previously defined as %s\n", field.Name(), reg(fr.structs[vd][index].Var))
			} // else Accessed before (and unchanged)
			fr.locals[field] = fr.structs[vd][index]

		case Nothing, Untracked:
			// Nothing: Very likely external struct.
			// Untracked: likely branches of return values (e.g. returning nil)
			fmt.Fprintf(os.Stderr, "   %s = %s(=%s)->[%d] (external) of type %s\n", cyan(reg(field)), inst.X.Name(), vd.String(), index, field.Type().String())
			vd := utils.NewDef(struc) // New external struct
			fr.locals[struc] = vd
			fr.env.structs[vd] = make(Fields, stype.NumFields())
			vdField := utils.NewDef(field) // New external field
			fr.env.structs[vd][index] = vdField
			fr.locals[field] = vdField
			fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as field definition of type %s\n", field.Name(), inst.Type().(*types.Pointer).Elem().Underlying().String())
			// If field is struct
			if fieldType, ok := deref(field.Type()).Underlying().(*types.Struct); ok {
				fr.env.structs[vdField] = make(Fields, fieldType.NumFields())
				fmt.Fprintf(os.Stderr, "     ^ field %s previously defined as %s\n", field.Name(), reg(fr.env.structs[vd][index].Var))
			}

		default:
			panic(fmt.Sprintf("FieldAddr: Cannot access non-struct %s %T %d", reg(struc), deref(struc.Type()).Underlying(), kind))
		}
	} else {
		panic(fmt.Sprintf("FieldAddr: Cannot access field - %s not a struct\n", reg(struc)))
	}
}

func visitField(inst *ssa.Field, fr *frame) {
	field := inst
	struc := inst.X
	index := inst.Field

	if stype, ok := struc.Type().Underlying().(*types.Struct); ok {
		switch vd, kind := fr.get(struc); kind {
		case Struct:
			fmt.Fprintf(os.Stderr, "   %s = %s(=%s).[%d] of type %s\n", cyan(reg(field)), struc.Name(), vd.String(), index, field.Type().String())
			if fr.env.structs[vd][index] == nil { // First use
				vdField := utils.NewDef(field)
				fr.env.structs[vd][index] = vdField
				fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as field definition\n", field.Name())
				// If field is struct
				if fieldType, ok := field.Type().Underlying().(*types.Struct); ok {
					fr.env.structs[vdField] = make(Fields, fieldType.NumFields())
					fmt.Fprintf(os.Stderr, "     ^ field %s is a struct (allocating)\n", field.Name())
				}
			} else if fr.env.structs[vd][index].Var != field { // Previously defined
				fmt.Fprintf(os.Stderr, "     ^ field %s previously defined as %s\n", field.Name(), reg(fr.env.structs[vd][index].Var))
			} // else Accessed before (and unchanged)
			fr.locals[field] = fr.env.structs[vd][index]

		case LocalStruct:
			fmt.Fprintf(os.Stderr, "   %s = %s(=%s).[%d] (local) of type %s\n", cyan(reg(field)), struc.Name(), vd.String(), index, field.Type().String())
			if fr.structs[vd][index] == nil { // First use
				vdField := utils.NewDef(field)
				fr.structs[vd][index] = vdField
				fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as field definition\n", field.Name())
				// If field is struct
				if fieldType, ok := field.Type().Underlying().(*types.Struct); ok {
					fr.structs[vdField] = make(Fields, fieldType.NumFields())
					fmt.Fprintf(os.Stderr, "     ^ field %s is a struct (allocating locally)\n", field.Name())
				}
			} else if fr.structs[vd][index].Var != field { // Previously defined
				fmt.Fprintf(os.Stderr, "     ^ field %s previously defined as %s\n", field.Name(), reg(fr.structs[vd][index].Var))
			} // else Accessed before (and unchanged)
			fr.locals[field] = fr.structs[vd][index]

		case Nothing, Untracked:
			// Nothing: Very likely external struct.
			// Untracked: likely branches of return values (e.g. returning nil)
			fmt.Fprintf(os.Stderr, "   %s = %s(=%s).[%d] (external) of type %s\n", cyan(reg(field)), inst.X.Name(), vd.String(), index, field.Type().String())
			vd := utils.NewDef(struc) // New external struct
			fr.locals[struc] = vd
			fr.env.structs[vd] = make(Fields, stype.NumFields())
			vdField := utils.NewDef(field) // New external field
			fr.env.structs[vd][index] = vdField
			fr.locals[field] = vdField
			fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as field definition of type %s\n", field.Name(), inst.Type().Underlying().String())
			// If field is struct
			if fieldType, ok := field.Type().Underlying().(*types.Struct); ok {
				fr.env.structs[vdField] = make(Fields, fieldType.NumFields())
				fmt.Fprintf(os.Stderr, "     ^ field %s previously defined as %s\n", field.Name(), reg(fr.env.structs[vd][index].Var))
			}

		default:
			panic(fmt.Sprintf("Field: Cannot access non-struct %s %T %d", reg(struc), struc.Type(), kind))
		}
	} else {
		panic(fmt.Sprintf("Field: Cannot access field - %s not a struct\n", reg(struc)))
	}
}

func visitIndexAddr(inst *ssa.IndexAddr, fr *frame) {
	elem := inst
	array := inst.X
	index := inst.Index
	_, isArray := deref(array.Type()).Underlying().(*types.Array)
	_, isSlice := deref(array.Type()).Underlying().(*types.Slice)

	if isArray || isSlice {
		switch vd, kind := fr.get(array); kind {
		case Array:
			fmt.Fprintf(os.Stderr, "   %s = &%s(=%s)[%d] of type %s\n", cyan(reg(elem)), array.Name(), vd.String(), index, elem.Type().String())
			if fr.env.arrays[vd][index] == nil { // First use
				vdelem := utils.NewDef(elem)
				fr.env.arrays[vd][index] = vdelem
				fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as elem definition\n", elem.Name())
			} else if fr.env.arrays[vd][index].Var != elem { // Previously defined
				fmt.Fprintf(os.Stderr, "     ^ elem %s previously defined as %s\n", elem.Name(), reg(fr.env.arrays[vd][index].Var))
			} // else Accessed before (and unchanged)
			fr.locals[elem] = fr.env.arrays[vd][index]

		case LocalArray:
			fmt.Fprintf(os.Stderr, "   %s = &%s(=%s)[%d] (local) of type %s\n", cyan(reg(elem)), array.Name(), vd.String(), index, elem.Type().String())
			if fr.arrays[vd][index] == nil { // First use
				vdElem := utils.NewDef(elem)
				fr.arrays[vd][index] = vdElem
				fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as elem definition\n", elem.Name())
			} else if fr.arrays[vd][index].Var != elem { // Previously defined
				fmt.Fprintf(os.Stderr, "     ^ elem %s previously defined as %s\n", elem.Name(), reg(fr.arrays[vd][index].Var))
			} // else Accessed before (and unchanged)
			fr.locals[elem] = fr.arrays[vd][index]

		case Nothing, Untracked:
			// Nothing: Very likely external struct.
			// Untracked: likely branches of return values (e.g. returning nil)
			fmt.Fprintf(os.Stderr, "   %s = &%s(=%s)[%d] (external) of type %s\n", cyan(reg(elem)), inst.X.Name(), vd.String(), index, elem.Type().String())
			vd := utils.NewDef(array) // New external array
			fr.locals[array] = vd
			fr.env.arrays[vd] = make(Elems)
			vdElem := utils.NewDef(elem) // New external elem
			fr.env.arrays[vd][index] = vdElem
			fr.locals[elem] = vdElem
			fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as elem definition of type %s\n", elem.Name(), inst.Type().(*types.Pointer).Elem().Underlying().String())

		default:
			panic(fmt.Sprintf("IndexAddr: Cannot access non-array %s", reg(array)))
		}
	} else {
		panic(fmt.Sprintf("IndexAddr: Cannot access field - %s not an array", reg(array)))
	}
}

func visitIndex(inst *ssa.Index, fr *frame) {
	elem := inst
	array := inst.X
	index := inst.Index
	_, isArray := array.Type().Underlying().(*types.Array)
	_, isSlice := array.Type().Underlying().(*types.Slice)

	if isArray || isSlice {
		switch vd, kind := fr.get(array); kind {
		case Array:
			fmt.Fprintf(os.Stderr, "   %s = %s(=%s)[%d] of type %s\n", cyan(reg(elem)), array.Name(), vd.String(), index, elem.Type().String())
			if fr.env.arrays[vd][index] == nil { // First use
				vdelem := utils.NewDef(elem)
				fr.env.arrays[vd][index] = vdelem
				fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as elem definition\n", elem.Name())
			} else if fr.env.arrays[vd][index].Var != elem { // Previously defined
				fmt.Fprintf(os.Stderr, "     ^ elem %s previously defined as %s\n", elem.Name(), reg(fr.env.arrays[vd][index].Var))
			} // else Accessed before (and unchanged)
			fr.locals[elem] = fr.env.arrays[vd][index]

		case LocalArray:
			fmt.Fprintf(os.Stderr, "   %s = %s(=%s)[%d] (local) of type %s\n", cyan(reg(elem)), array.Name(), vd.String(), index, elem.Type().String())
			if fr.arrays[vd][index] == nil { // First use
				vdElem := utils.NewDef(elem)
				fr.arrays[vd][index] = vdElem
				fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as elem definition\n", elem.Name())
			} else if fr.arrays[vd][index].Var != elem { // Previously defined
				fmt.Fprintf(os.Stderr, "     ^ elem %s previously defined as %s\n", elem.Name(), reg(fr.arrays[vd][index].Var))
			} // else Accessed before (and unchanged)
			fr.locals[elem] = fr.arrays[vd][index]

		case Nothing, Untracked:
			// Nothing: Very likely external struct.
			// Untracked: likely branches of return values (e.g. returning nil)
			fmt.Fprintf(os.Stderr, "   %s = %s(=%s)[%d] (external) of type %s\n", cyan(reg(elem)), inst.X.Name(), vd.String(), index, elem.Type().String())
			vd := utils.NewDef(array) // New external array
			fr.locals[array] = vd
			fr.env.arrays[vd] = make(Elems)
			vdElem := utils.NewDef(elem) // New external elem
			fr.env.arrays[vd][index] = vdElem
			fr.locals[elem] = vdElem
			fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as elem definition of type %s\n", elem.Name(), inst.Type().(*types.Pointer).Elem().Underlying().String())

		default:
			panic(fmt.Sprintf("Index: Cannot access non-array %s", reg(array)))
		}
	} else {
		panic(fmt.Sprintf("Index: Cannot access element - %s not an array", reg(array)))
	}
}

func visitDefer(inst *ssa.Defer, fr *frame) {
	fr.defers = append(fr.defers, inst)
}

func visitRunDefers(inst *ssa.RunDefers, fr *frame) {
	for i := len(fr.defers) - 1; i >= 0; i-- {
		fr.callCommon(fr.defers[i].Value(), fr.defers[i].Common())
	}
}

func visitPhi(inst *ssa.Phi, fr *frame) {
	// In the case of channels, find the last defined channel and replace it.
	if _, ok := inst.Type().(*types.Chan); ok {
		//preds := inst.Block().Preds // PredBlocks: order is significant.
		fr.locals[inst], _ = fr.get(inst.Edges[0])
		fr.phi[inst] = inst.Edges
	}
}

func visitTypeAssert(inst *ssa.TypeAssert, fr *frame) {
	if iface, ok := inst.AssertedType.(*types.Interface); ok {
		if meth, _ := types.MissingMethod(inst.X.Type(), iface, true); meth == nil { // No missing methods
			switch vd, kind := fr.get(inst.X); kind {
			case Struct, LocalStruct, Array, LocalArray, Chan:
				fr.tuples[inst] = make(Tuples, 2)
				fr.tuples[inst][0] = vd
				fmt.Fprintf(os.Stderr, "   %s = %s.(type assert %s) iface\n", reg(inst), reg(inst.X), inst.AssertedType.String())
				fmt.Fprintf(os.Stderr, "    ^ defined as %s\n", vd.String())

			default:
				fmt.Fprintf(os.Stderr, "   %s = %s.(type assert %s)\n", red(reg(inst)), reg(inst.X), inst.AssertedType.String())
				fmt.Fprintf(os.Stderr, "    ^ untracked/unknown\n")
			}
			return
		}
	} else { // Concrete type
		if types.Identical(inst.AssertedType.Underlying(), inst.X.Type().Underlying()) {
			switch vd, kind := fr.get(inst.X); kind {
			case Struct, LocalStruct, Array, LocalArray, Chan:
				fr.tuples[inst] = make(Tuples, 2)
				fr.tuples[inst][0] = vd
				fmt.Fprintf(os.Stderr, "   %s = %s.(type assert %s) concrete\n", reg(inst), reg(inst.X), inst.AssertedType.String())
				fmt.Fprintf(os.Stderr, "    ^ defined as %s\n", vd.String())

			default:
				fmt.Fprintf(os.Stderr, "   %s = %s.(type assert %s)\n", red(reg(inst)), reg(inst.X), inst.AssertedType.String())
				fmt.Fprintf(os.Stderr, "    ^ untracked/unknown\n")
			}
			return
		}
	}
	fmt.Fprintf(os.Stderr, "   # %s = %s.(%s) impossible type assertion\n", red(reg(inst)), reg(inst.X), inst.AssertedType.String())
}
