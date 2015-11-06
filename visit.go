package main

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

	if len(blk.Preds) > 1 {
		blkLabel := fmt.Sprintf("%s#%d", blk.Parent().String(), blk.Index)

		if _, found := fr.gortn.visited[blk]; found {
			fr.gortn.AddNode(sesstype.MkGotoNode(blkLabel))
			return
		}

		// Make a label for other edges that enter this block
		lblNode := sesstype.MkLabelNode(blkLabel)
		fr.gortn.AddNode(lblNode)
		fr.gortn.visited[blk] = lblNode // XXX visited is initialised by append if lblNode is head of tree
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

	if fn != nil && callee.caller != nil && fn.String() == callee.caller.fn.String() {
		fmt.Fprintf(os.Stderr, "  !! Recursive function %s\n", fn.Name())
		return false
	}

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
			fmt.Fprintf(os.Stderr, "   # unhandled %s = "+red("%s")+"\n", inst.Name(), inst.String())
		}

	case *ssa.Call:
		visitCall(inst, fr)

	case *ssa.Extract:
		visitExtract(inst, fr)

	case *ssa.Go:
		callgo(inst, fr)

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

	default:
		// Everything else not handled yet
		if v, ok := inst.(ssa.Value); ok {
			fmt.Fprintf(os.Stderr, "   # unhandled %s = "+red("%s")+"\n", v.Name(), v.String())
		} else {
			fmt.Fprintf(os.Stderr, "   # unhandled "+red("%s")+"\n", inst.String())
		}
	}

	return cont
}

func visitExtract(e *ssa.Extract, fr *frame) {
	if tpl, ok := fr.env.tuples[e.Tuple]; ok {
		fmt.Fprintf(os.Stderr, "  (Extract Tuple %s #%d == %s)\n", e.Tuple.Name(), e.Index, tpl[e.Index].String())
	} else {
		// Check if we are extracting select index
		if _, ok := fr.env.selNode[e.Tuple]; ok && e.Index == 0 {
			fmt.Fprintf(os.Stderr, "  (Select %s index = %s)\n", e.Tuple.Name(), e.Name())
			fr.env.selIdx[e] = e.Tuple
			return
		}
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
			if e.Index < len(tpl) {
				fmt.Fprintf(os.Stderr, "  (Extract Tuple %s #%d == %s)\n", e.Tuple.Name(), e.Index, tpl[e.Index].String())
			} else {
				fmt.Fprintf(os.Stderr, "  (Extract Tuple %s #%d from length %d)\n", e.Tuple.Name(), e.Index, len(tpl))
			}
		} else {
			fmt.Fprintf(os.Stderr, "  # "+red("%s")+" of type %s\n", e.String(), e.Type().String())
		}
	}
}

func visitMakeClosure(inst *ssa.MakeClosure, frm *frame) {
	frm.env.closures[inst] = inst.Bindings
}

// visitAlloc is for variable allocation (usually by 'new')
// Everything allocated here are pointers
func visitAlloc(inst *ssa.Alloc, fr *frame) {
	t := inst.Type().Underlying().(*types.Pointer).Elem()
	if _, ok := t.(*types.Chan); ok {
		ch := fr.env.session.MakeChan(inst.Name(), fr.gortn.role, t)
		fr.env.chans[inst] = ch // Ptr to channel
	} else if st, ok := t.Underlying().(*types.Struct); ok {
		fr.env.structs[inst] = make([]ssa.Value, st.NumFields())
		fmt.Fprintf(os.Stderr, "   Alloc (struct) %s of type %s (%d fields)%s\n", inst.Name(), inst.Type().String(), st.NumFields(), loc(fr.fn.Prog.Fset, inst.Pos()))
	} else if ar, ok := t.Underlying().(*types.Array); ok {
		fr.env.arrays[inst] = make(map[ssa.Value]ssa.Value)
		fmt.Fprintf(os.Stderr, "   Alloc (array) %s of type %s (%d elems)%s\n", inst.Name(), inst.Type().String(), ar.Len(), loc(fr.fn.Prog.Fset, inst.Pos()))
	} else {
		fmt.Fprintf(os.Stderr, "   # %s = "+red("Alloc %s")+" of type %s\n", inst.Name(), inst.String(), t.String())
	}
}

