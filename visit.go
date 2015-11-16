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
	//fmt.Fprintf(os.Stderr, "-- Block %d (in:%d, out:%d)\n", blk.Index, len(blk.Preds), len(blk.Succs))

	if len(blk.Preds) > 1 {
		blkLabel := fmt.Sprintf("%s#%d", blk.Parent().String(), blk.Index)

		if _, found := fr.gortn.visited[blk]; found {
			fr.gortn.AddNode(sesstype.MkGotoNode(blkLabel))
			return
		} else {
			// Make a label for other edges that enter this block
			label := sesstype.MkLabelNode(blkLabel)
			fr.gortn.AddNode(label)
			fr.gortn.visited[blk] = label // XXX visited is initialised by append if lblNode is head of tree
		}
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
	//fmt.Fprintf(os.Stderr, " -- Enter Function %s() %s\n", fn.String(), loc(fn.Prog.Fset, fn.Pos()))

	if fn.Blocks == nil {
		//fmt.Fprintf(os.Stderr, "  # Ignore builtin/external '"+fn.String()+"' with no Blocks\n")
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
			visitDeref(inst, fr)
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

	case *ssa.Defer:
		visitDefer(inst, fr)

	case *ssa.RunDefers:
		visitRunDefers(inst, fr)

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
				if _, ok := extTpl.At(e.Index).Type().(*types.Chan); ok {
					//fr.env.chans[e] = fr.env.session.MakeChan(e.Tuple, fr.gortn.role)
					panic("undefined channel")
				}
			}
			if e.Index < len(tpl) {
				fmt.Fprintf(os.Stderr, "  (Extract Tuple %s #%d == %s)\n", e.Tuple.Name(), e.Index, tpl[e.Index].String())
			} else {
				fmt.Fprintf(os.Stderr, "  (Extract Tuple %s #%d from length %d)\n", e.Tuple.Name(), e.Index, len(tpl))
			}
		} else {
			fmt.Fprintf(os.Stderr, "   # "+red("%s")+" of type %s\n", e.String(), e.Type().String())
		}
	}
}

func visitMakeClosure(inst *ssa.MakeClosure, frm *frame) {
	frm.env.closures[inst] = inst.Bindings
}

// visitAlloc is for variable allocation (usually by 'new')
// Everything allocated here are pointers
func visitAlloc(inst *ssa.Alloc, fr *frame) {
	var mem string
	locn := loc(fr.fn.Prog.Fset, inst.Pos())
	if derefT, ok := inst.Type().(*types.Pointer); ok {
		switch t := derefT.Elem().Underlying().(type) {
		case *types.Array:
			if inst.Heap {
				fr.env.arrays[inst] = make(ArrayElems)
				mem = "heap"
			} else {
				fr.arrays[inst] = make(ArrayElems)
				mem = "stack"
			}
			fmt.Fprintf(os.Stderr, "   %s = Alloc (array@%s) of type %s (%d elems) at %s\n", cyan(reg(inst)), mem, inst.Type().String(), t.Len(), locn)
			return

		case *types.Struct:
			if inst.Heap {
				fr.env.structs[inst] = make(StructFields, t.NumFields())
				mem = "heap"
			} else {
				fr.structs[inst] = make(StructFields, t.NumFields())
				mem = "stack"
			}
			fmt.Fprintf(os.Stderr, "   %s = Alloc (struct@%s) of type %s (%d fields) at %s\n", cyan(reg(inst)), mem, inst.Type().String(), t.NumFields(), locn)
			return

		case *types.Chan:
			fmt.Fprintf(os.Stderr, "   %s = Alloc (chan) of type %s at %s\n", cyan(reg(inst)), inst.Type().String(), locn)
			return

		default:
			fmt.Fprintf(os.Stderr, "   # %s = "+red("Alloc %s")+" of type %s\n", inst.Name(), inst.String(), t.String())
			return
		}
	}
	fmt.Fprintf(os.Stderr, "   # %s = "+red("Alloc %s")+" of non-pointer type %s\n", inst.Name(), inst.String(), inst.Type().String())
}

func visitDeref(inst *ssa.UnOp, fr *frame) {
	ptr := inst.X
	val := inst

	fr.locals[val] = ptr
	fmt.Fprintf(os.Stderr, "   %s = *%s\n", cyan(reg(val)), ptr.Name())

	if c, r := fr.findChan(fr.get(val)); c != nil {
		fmt.Fprintf(os.Stderr, "    ^ channel %s in %s\n", green(c.Name()), r.Name())
	}
}