func visitValueof(inst *ssa.UnOp, fr *frame) {
	ptr := inst.X
	val := inst
	if ch, found := fr.env.chans[fr.get(ptr)]; found {
		fr.env.chans[val] = ch
		fmt.Fprintf(os.Stderr, " --> It's a channel\n")
	}
	fr.locals[val] = ptr
	fmt.Fprintf(os.Stderr, "   %s = &%s\n", val.Name(), ptr.Name())
}

func visitSelect(s *ssa.Select, fr *frame) {
	if fr.gortn.leaf == nil {
		fr.gortn.AddNode(sesstype.MkLabelNode("__head__"))
	}
	fr.env.selNode[s] = fr.gortn.leaf
	for _, state := range s.States {
		if ch, ok := fr.env.chans[fr.get(state.Chan)]; ok {
			switch state.Dir {
			case types.SendOnly:
				fr.gortn.leaf = fr.env.selNode[s]
				fr.gortn.AddNode(sesstype.MkSelectSendNode(fr.gortn.role, ch))
			case types.RecvOnly:
				fr.gortn.leaf = fr.env.selNode[s]
				fr.gortn.AddNode(sesstype.MkSelectRecvNode(ch, fr.gortn.role))
			default:
				panic("Cannot handle 'select' with SendRecv channels")
			}
			fmt.Fprintf(os.Stderr, "   select "+orange("%s")+"\n", fr.gortn.leaf.String())
		} else {
			panic("Channel " + fr.get(state.Chan).Name() + fr.get(state.Chan).Parent().String() + " " + loc(fr.fn.Prog.Fset, state.Chan.Pos()) + " not found!\n")
		}
	}
}

func visitReturn(ret *ssa.Return, fr *frame) []ssa.Value {
	fmt.Fprintf(os.Stderr, " -- Return from %s\n", fr.fn.String())
	//fr.gortn.append(sesstype.MkEndNode())
	return ret.Results
}

// Handles function call.
// Wrapper for calling visitFunc and performing argument translation.
func visitCall(c *ssa.Call, caller *frame) {
	if !caller.env.calls[c] {
		call(c, caller)
		caller.env.calls[c] = true
	}
}

func visitIf(inst *ssa.If, fr *frame) {
	if len(inst.Block().Succs) != 2 {
		panic("Cannot handle If with more or less than 2 successor blocks!")
	}

	// Check if this is a select-test-jump, if so handle separately.
	if selTest, isSelTest := fr.env.selTest[inst.Cond]; isSelTest {
		fmt.Fprintf(os.Stderr, "  @ Switch to select branch #%d\n", selTest.idx)
		if selParent, ok := fr.env.selNode[selTest.tpl]; ok {
			ifparent := fr.gortn.leaf
			fmt.Fprintf(os.Stderr, "parent %s\n", selParent)
			fr.gortn.leaf = selParent.Child(selTest.idx)
			visitBlock(inst.Block().Succs[0], fr)

			fr.gortn.leaf = ifparent
			visitBlock(inst.Block().Succs[1], fr)
		} else {
			panic("Select without corresponding sesstype.Node")
		}
	} else {
		ifparent := fr.gortn.leaf
		visitBlock(inst.Block().Succs[0], fr)
		if fr.gortn.leaf == ifparent {
			fr.gortn.AddNode(&sesstype.EmptyBodyNode{})
		}

		fr.gortn.leaf = ifparent
		visitBlock(inst.Block().Succs[1], fr)
		if fr.gortn.leaf == ifparent {
			fr.gortn.AddNode(&sesstype.EmptyBodyNode{})
		}
	}

	// This is end of the block so no continuation
}

func visitMakeChan(mc *ssa.MakeChan, fr *frame) {
	ch := fr.env.session.MakeChan(mc.Name(), fr.gortn.role, mc.Type())
	fr.env.chans[mc] = ch // Ptr to channel
	fr.gortn.AddNode(sesstype.MkNewChanNode(ch))
	fmt.Fprintf(os.Stderr, "   MakeChan %s of type %s for %s %v\n", mc.Name(), mc.Type().String(), fr.gortn.role.Name(), fr.gortn.leaf)
}

func visitSend(send *ssa.Send, fr *frame) {
	if ch, ok := fr.env.chans[fr.get(send.Chan)]; ok {
		fr.gortn.AddNode(sesstype.MkSendNode(fr.gortn.role, ch))
		fmt.Fprintf(os.Stderr, "  "+orange("%s")+"\n", fr.gortn.leaf.String())
	} else {
		fmt.Fprintf(os.Stderr, "Send%s: '%s' is not a channel", loc(fr.fn.Prog.Fset, send.Pos()), send.Chan.Name())
	}
}

func visitRecv(recv *ssa.UnOp, fr *frame) {
	if ch, ok := fr.env.chans[fr.get(recv.X)]; ok {
		fr.gortn.AddNode(sesstype.MkRecvNode(ch, fr.gortn.role))
		fmt.Fprintf(os.Stderr, "  "+orange("%s")+"\n", fr.gortn.leaf.String())
	} else {
		fmt.Fprintf(os.Stderr, "Recv%s: '%s' is not a channel", loc(fr.fn.Prog.Fset, recv.Pos()), recv.X.Name())
	}
}

// visitClose for the close() builtin primitive.
func visitClose(ch sesstype.Chan, fr *frame) {
	fmt.Fprintf(os.Stderr, " -- Enter close()\n")

	fr.gortn.AddNode(sesstype.MkSendNode(fr.gortn.role, ch))
	fr.gortn.AddNode(sesstype.MkEndNode(ch))
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

	if _, ok := dstPtr.(*ssa.Global); ok {
		fr.env.globals[dstPtr] = source
		fmt.Fprintf(os.Stderr, "   # store (global) *%s = %s of type %s\n", dstPtr.Name(), source.Name(), source.Type().String())
	} else {
		fr.locals[dstPtr] = source
		fmt.Fprintf(os.Stderr, "   # store *%s = %s of type %s\n", dstPtr.Name(), source.Name(), source.Type().String())
	}

	//if ch, found := fr.env.chans[fr.get(source)]; found {
	//	fr.env.chans[dstPtr] = ch
	//	fmt.Fprintf(os.Stderr, "   & store (chan) *%s -> ch{ %s }\n", dstPtr.Name(), ch.Name())
	//}

	// Check if this is a field or array element.
	if fInfo, ok := fr.fields[dstPtr]; ok {
		fr.env.structs[fInfo.str][fInfo.idx] = source // Update existing field to source
		fmt.Fprintf(os.Stderr, "   & store (struct) %s as %s:%s.[%d] of type %s\n", source.Name(), fInfo.str.Name(), fr.get(fInfo.str).Name(), fInfo.idx, source.Type().String())
	} else if aInfo, ok := fr.elems[dstPtr]; ok {
		fr.env.arrays[fr.get(aInfo.arr)][aInfo.idx] = source
		fmt.Fprintf(os.Stderr, "   & store (array) %s as %s:%s[%d] of type %s\n", source.Name(), aInfo.arr.Name(), fr.get(aInfo.arr).Name(), aInfo.idx, source.Type().String())
	}
}

func visitChangeType(inst *ssa.ChangeType, fr *frame) {
	if _, ok := inst.Type().(*types.Chan); ok {
		if ch, found := fr.env.chans[fr.get(inst.X)]; found {
			fr.env.chans[inst] = ch
			fmt.Fprintf(os.Stderr, "   & changetype from %s to %s (channel %s)\n", inst.X.Name(), inst.Name(), ch.Name())
		} else {
			panic("Channel " + inst.X.Name() + loc(fr.fn.Prog.Fset, inst.Pos()) + " not found!\n")
		}
	} else {
		fr.locals[inst] = inst.X
		fmt.Fprintf(os.Stderr, "   # %s = "+red("%s")+"\n", inst.Name(), inst.String())
	}
}