func visitSelect(s *ssa.Select, fr *frame) {
	if fr.gortn.leaf == nil {
		fr.gortn.AddNode(sesstype.MkLabelNode("__head__"))
	}
	fr.env.selNode[s] = fr.gortn.leaf
	for _, state := range s.States {
		if c, ok := fr.gortn.chans[fr.get(state.Chan)]; ok {
			switch state.Dir {
			case types.SendOnly:
				fr.gortn.leaf = fr.env.selNode[s]
				fr.gortn.AddNode(sesstype.MkSelectSendNode(fr.gortn.role, c))
			case types.RecvOnly:
				fr.gortn.leaf = fr.env.selNode[s]
				fr.gortn.AddNode(sesstype.MkSelectRecvNode(c, fr.gortn.role))
			default:
				panic("Cannot handle 'select' with SendRecv channels")
			}
			fmt.Fprintf(os.Stderr, "   select "+orange("%s")+"\n", (*fr.gortn.leaf).String())
		} else {
			fr.printCallStack()
			panic(fmt.Sprintf("select: channel %s=%s at %s not found!\n", reg(state.Chan), reg(fr.get(state.Chan)), loc(fr.fn.Prog.Fset, state.Chan.Pos())))
		}
	}
}

func visitReturn(ret *ssa.Return, fr *frame) []ssa.Value {
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

	ifparent := fr.gortn.leaf
	if ifparent == nil {
		panic("parent is nil")
	}

	// Check if this is a select-test-jump, if so handle separately.
	if selTest, isSelTest := fr.env.selTest[inst.Cond]; isSelTest {
		fmt.Fprintf(os.Stderr, "  @ Switch to select branch #%d\n", selTest.idx)
		if selParent, ok := fr.env.selNode[selTest.tpl]; ok {
			fr.gortn.leaf = ifparent
			fmt.Fprintf(os.Stderr, "parent %s\n", *selParent)
			*fr.gortn.leaf = (*selParent).Child(selTest.idx)
			visitBlock(inst.Block().Succs[0], fr)

			*fr.gortn.leaf = (*selParent).Child(selTest.idx)
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

func visitMakeChan(mc *ssa.MakeChan, fr *frame) {
	locn := loc(fr.fn.Prog.Fset, mc.Pos())
	role := fr.gortn.role

	ch := fr.env.session.MakeChan(mc, role)
	if _ch, ok := fr.gortn.chans[mc]; ok {
		fmt.Fprintf(os.Stderr, "Channel %s exists, is this a recursive function? See %s() at %s\n", _ch.Name(), mc.Parent().String(), locn)
		fr.printCallStack()
		//panic(fmt.Sprintf("Cannot make new channel: channel %s exists", reg(mc)))
	}
	fr.gortn.chans[mc] = ch
	fr.gortn.AddNode(sesstype.MkNewChanNode(ch))
	fmt.Fprintf(os.Stderr, "   New channel "+green("%s { type: %s }")+" by %s() at %s\n", ch.Name(), ch.Type().String(), mc.Parent().String(), locn)
	fmt.Fprintf(os.Stderr, "                 ^ in role %s\n", role.Name())
}

func visitSend(send *ssa.Send, fr *frame) {
	if c, _ := fr.findChan(fr.get(send.Chan)); c != nil {
		fr.gortn.AddNode(sesstype.MkSendNode(fr.gortn.role, c))
		fmt.Fprintf(os.Stderr, "  "+orange("%s")+"\n", (*fr.gortn.leaf).String())
	} else {
		fmt.Fprintf(os.Stderr, "Send%s: '%s' is not a channel", loc(fr.fn.Prog.Fset, send.Pos()), send.Chan.Name())
	}
}

func visitRecv(recv *ssa.UnOp, fr *frame) {
	if c, _ := fr.findChan(fr.get(recv.X)); c != nil {
		fr.gortn.AddNode(sesstype.MkRecvNode(c, fr.gortn.role))
		fmt.Fprintf(os.Stderr, "  "+orange("%s")+"\n", (*fr.gortn.leaf).String())
	} else {
		fmt.Fprintf(os.Stderr, "Recv%s: '%s' is not a channel", loc(fr.fn.Prog.Fset, recv.Pos()), recv.X.Name())
	}
}

// visitClose for the close() builtin primitive.
func visitClose(ch sesstype.Chan, fr *frame) {
	fr.gortn.AddNode(sesstype.MkEndNode(ch))
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
	dstPtr := inst.Addr

	if _, ok := dstPtr.(*ssa.Global); ok {
		fr.env.globals[dstPtr] = source
		fmt.Fprintf(os.Stderr, "   # store (global) *%s = %s of type %s\n", dstPtr.String(), source.Name(), source.Type().String())
	} else {
		fr.locals[dstPtr] = source
		fmt.Fprintf(os.Stderr, "   # store *%s = %s of type %s\n", reg(dstPtr), reg(fr.get(source)), source.Type().String())
	}

	if fieldInfo, ok := fr.fields[dstPtr]; ok { // if dstPtr is a struct field
		if str, heap, ok := fr.getStruct(fieldInfo.str); ok {
			str[fieldInfo.idx] = source // OVERWRITE existing value
			if heap {
				fmt.Fprintf(os.Stderr, "            ^ stored (struct@heap) %s as %s.[%d] of type %s\n", dstPtr.Name(), fieldInfo.str.Name(), fieldInfo.idx, source.Type().String())
			} else {
				fmt.Fprintf(os.Stderr, "            ^ stored (struct@stack) %s as %s.[%d] of type %s\n", dstPtr.Name(), fieldInfo.str.Name(), fieldInfo.idx, source.Type().String())
			}
		} else {
			if t, ok := deref(fieldInfo.str.Type()).Underlying().(*types.Struct); ok {
				fr.env.structs[fieldInfo.str] = make(StructFields, t.NumFields())
				fr.env.structs[fieldInfo.str][fieldInfo.idx] = source
				fmt.Fprintf(os.Stderr, "            ^ stored (new struct@heap) %s as %s.[%d] of type %s\n", dstPtr.Name(), fieldInfo.str.Name(), fieldInfo.idx, source.Type().String())
			}
		}
	}

	if elemInfo, ok := fr.elems[dstPtr]; ok { // if dstPtr is an array element
		if arr, heap, ok := fr.getArray(elemInfo.arr); ok {
			arr[elemInfo.idx] = source // OVERWRITE existing value
			if heap {
				fmt.Fprintf(os.Stderr, "            ^ stored (array@heap) %s as %s[%s] of type %s\n", red(dstPtr.Name()), elemInfo.arr.Name(), elemInfo.idx, source.Type().String())
			} else {
				fmt.Fprintf(os.Stderr, "            ^ stored (array@struct) %s as %s[%s] of type %s\n", red(dstPtr.Name()), elemInfo.arr.Name(), elemInfo.idx, source.Type().String())
			}
		} else {
			if _, ok := deref(elemInfo.arr.Type()).Underlying().(*types.Array); ok {
				fr.env.arrays[elemInfo.arr] = make(ArrayElems)
				fr.env.arrays[elemInfo.arr][elemInfo.idx] = source
				fmt.Fprintf(os.Stderr, "            ^ stored (new array@heap) %s as %s[%s] of type %s\n", dstPtr.Name(), elemInfo.arr.Name(), elemInfo.idx, source.Type().String())
			}
		}
	}
}

func visitChangeType(inst *ssa.ChangeType, fr *frame) {
	if _, ok := inst.Type().(*types.Chan); ok {
		if ch, found := fr.gortn.chans[inst.X]; found {
			fr.locals[inst] = inst.X
			fmt.Fprintf(os.Stderr, "   & changetype from %s to %s (channel %s)\n", green(reg(inst.X)), reg(inst), ch.Name())
		} else {
			panic("Channel " + reg(inst) + loc(fr.fn.Prog.Fset, inst.Pos()) + " not found!\n")
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
			//fmt.Fprintf(os.Stderr, "   # %s = "+red("%s")+"\n", inst.Name(), inst.String())
		}
	default:
		//fmt.Fprintf(os.Stderr, "   # %s = "+red("%s")+"\n", inst.Name(), inst.String())
	}
}

func visitSlice(inst *ssa.Slice, fr *frame) {
	fr.env.arrays[inst] = make(map[ssa.Value]ssa.Value)
}

func visitMakeSlice(inst *ssa.MakeSlice, fr *frame) {
	fr.env.arrays[inst] = make(map[ssa.Value]ssa.Value)
}

func visitFieldAddr(inst *ssa.FieldAddr, fr *frame) {
	field := inst
	struc := fr.get(inst.X)
	index := inst.Field

	if struc == nil {
		panic("struct cannot be nil!")
	}

	// If struct has been allocated
	if str, _, ok := fr.getStruct(struc); ok {
		// This is the new field access.
		fd := FieldDecomp{
			str: struc,
			idx: index,
		}
		fmt.Fprintf(os.Stderr, "   %s = %s(=%s)->[%d] of type %s\n", cyan(reg(field)), inst.X.Name(), reg(struc), index, field.Type().String())
		if inst.X == struc {
			fmt.Fprintf(os.Stderr, "     ^ %s is defined in this scope: %s\n", struc.Name(), struc.String())
		}
		if str[index] == nil { // Struct exists but field does not
			str[index] = field
			fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as field definition\n", field.Name())
		} else if str[index] != field { // Field defined elsewhere
			fr.locals[field] = str[index]
		}
		fr.fields[field] = fd
	} else if _, ok := deref(struc.Type()).Underlying().(*types.Struct); ok { // struct has not been allocated but is a struct (e.g. uninitialised)
		fr.env.structs[struc] = make(StructFields) // Create external struct
		fr.fields[field] = FieldDecomp{
			str: struc,
			idx: index,
		}
		fmt.Fprintf(os.Stderr, "   %s = (unwritten) %s(=%s)->[%d] of type %s\n", cyan(reg(field)), inst.X.Name(), reg(struc), index, field.Type().String())
	} else {
		fmt.Fprintf(os.Stderr, "   "+red("%s = %s->[%d] of type %s/%s")+"\n", reg(field), struc.Name(), index, struc.Type().String(), field.Type().String())
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
	if str, _, ok := fr.getStruct(struc); ok {
		fr.fields[field] = FieldDecomp{
			str: struc,
			idx: index,
		}
		// No need to update local, field is index into fr.fields too.
		fmt.Fprintf(os.Stderr, "   %s = %s(=%s).[%d] of type %s\n", cyan(reg(field)), inst.X.Name(), struc.Name(), index, field.Type().String())
		if str[index] == nil { // Struct exists but field does not
			str[index] = field
			fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as field definition\n", field.Name())
		}
	} else if _, ok := struc.Type().Underlying().(*types.Struct); ok { // struct has not been allocated but is a struct (e.g. uninitialised)
		fr.env.structs[struc] = make(StructFields) // Create external struct
		fr.fields[field] = FieldDecomp{
			str: struc,
			idx: index,
		}
		fmt.Fprintf(os.Stderr, "   %s = (unwritten) %s:%s.[%d] of type %s\n", cyan(reg(field)), inst.X.Name(), struc.Name(), index, field.Type().String())
	} else {
		fmt.Fprintf(os.Stderr, "   "+red("%s = %s.[%d] of type %s")+"\n", reg(field), struc.Name(), index, field.Type().String())
		panic("Wrong type and struct not found")
	}
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

	if arr, _, ok := fr.getArray(array); ok {
		fr.elems[elem] = ElemDecomp{
			arr: array,
			idx: index,
		}
		fmt.Fprintf(os.Stderr, "   %s = &%s(=%s)[%s] of type %s\n", cyan(reg(elem)), inst.X.Name(), array.Name(), index, elem.Type().String())
		if arr[index] == nil { // Array exists but field does not
			arr[index] = elem
			fmt.Fprintf(os.Stderr, "     ^ accessed for the first time: use %s as field definition\n", elem.Name())
		}
	} else if _, ok := deref(array.Type()).Underlying().(*types.Array); ok {
		fr.env.arrays[array] = make(ArrayElems)
		fr.elems[elem] = ElemDecomp{
			arr: array,
			idx: index,
		}
		fmt.Fprintf(os.Stderr, "   %s = (unwritten) &%s(=%s)[%s] of type %s\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
	} else if _, ok := deref(array.Type()).Underlying().(*types.Slice); ok {
		fr.env.arrays[array] = make(ArrayElems)
		fr.elems[elem] = ElemDecomp{
			arr: array,
			idx: index,
		}
		fmt.Fprintf(os.Stderr, "   %s = (unwritten) &%s(=%s)[%s] of type %s\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
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

	if arr, _, ok := fr.getArray(array); ok {
		fr.elems[elem] = ElemDecomp{
			arr: array,
			idx: index,
		}
		fmt.Fprintf(os.Stderr, "   %s = %s(=%s)[%s] of type %s\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
		if arr[index] == nil { // Array exists but field does not
			arr[index] = elem
			fmt.Fprintf(os.Stderr, "     ^ %s accessed for the first time: use %s as field definition\n", reg(elem), elem.Name())
		}
	} else if _, ok := deref(array.Type()).(*types.Array); ok {
		fr.env.arrays[array] = make(ArrayElems)
		fr.elems[elem] = ElemDecomp{
			arr: array,
			idx: index,
		}
		fmt.Fprintf(os.Stderr, "   %s = (unwritten) %s(=%s)[%s] of type %s\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
	} else {
		fmt.Fprintf(os.Stderr, "   "+red("%s = %s(=%s)[%s] of type %s")+"\n", elem.Name(), inst.X.Name(), array.Name(), index.Name(), elem.Type().String())
		panic("Wrong type and array not found")
	}
}

func visitDefer(inst *ssa.Defer, fr *frame) {
	fr.defers = append(fr.defers, inst)
}

func visitRunDefers(inst *ssa.RunDefers, fr *frame) {
	for i := len(fr.defers) - 1; i >= 0; i-- {
		callcommon(fr.defers[i].Value(), fr.defers[i].Common(), fr)
	}
}