func visitBinOp(inst *ssa.BinOp, fr *frame) {
	switch inst.Op {
	case token.EQL:
		if selTuple, isSelTuple := fr.env.selIdx[inst.X]; isSelTuple {
			branchId := int(inst.Y.(*ssa.Const).Int64())
			fr.env.selTest[inst] = struct {
				idx int
				tpl ssa.Value
			}{
				branchId, selTuple,
			}
		} else {
			fmt.Fprintf(os.Stderr, "   # %s = "+red("%s")+"\n", inst.Name(), inst.String())
		}
	default:
		fmt.Fprintf(os.Stderr, "   # %s = "+red("%s")+"\n", inst.Name(), inst.String())
	}
}

func visitFieldAddr(inst *ssa.FieldAddr, fr *frame) {
	field := inst
	struc := fr.get(inst.X)
	index := inst.Field

	if struc == nil {
		panic("struct cannot be nil!")
	}

	// If struct has been allocated
	if _str, ok := fr.env.structs[struc]; ok {
		// If struct exists but struc->#index is empty, point to field
		if f := _str[index]; f == nil {
			_str[index] = field
			fmt.Fprintf(os.Stderr, "   %s->[%d] uninitialised\n", struc.Name(), index)
		} else if _str[index] != field {
			fr.locals[field] = _str[index]
		}
		fr.fields[field] = struct {
			idx int
			str ssa.Value
		}{
			index,
			struc,
		}
		fmt.Fprintf(os.Stderr, "   %s = %s(=%s)->[%d] of type %s\n", field.Name(), inst.X.Name(), struc.Name(), index, field.Type().String())
	} else if strucType, ok := deref(deref(struc.Type())).Underlying().(*types.Struct); ok { // struct has not been allocated but is a struct (e.g. uninitialised)
		// XXX Double dereferencing, luckily Underlying seems to be idempotent.
		fr.env.structs[struc] = make([]ssa.Value, strucType.NumFields())
		fr.fields[field] = struct {
			idx int
			str ssa.Value
		}{
			index,
			struc,
		}
		fmt.Fprintf(os.Stderr, "   %s = (new empty) %s(=%s)->[%d] of type %s\n", field.Name(), inst.X.Name(), struc.Name(), index, field.Type().String())
	} else {
		fmt.Fprintf(os.Stderr, "   "+red("%s = %s->[%d] of type %s")+"\n", field.Name(), struc.Name(), index, field.Type().String())
		panic("Wrong type and struct not found")
	}
}

func visitField(inst *ssa.Field, fr *frame) {
	field := inst
	struc := fr.get(inst.X)
	index := inst.Field

	if struc == nil {
		panic("struct cannot be nil!")
	}

	// If struct has been allocated
	if _str, ok := fr.env.structs[struc]; ok {
		// If struct exists but struc->#index is empty, point to field
		if f := _str[index]; f == nil {
			_str[index] = field
			fmt.Fprintf(os.Stderr, "   %s.[%d] uninitialised\n", struc.Name(), index)
		} else if _str[index] != field {
			fr.locals[field] = _str[index]
		}
		fr.fields[field] = struct {
			idx int
			str ssa.Value
		}{
			index,
			struc,
		}
		fmt.Fprintf(os.Stderr, "   %s = %s(=%s).[%d] of type %s\n", field.Name(), inst.X.Name(), struc.Name(), index, field.Type().String())
	} else if strucType, ok := struc.Type().(*types.Struct); ok { // struct has not been allocated but is a struct (e.g. uninitialised)
		fr.env.structs[struc] = make([]ssa.Value, strucType.NumFields())
		fr.fields[field] = struct {
			idx int
			str ssa.Value
		}{
			index,
			struc,
		}
		fmt.Fprintf(os.Stderr, "   %s = (new empty) %s:%s.[%d] of type %s\n", field.Name(), inst.X.Name(), struc.Name(), index, field.Type().String())
	} else {
		fmt.Fprintf(os.Stderr, "   "+red("%s = %s.[%d] of type %s")+"\n", field.Name(), struc.Name(), index, field.Type().String())
		panic("Wrong type and struct not found")
	}
}

func visitSlice(inst *ssa.Slice, fr *frame) {
	fr.env.arrays[inst] = make(map[ssa.Value]ssa.Value)
}

func visitMakeSlice(inst *ssa.MakeSlice, fr *frame) {
	fr.env.arrays[inst] = make(map[ssa.Value]ssa.Value)
}

func visitIndexAddr(inst *ssa.IndexAddr, fr *frame) {
	elem := inst
	array := fr.get(inst.X)
	index := fr.get(inst.Index)

	if array == nil {
		panic("array cannot be nil!")
	}

	if index == nil {
		panic("index cannot be nil!")
	}

	if _arr, ok := fr.env.arrays[array]; ok {
		// if array exists but array[#index] is empty, point to elem
		if f := _arr[index]; f == nil {
			_arr[index] = elem
			fmt.Fprintf(os.Stderr, "   &%s[%s] uninitialised\n", array.Name(), index.Name())
		} else if _arr[index] != elem {
			fr.locals[elem] = _arr[index]
		}
		fr.elems[elem] = struct {
			idx ssa.Value
			arr ssa.Value
		}{
			index,
			array,
		}
		fmt.Fprintf(os.Stderr, "   %s = &%s(=%s)[%s] of type %s\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
	} else if _, ok := deref(deref(array.Type())).Underlying().(*types.Array); ok {
		fr.env.arrays[array] = make(map[ssa.Value]ssa.Value)
		fr.elems[elem] = struct {
			idx ssa.Value
			arr ssa.Value
		}{
			index,
			array,
		}
		fmt.Fprintf(os.Stderr, "   %s = (new empty) &%s(=%s)[%s] of type %s\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
	} else if _, ok := deref(deref(array.Type())).Underlying().(*types.Slice); ok {
		fr.env.arrays[array] = make(map[ssa.Value]ssa.Value)
		fr.elems[elem] = struct {
			idx ssa.Value
			arr ssa.Value
		}{
			index,
			array,
		}
		fmt.Fprintf(os.Stderr, "   %s = (new empty) &%s(=%s)[%s] of type %s\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
	} else {
		fmt.Fprintf(os.Stderr, "   "+red("%s = &%s(=%s)[%s] of type %s")+"\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
		panic("Wrong type and array not found")
	}
}

func visitIndex(inst *ssa.Index, fr *frame) {
	elem := inst
	array := fr.get(inst.X)
	index := fr.get(inst.Index)

	if array == nil {
		panic("array cannot be nil!")
	}

	if index == nil {
		panic("index cannot be nil!")
	}

	if _arr, ok := fr.env.arrays[array]; ok {
		// if array exists but array[#index] is empty, point to elem
		if f := _arr[index]; f == nil {
			_arr[index] = elem
			fmt.Fprintf(os.Stderr, "   %s[%s] uninitialised\n", array.Name(), index.Name())
		} else if _arr[index] != elem {
			fr.locals[elem] = _arr[index]
		}
		fr.elems[elem] = struct {
			idx ssa.Value
			arr ssa.Value
		}{
			index,
			array,
		}
		fmt.Fprintf(os.Stderr, "   %s = %s(=%s)[%s] of type %s\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
	} else if _, ok := deref(array.Type()).(*types.Array); ok {
		fr.env.arrays[array] = make(map[ssa.Value]ssa.Value)
		fr.elems[elem] = struct {
			idx ssa.Value
			arr ssa.Value
		}{
			index,
			array,
		}
		fmt.Fprintf(os.Stderr, "   %s = (new empty) %s(=%s)[%s] of type %s\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
	} else {
		fmt.Fprintf(os.Stderr, "   "+red("%s = %s(=%s)[%s] of type %s")+"\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
		panic("Wrong type and array not found")
	}
}
