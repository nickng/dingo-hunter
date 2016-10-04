package typeinfer

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/nickng/dingo-hunter/typeinfer/migo"
	"golang.org/x/tools/go/ssa"
)

// visitFunc analyses function body.
func visitFunc(fn *ssa.Function, infer *TypeInfer, f *Function) {
	infer.Env.MigoProg.AddFunction(f.FuncDef)

	infer.Logger.Printf(f.Sprintf(FuncEnterSymbol+"───── func %s ─────", fn.Name()))
	defer infer.Logger.Printf(f.Sprintf(FuncExitSymbol+"───── func %s ─────", fn.Name()))
	if fn.Name() == "init" {
		if _, ok := f.Prog.InitPkgs[fn.Package()]; !ok {
			f.Prog.InitPkgs[fn.Package()] = true
		}
		f.hasBody = true
		return
	}
	for val, instance := range f.locals {
		infer.Logger.Printf(f.Sprintf(ParamSymbol+"%s = %s", val.Name(), instance))
		f.revlookup[instance.String()] = val.Name() // If it comes from params..
	}

	if fn.Blocks == nil {
		infer.Logger.Print(f.Sprintf(MoreSymbol + "« no function body »"))
		f.hasBody = false // No body
		return
	}
	// When entering function, always visit as block 0
	block0 := NewBlock(f, fn.Blocks[0], 0)
	visitBasicBlock(fn.Blocks[0], infer, f, block0, &Loop{Parent: f})
	f.hasBody = true
}

func visitBasicBlock(blk *ssa.BasicBlock, infer *TypeInfer, f *Function, bPrev *Block, l *Loop) {
	loopStateTransition(blk, infer, f, &l)
	if l.Bound == Static && l.HasNext() {
		infer.Logger.Printf(f.Sprintf(BlockSymbol+"%s %d (loop %s=%d)", fmtBlock("block"), blk.Index, l.CondVar.Name(), l.Index))
		// Loop and can continue, so don't mark as visited yet
	} else {
		if _, ok := f.Visited[blk]; ok {
			infer.Logger.Printf(f.Sprintf(BlockSymbol+"%s %d (visited)", fmtBlock("block"), blk.Index))
			f.Visited[blk]++
			return
		}
	}
	infer.Logger.Printf(f.Sprintf(BlockSymbol+"%s %d; %s", fmtBlock("block"), blk.Index, fmtLoopHL(blk.Comment)))
	f.Visited[blk] = 0
	for _, instr := range blk.Instrs {
		visitInstr(instr, infer, &Context{f, bPrev, l})
	}
}

func visitInstr(instr ssa.Instruction, infer *TypeInfer, ctx *Context) {
	switch instr := instr.(type) {
	case *ssa.Alloc:
		visitAlloc(instr, infer, ctx)
	case *ssa.BinOp:
		visitBinOp(instr, infer, ctx)
	case *ssa.Call:
		visitCall(instr, infer, ctx)
	case *ssa.ChangeInterface:
		visitChangeInterface(instr, infer, ctx)
	case *ssa.ChangeType:
		visitChangeType(instr, infer, ctx)
	case *ssa.Convert:
		visitConvert(instr, infer, ctx)
	case *ssa.DebugRef:
		//infer.Logger.Printf(ctx.F.Sprintf(SkipSymbol+"debug\t\t%s", instr))
	case *ssa.Defer:
		visitDefer(instr, infer, ctx)
	case *ssa.Extract:
		visitExtract(instr, infer, ctx)
	case *ssa.FieldAddr:
		visitFieldAddr(instr, infer, ctx)
	case *ssa.Go:
		visitGo(instr, infer, ctx)
	case *ssa.If:
		visitIf(instr, infer, ctx)
	case *ssa.Index:
		visitIndex(instr, infer, ctx)
	case *ssa.IndexAddr:
		visitIndexAddr(instr, infer, ctx)
	case *ssa.Jump:
		visitJump(instr, infer, ctx)
	case *ssa.Lookup:
		visitLookup(instr, infer, ctx)
	case *ssa.MakeChan:
		visitMakeChan(instr, infer, ctx)
	case *ssa.MakeClosure:
		visitMakeClosure(instr, infer, ctx)
	case *ssa.MakeInterface:
		visitMakeInterface(instr, infer, ctx)
	case *ssa.MakeMap:
		visitMakeMap(instr, infer, ctx)
	case *ssa.MakeSlice:
		visitMakeSlice(instr, infer, ctx)
	case *ssa.MapUpdate:
		visitMapUpdate(instr, infer, ctx)
	case *ssa.Next:
		visitNext(instr, infer, ctx)
	case *ssa.Phi:
		visitPhi(instr, infer, ctx)
	case *ssa.Return:
		visitReturn(instr, infer, ctx)
	case *ssa.RunDefers:
		visitRunDefers(instr, infer, ctx)
	case *ssa.Send:
		visitSend(instr, infer, ctx)
	case *ssa.Select:
		visitSelect(instr, infer, ctx)
	case *ssa.Slice:
		visitSlice(instr, infer, ctx)
	case *ssa.Store:
		visitStore(instr, infer, ctx)
	case *ssa.TypeAssert:
		visitTypeAssert(instr, infer, ctx)
	case *ssa.UnOp:
		switch instr.Op {
		case token.ARROW:
			visitRecv(instr, infer, ctx)
		case token.MUL:
			visitDeref(instr, infer, ctx)
		default:
			visitSkip(instr, infer, ctx)
		}
	default:
		visitSkip(instr, infer, ctx)
	}
}

func visitAlloc(instr *ssa.Alloc, infer *TypeInfer, ctx *Context) {
	allocType := instr.Type().(*types.Pointer).Elem()
	switch t := allocType.Underlying().(type) {
	case *types.Array: // Static size array
		ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
		if instr.Heap {
			ctx.F.Prog.arrays[ctx.F.locals[instr]] = make(Elems, t.Len())
			infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc (array@heap) of type %s (%d elems)", ctx.F.locals[instr], instr.Type(), t.Len()))
		} else {
			ctx.F.arrays[ctx.F.locals[instr]] = make(Elems, t.Len())
			infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc (array@local) of type %s (%d elems)", ctx.F.locals[instr], instr.Type(), t.Len()))
		}
	case *types.Struct:
		ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
		if instr.Heap {
			ctx.F.Prog.structs[ctx.F.locals[instr]] = make(Fields, t.NumFields())
			infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc (struct@heap) of type %s (%d fields)", ctx.F.locals[instr], instr.Type(), t.NumFields()))
		} else {
			ctx.F.structs[ctx.F.locals[instr]] = make(Fields, t.NumFields())
			infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc (struct@local) of type %s (%d fields)", ctx.F.locals[instr], instr.Type(), t.NumFields()))
		}
	case *types.Pointer:
		switch pt := t.Elem().Underlying().(type) {
		case *types.Array:
			ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
			if instr.Heap {
				ctx.F.Prog.arrays[ctx.F.locals[instr]] = make(Elems, pt.Len())
				infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc/indirect (array@heap) of type %s (%d elems)", ctx.F.locals[instr], instr.Type(), pt.Len()))
			} else {
				ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
				ctx.F.arrays[ctx.F.locals[instr]] = make(Elems, pt.Len())
				infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc/indirect (array@local) of type %s (%d elems)", ctx.F.locals[instr], instr.Type(), pt.Len()))
			}
		case *types.Struct:
			ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
			if instr.Heap {
				ctx.F.Prog.structs[ctx.F.locals[instr]] = make(Fields, pt.NumFields())
				infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc/indirect (struct@heap) of type %s (%d fields)", ctx.F.locals[instr], instr.Type(), pt.NumFields()))
			} else {
				ctx.F.structs[ctx.F.locals[instr]] = make(Fields, pt.NumFields())
				infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc/indirect (struct@local) of type %s (%d fields)", ctx.F.locals[instr], instr.Type(), pt.NumFields()))
			}
		default:
			ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
			infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc/indirect of type %s", ctx.F.locals[instr], instr.Type().Underlying()))
		}
	default:
		ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
		infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = alloc of type %s", ctx.F.locals[instr], instr.Type().Underlying()))
	}
}

func visitBinOp(instr *ssa.BinOp, infer *TypeInfer, ctx *Context) {
	if ctx.L.State == Enter {
		switch ctx.L.Bound {
		case Unknown:
			switch instr.Op {
			case token.LSS: // i < N
				if i, ok := instr.Y.(*ssa.Const); ok && i.Value.Kind() == constant.Int {
					ctx.L.SetCond(instr, i.Int64()-1)
					if _, ok := instr.X.(*ssa.Phi); ok && ctx.L.Start < ctx.L.End {
						ctx.L.Bound = Static
						infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"i <= %s", fmtLoopHL(ctx.L.End)))
						return
					}
					ctx.L.Bound = Dynamic
					return
				}
			case token.LEQ: // i <= N
				if i, ok := instr.Y.(*ssa.Const); ok && i.Value.Kind() == constant.Int {
					ctx.L.SetCond(instr, i.Int64())
					if _, ok := instr.X.(*ssa.Phi); ok && ctx.L.Start < ctx.L.End {
						ctx.L.Bound = Static
						infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"i <= %s", fmtLoopHL(ctx.L.End)))
						return
					}
					ctx.L.Bound = Dynamic
					return
				}
			case token.GTR: // i > N
				if i, ok := instr.Y.(*ssa.Const); ok && i.Value.Kind() == constant.Int {
					ctx.L.SetCond(instr, i.Int64()+1)
					if _, ok := instr.X.(*ssa.Phi); ok && ctx.L.Start > ctx.L.End {
						ctx.L.Bound = Static
						infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"i > %s", fmtLoopHL(ctx.L.End)))
						return
					}
					ctx.L.Bound = Dynamic
					return
				}
			case token.GEQ: // i >= N
				if i, ok := instr.Y.(*ssa.Const); ok && i.Value.Kind() == constant.Int {
					ctx.L.SetCond(instr, i.Int64())
					if _, ok := instr.X.(*ssa.Phi); ok && ctx.L.Start > ctx.L.End {
						ctx.L.Bound = Static
						infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"i >= %s", fmtLoopHL(ctx.L.End)))
						return
					}
					ctx.L.Bound = Dynamic
					return
				}
			}
		}
	}
	ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
	visitSkip(instr, infer, ctx)
}

func visitCall(instr *ssa.Call, infer *TypeInfer, ctx *Context) {
	infer.Logger.Printf(ctx.F.Sprintf(CallSymbol+"%s = %s", instr.Name(), instr.String()))
	ctx.F.Call(instr, infer, ctx.B, ctx.L)
}

func visitChangeType(instr *ssa.ChangeType, infer *TypeInfer, ctx *Context) {
	inst, ok := ctx.F.locals[instr.X]
	if !ok {
		infer.Logger.Fatalf("changetype: %s: %v → %v", ErrUnknownValue, instr.X, instr)
		return
	}
	ctx.F.locals[instr] = inst
	if a, ok := ctx.F.arrays[ctx.F.locals[instr.X]]; ok {
		ctx.F.arrays[ctx.F.locals[instr]] = a
	}
	if s, ok := ctx.F.structs[ctx.F.locals[instr.X]]; ok {
		ctx.F.structs[ctx.F.locals[instr]] = s
	}
	if m, ok := ctx.F.maps[ctx.F.locals[instr.X]]; ok {
		ctx.F.maps[ctx.F.locals[instr]] = m
	}
	infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s = %s (type: %s ← %s)", instr.Name(), instr.X.Name(), fmtType(instr.Type()), fmtType(instr.X.Type().Underlying())))
	ctx.F.revlookup[instr.Name()] = instr.X.Name()
	if indirect, ok := ctx.F.revlookup[instr.X.Name()]; ok {
		ctx.F.revlookup[instr.Name()] = indirect
	}
	return
}

func visitChangeInterface(instr *ssa.ChangeInterface, infer *TypeInfer, ctx *Context) {
	inst, ok := ctx.F.locals[instr.X]
	if !ok {
		infer.Logger.Fatalf("changeiface: %s: %v → %v", ErrUnknownValue, instr.X, instr)
	}
	ctx.F.locals[instr] = inst
}

func visitConvert(instr *ssa.Convert, infer *TypeInfer, ctx *Context) {
	_, ok := ctx.F.locals[instr.X]
	if !ok {
		if c, ok := instr.X.(*ssa.Const); ok {
			ctx.F.locals[instr.X] = &Const{c}
		} else if _, ok := instr.X.(*ssa.Global); ok {
			inst, ok := ctx.F.Prog.globals[instr.X]
			if !ok {
				infer.Logger.Fatalf("convert (global): %s: %+v", ErrUnknownValue, instr.X)
			}
			ctx.F.locals[instr.X] = inst
			infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s convert= %s (global)", ctx.F.locals[instr], instr.X.Name()))
			return
		} else {
			infer.Logger.Fatalf("convert: %s: %+v", ErrUnknownValue, instr.X)
			return
		}
	}
	ctx.F.locals[instr] = ctx.F.locals[instr.X]
	infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s convert= %s", ctx.F.locals[instr], instr.X.Name()))
}

func visitDefer(instr *ssa.Defer, infer *TypeInfer, ctx *Context) {
	ctx.F.defers = append(ctx.F.defers, instr)
}

func visitDeref(instr *ssa.UnOp, infer *TypeInfer, ctx *Context) {
	ptr, val := instr.X, instr
	// Globactx.L.
	if _, ok := ptr.(*ssa.Global); ok {
		inst, ok := ctx.F.Prog.globals[ptr]
		if !ok {
			infer.Logger.Fatalf("deref (global): %s: %+v", ErrUnknownValue, ptr)
			return
		}
		ctx.F.locals[ptr], ctx.F.locals[val] = inst, inst
		infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s deref= %s (global) of type %s", inst, ptr, ptr.Type()))
		// Initialise global array/struct if needed.
		initNestedRefVar(infer, ctx, ctx.F.locals[ptr], true)
		return
	}
	if basic, ok := derefType(ptr.Type()).Underlying().(*types.Basic); ok && basic.Kind() == types.Byte {
		infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"deref: %+v is a byte", ptr))
		// Create new byte instance here, bytes do no need explicit allocation.
		ctx.F.locals[ptr] = &Value{ptr, ctx.F.InstanceID(), ctx.L.Index}
	}
	// Locactx.L.
	inst, ok := ctx.F.locals[ptr]
	if !ok {
		infer.Logger.Fatalf("deref: %s: %+v", ErrUnknownValue, ptr)
		return
	}
	ctx.F.locals[ptr], ctx.F.locals[val] = inst, inst
	infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s deref= %s of type %s", val, ptr, ptr.Type()))
	// Initialise array/struct if needed.
	initNestedRefVar(infer, ctx, ctx.F.locals[ptr], false)
	return
}

func visitExtract(instr *ssa.Extract, infer *TypeInfer, ctx *Context) {
	if tupleInst, ok := ctx.F.locals[instr.Tuple]; ok {
		if _, ok := ctx.F.tuples[tupleInst]; !ok { // Tuple uninitialised
			infer.Logger.Fatalf("extract: %s: Unexpected tuple: %+v", ErrUnknownValue, instr)
			return
		}
		if inst := ctx.F.tuples[tupleInst][instr.Index]; inst == nil {
			ctx.F.tuples[tupleInst][instr.Index] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
		}
		ctx.F.locals[instr] = ctx.F.tuples[tupleInst][instr.Index]
		initNestedRefVar(infer, ctx, ctx.F.locals[instr], false)
		// Detect select tuple.
		if _, ok := ctx.F.selects[tupleInst]; ok {
			switch instr.Index {
			case 0:
				ctx.F.selects[tupleInst].Index = ctx.F.locals[instr]
				infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = extract select{%d} (select-index) for %s", ctx.F.locals[instr], instr.Index, instr))
				return
			default:
				infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = extract select{%d} for %s", ctx.F.locals[instr], instr.Index, instr))
				return
			}
		}
		// Detect commaok tuple.
		if commaOk, ok := ctx.F.commaok[tupleInst]; ok {
			switch instr.Index {
			case 0:
				infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = extract commaOk{%d} for %s", ctx.F.locals[instr], instr.Index, instr))
				return
			case 1:
				commaOk.OkCond = ctx.F.locals[instr]
				infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = extract commaOk{%d} (ok-test) for %s", ctx.F.locals[instr], instr.Index, instr))
				return
			}
		}
		infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = tuple %s[%d] of %d", ctx.F.locals[instr], tupleInst, instr.Index, len(ctx.F.tuples[tupleInst])))
		return
	}
}

func visitField(instr *ssa.Field, infer *TypeInfer, ctx *Context) {
	field, struc, index := instr, instr.X, instr.Field
	if sType, ok := struc.Type().Underlying().(*types.Struct); ok {
		sInst, ok := ctx.F.locals[struc]
		if !ok {
			infer.Logger.Fatalf("field: %s :%+v", ErrUnknownValue, struc)
			return
		}
		fields, ok := ctx.F.structs[sInst]
		if !ok {
			fields, ok = ctx.F.Prog.structs[sInst]
			if !ok {
				infer.Logger.Fatalf("field: %s: struct uninitialised %+v", ErrUnknownValue, sInst)
				return
			}
		}
		infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s = %s"+FieldSymbol+"{%d} of type %s", instr.Name(), sInst, index, sType.String()))
		if fields[index] != nil {
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"accessed as %s", fields[index]))
		} else {
			fields[index] = &Value{field, ctx.F.InstanceID(), ctx.L.Index}
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"field uninitialised, set to %s", field.Name()))
		}
		initNestedRefVar(infer, ctx, ctx.F.locals[field], false)
		ctx.F.locals[field] = fields[index]
		return
	}
	infer.Logger.Fatalf("field: %s: field is not struct: %+v", ErrInvalidVarRead, struc)
}

func visitFieldAddr(instr *ssa.FieldAddr, infer *TypeInfer, ctx *Context) {
	field, struc, index := instr, instr.X, instr.Field
	if sType, ok := derefType(struc.Type()).Underlying().(*types.Struct); ok {
		sInst, ok := ctx.F.locals[struc]
		if !ok {
			sInst, ok = ctx.F.Prog.globals[struc]
			if !ok {
				infer.Logger.Fatalf("field-addr: %s: %+v", ErrUnknownValue, struc)
				return
			}
		}
		// Check status of instance.
		switch inst := sInst.(type) {
		case *Value: // Continue
		case *External: // Continue
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"field-addr: %+v is external", sInst))
			ctx.F.locals[field] = inst
			return
		case *Const:
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"field-addr: %+v is a constant", sInst))
			if inst.Const.IsNil() {
				ctx.F.locals[field] = inst
			}
			return
		default:
			infer.Logger.Fatalf("field-addr: %s: not instance %+v", ErrUnknownValue, sInst)
			return
		}
		// Find the struct.
		fields, ok := ctx.F.structs[sInst]
		if !ok {
			fields, ok = ctx.F.Prog.structs[sInst]
			if !ok {
				infer.Logger.Fatalf("field-addr: %s: struct uninitialised %+v", ErrUnknownValue, sInst)
				return
			}
		}
		infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s = %s"+FieldSymbol+"{%d} of type %s", instr.Name(), sInst, index, sType.String()))
		if fields[index] != nil {
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"accessed as %s", fields[index]))
		} else {
			fields[index] = &Value{field, ctx.F.InstanceID(), ctx.L.Index}
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"field uninitialised, set to %s", field.Name()))
		}
		initNestedRefVar(infer, ctx, fields[index], false)
		ctx.F.locals[field] = fields[index]
		return
	}
	infer.Logger.Fatalf("field-addr: %s: field is not struct: %+v", ErrInvalidVarRead, struc)
}

func visitGo(instr *ssa.Go, infer *TypeInfer, ctx *Context) {
	infer.Logger.Printf(ctx.F.Sprintf(SpawnSymbol+"%s %s", fmtSpawn("spawn"), instr))
	ctx.F.Go(instr, infer)
}

func visitIf(instr *ssa.If, infer *TypeInfer, ctx *Context) {
	if len(instr.Block().Succs) != 2 {
		infer.Logger.Fatal(ErrInvalidIfSucc)
	}
	// Detect and unroll ctx.L.
	if ctx.L.State != NonLoop && ctx.L.Bound == Static && instr.Cond == ctx.L.CondVar {
		if ctx.L.HasNext() {
			infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"loop continue %s", ctx.L))
			visitBasicBlock(instr.Block().Succs[0], infer, ctx.F, NewBlock(ctx.F, instr.Block().Succs[0], ctx.B.Index), ctx.L)
		} else {
			infer.Logger.Printf(ctx.F.Sprintf(LoopSymbol+"loop exit %s", ctx.L))
			//ctx.F.Visited[instr.Block()] = 0
			visitBasicBlock(instr.Block().Succs[1], infer, ctx.F, NewBlock(ctx.F, instr.Block().Succs[1], ctx.B.Index), ctx.L)
		}
		return
	}
	// Detect Select branches.
	if bin, ok := instr.Cond.(*ssa.BinOp); ok && bin.Op == token.EQL {
		for _, sel := range ctx.F.selects {
			if bin.X == sel.Index.(*Value).Value {
				if i, ok := bin.Y.(*ssa.Const); ok && i.Value.Kind() == constant.Int {
					//infer.Logger.Print(fmt.Sprintf("[select-%d]", i.Int64()), ctx.F.FuncDef.String())
					parDef := ctx.F.FuncDef
					parDef.PutAway() // Save select
					visitBasicBlock(instr.Block().Succs[0], infer, ctx.F, NewBlock(ctx.F, instr.Block().Succs[0], ctx.B.Index), ctx.L)
					ctx.F.FuncDef.PutAway() // Save case
					selCase, err := ctx.F.FuncDef.Restore()
					if err != nil {
						infer.Logger.Fatal("select-case:", err)
					}
					sel.MigoStmt.Cases[i.Int64()] = append(sel.MigoStmt.Cases[i.Int64()], selCase...)
					selParent, err := parDef.Restore()
					if err != nil {
						infer.Logger.Fatal("select-parent:", err)
					}
					parDef.AddStmts(selParent...)

					// Test if select has default branch & if this is default
					if !sel.Instr.Blocking && i.Int64() == int64(len(sel.Instr.States)-1) {
						infer.Logger.Print(ctx.F.Sprintf(SelectSymbol + "default"))
						parDef := ctx.F.FuncDef
						parDef.PutAway() // Save select
						visitBasicBlock(instr.Block().Succs[1], infer, ctx.F, NewBlock(ctx.F, instr.Block().Succs[1], ctx.B.Index), ctx.L)
						ctx.F.FuncDef.PutAway() // Save case
						selDefault, err := ctx.F.FuncDef.Restore()
						if err != nil {
							infer.Logger.Fatal("select-default:", err)
						}
						sel.MigoStmt.Cases[len(sel.MigoStmt.Cases)-1] = append(sel.MigoStmt.Cases[len(sel.MigoStmt.Cases)-1], selDefault...)
						selParent, err := parDef.Restore()
						if err != nil {
							infer.Logger.Fatal("select-parent:", err)
						}
						parDef.AddStmts(selParent...)
					} else {
						infer.Logger.Printf(ctx.F.Sprintf(IfSymbol+"select-else "+JumpSymbol+"%d", instr.Block().Succs[1].Index))
						visitBasicBlock(instr.Block().Succs[1], infer, ctx.F, NewBlock(ctx.F, instr.Block().Succs[1], ctx.B.Index), ctx.L)
					}
					return // Select if-then-else handled
				}
			}
		}
	}

	var cond string
	if inst, ok := ctx.F.locals[instr.Cond]; ok && isCommaOk(ctx.F, inst) {
		cond = fmt.Sprintf("comma-ok %s", instr.Cond.Name())
	} else {
		cond = fmt.Sprintf("%s", instr.Cond.Name())
	}

	// Save parent.
	ctx.F.FuncDef.PutAway()
	infer.Logger.Printf(ctx.F.Sprintf(IfSymbol+"if %s then"+JumpSymbol+"%d", cond, instr.Block().Succs[0].Index))
	visitBasicBlock(instr.Block().Succs[0], infer, ctx.F, NewBlock(ctx.F, instr.Block().Succs[0], ctx.B.Index), ctx.L)
	// Save then.
	ctx.F.FuncDef.PutAway()
	infer.Logger.Printf(ctx.F.Sprintf(IfSymbol+"if %s else"+JumpSymbol+"%d", cond, instr.Block().Succs[1].Index))
	visitBasicBlock(instr.Block().Succs[1], infer, ctx.F, NewBlock(ctx.F, instr.Block().Succs[1], ctx.B.Index), ctx.L)
	// Save else.
	ctx.F.FuncDef.PutAway()
	elseStmts, err := ctx.F.FuncDef.Restore() // Else
	if err != nil {
		infer.Logger.Fatal("restore else:", err)
	}
	thenStmts, err := ctx.F.FuncDef.Restore() // Then
	if err != nil {
		infer.Logger.Fatal("restore then:", err)
	}
	parentStmts, err := ctx.F.FuncDef.Restore() // Parent
	if err != nil {
		infer.Logger.Fatal("restore if-then-else parent:", err)
	}
	ctx.F.FuncDef.AddStmts(parentStmts...)
	ctx.F.FuncDef.AddStmts(&migo.IfStatement{Then: thenStmts, Else: elseStmts})
}

func visitIndex(instr *ssa.Index, infer *TypeInfer, ctx *Context) {
	elem, array, index := instr, instr.X, instr.Index
	// Array.
	if aType, ok := array.Type().Underlying().(*types.Array); ok {
		aInst, ok := ctx.F.locals[array]
		if !ok {
			aInst, ok = ctx.F.Prog.globals[array]
			if !ok {
				infer.Logger.Fatalf("index: %s: array %+v", ErrUnknownValue, array)
				return
			}
		}
		elems, ok := ctx.F.arrays[aInst]
		if !ok {
			elems, ok = ctx.F.Prog.arrays[aInst]
			if !ok {
				infer.Logger.Fatalf("index: %s: not an array %+v", ErrUnknownValue, aInst)
				return
			}
		}
		infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s = %s"+FieldSymbol+"[%s] of type %s", instr.Name(), aInst, index, aType.String()))
		if elems[index] != nil {
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"accessed as %s", elems[index]))
		} else {
			elems[index] = &Value{elem, ctx.F.InstanceID(), ctx.L.Index}
			infer.Logger.Printf(ctx.F.Sprintf(SubSymbol+"elem uninitialised, set to %s", elem.Name()))
		}
		initNestedRefVar(infer, ctx, elems[index], false)
		ctx.F.locals[elem] = elems[index]
		return
	}
}

func visitIndexAddr(instr *ssa.IndexAddr, infer *TypeInfer, ctx *Context) {
	elem, array, index := instr, instr.X, instr.Index
	// Array.
	if aType, ok := derefType(array.Type()).Underlying().(*types.Array); ok {
		aInst, ok := ctx.F.locals[array]
		if !ok {
			aInst, ok = ctx.F.Prog.globals[array]
			if !ok {
				infer.Logger.Fatalf("index-addr: %s: array %+v", ErrUnknownValue, array)
				return
			}
		}
		// Check status of instance.
		switch inst := aInst.(type) {
		case *Value: // Continue
		case *External: // External
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"index-addr: array %+v is external", aInst))
			ctx.F.locals[elem] = inst
		case *Const:
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"index-addr: array %+v is a constant", aInst))
			if inst.Const.IsNil() {
				ctx.F.locals[elem] = inst
			}
			return
		default:
			infer.Logger.Fatalf("index-addr: %s: array is not instance %+v", ErrUnknownValue, aInst)
			return
		}
		// Find the array.
		elems, ok := ctx.F.arrays[aInst]
		if !ok {
			elems, ok = ctx.F.Prog.arrays[aInst]
			if !ok {
				infer.Logger.Fatalf("index-addr: %s: array uninitialised %s", ErrUnknownValue, aInst)
				return
			}
		}
		infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s = %s"+FieldSymbol+"[%s] of type %s", instr.Name(), aInst, index, aType.String()))
		if elems[index] != nil {
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"accessed as %s", elems[index]))
		} else {
			elems[index] = &Value{elem, ctx.F.InstanceID(), ctx.L.Index}
			infer.Logger.Printf(ctx.F.Sprintf(SubSymbol+"elem uninitialised, set to %s", elem.Name()))
		}
		initNestedRefVar(infer, ctx, elems[index], false)
		ctx.F.locals[elem] = elems[index]
		return
	}
	// Slices.
	if sType, ok := derefType(array.Type()).Underlying().(*types.Slice); ok {
		sInst, ok := ctx.F.locals[array]
		if !ok {
			sInst, ok = ctx.F.Prog.globals[array]
			if !ok {
				infer.Logger.Fatalf("index-addr: %s: slice %+v", ErrUnknownValue, array)
				return
			}
		}
		// Check status of instance.
		switch inst := sInst.(type) {
		case *Value: // Continue
			if basic, ok := sType.Elem().(*types.Basic); ok && basic.Kind() == types.Byte {
				ctx.F.locals[elem] = inst
				return
			}
		case *External: // External
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"index-addr: slice %+v is external", sInst))
			ctx.F.locals[elem] = inst
			return
		case *Const:
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"index-addr: slice %+v is a constant", sInst))
			if inst.Const.IsNil() {
				ctx.F.locals[elem] = inst
			}
			return
		default:
			infer.Logger.Fatalf("index-addr: %s: slice is not instance %+v", ErrUnknownValue, sInst)
			return
		}
		// Find the slice.
		elems, ok := ctx.F.arrays[sInst]
		if !ok {
			elems, ok = ctx.F.Prog.arrays[sInst]
			if !ok {
				infer.Logger.Fatalf("index-addr: %s: slice uninitialised %+v", ErrUnknownValue, sInst)
				return
			}
		}
		infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s = %s"+FieldSymbol+"[%s] (slice) of type %s", instr.Name(), sInst, index, sType.String()))
		if elems[index] != nil {
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"accessed as %s", elems[index]))
		} else {
			elems[index] = &Value{elem, ctx.F.InstanceID(), ctx.L.Index}
			infer.Logger.Printf(ctx.F.Sprintf(SubSymbol+"elem uninitialised, set to %s", elem.Name()))
		}
		initNestedRefVar(infer, ctx, elems[index], false)
		ctx.F.locals[elem] = elems[index]
		return
	}
	infer.Logger.Fatalf("index-addr: %s: not array/slice %+v", ErrInvalidVarRead, array)
}

func visitJump(jump *ssa.Jump, infer *TypeInfer, ctx *Context) {
	if len(jump.Block().Succs) != 1 {
		infer.Logger.Fatal(ErrInvalidJumpSucc)
	}
	curr, next := jump.Block(), jump.Block().Succs[0]
	infer.Logger.Printf(ctx.F.Sprintf(SkipSymbol+"block %d%s%d", curr.Index, fmtLoopHL(JumpSymbol), next.Index))
	switch ctx.L.State {
	case Exit:
		ctx.L.State = NonLoop
	}
	if len(next.Preds) > 1 {
		infer.Logger.Printf(ctx.F.Sprintf(SplitSymbol+"Jump (%d ⇾ %d) %s", curr.Index, next.Index, ctx.L.String()))
		var stmt *migo.CallStatement
		if ctx.L.Bound == Static && ctx.L.HasNext() {
			stmt = &migo.CallStatement{Name: fmt.Sprintf("%s#%d_loop%d", ctx.F.Fn.String(), next.Index, ctx.L.Index), Params: []*migo.Parameter{}}
		} else {
			stmt = &migo.CallStatement{Name: fmt.Sprintf("%s#%d", ctx.F.Fn.String(), next.Index)}
			for _, p := range ctx.F.FuncDef.Params {
				stmt.AddParams(&migo.Parameter{Caller: p.Callee, Callee: p.Callee})
			}
		}
		//for _, s := range ctx.F.FuncDef.Stmts {
		//	if nc, ok := s.(*migo.NewChanStatement); ok {
		//		stmt.AddParams(&migo.Parameter{Caller: nc.Name, Callee: nc.Name})
		//	}
		//}
		//for _, p := range ctx.F.FuncDef.Params {
		//	stmt.AddParams(&migo.Parameter{Caller: p.Callee, Callee: p.Callee})
		//}
		ctx.F.FuncDef.AddStmts(stmt)
		if _, visited := ctx.F.Visited[next]; !visited {
			newBlock := NewBlock(ctx.F, next, ctx.B.Index)
			oldFunc, newFunc := ctx.F.FuncDef, newBlock.MigoDef
			if ctx.L.Bound == Static && ctx.L.HasNext() {
				newFunc = migo.NewFunction(fmt.Sprintf("%s#%d_loop%d", ctx.F.Fn.String(), next.Index, ctx.L.Index))
			}
			for _, p := range stmt.Params {
				newFunc.AddParams(&migo.Parameter{Caller: p.Callee, Callee: p.Callee})
			}
			ctx.F.FuncDef = newFunc
			infer.Env.MigoProg.AddFunction(newFunc)
			visitBasicBlock(next, infer, ctx.F, newBlock, ctx.L)
			ctx.F.FuncDef = oldFunc
			return
		}
	}
	visitBasicBlock(next, infer, ctx.F, NewBlock(ctx.F, next, ctx.B.Index), ctx.L)
}

func visitLookup(instr *ssa.Lookup, infer *TypeInfer, ctx *Context) {
	v, ok := ctx.F.locals[instr.X]
	if !ok {
		if c, ok := instr.X.(*ssa.Const); ok {
			ctx.F.locals[instr.X] = &Const{c}
			v = ctx.F.locals[instr.X]
		} else {
			infer.Logger.Fatalf("lookup: %s: %+v", ErrUnknownValue, instr.X)
			return
		}
	}
	// Lookup test.
	idx, ok := ctx.F.locals[instr.Index]
	if !ok {
		if c, ok := instr.Index.(*ssa.Const); ok {
			idx = &Const{c}
		} else {
			idx = &Value{instr.Index, ctx.F.InstanceID(), ctx.L.Index}
		}
		ctx.F.locals[instr.Index] = idx
	}
	ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
	initNestedRefVar(infer, ctx, ctx.F.locals[instr], false)
	if instr.CommaOk {
		ctx.F.commaok[ctx.F.locals[instr]] = &CommaOk{Instr: instr, Result: ctx.F.locals[instr]}
		ctx.F.tuples[ctx.F.locals[instr]] = make(Tuples, 2) // { elem, lookupOk }
	}
	infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = lookup %s[%s]", ctx.F.locals[instr], v, idx))
}

func visitMakeChan(instr *ssa.MakeChan, infer *TypeInfer, ctx *Context) {
	newch := &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
	ctx.F.locals[instr] = newch
	chType, ok := instr.Type().(*types.Chan)
	if !ok {
		infer.Logger.Fatal(ErrMakeChanNonChan)
	}
	bufSz, ok := instr.Size.(*ssa.Const)
	if !ok {
		infer.Logger.Fatal(ErrNonConstChanBuf)
	}
	infer.Logger.Printf(ctx.F.Sprintf(ChanSymbol+"%s = %s {t:%s, buf:%d} @ %s",
		newch,
		fmtChan("chan"),
		chType.Elem(),
		bufSz.Int64(),
		fmtPos(infer.SSA.FSet.Position(instr.Pos()).String())))
	ctx.F.FuncDef.AddStmts(&migo.NewChanStatement{Name: instr, Chan: newch.String(), Size: bufSz.Int64()})
}

func visitMakeClosure(instr *ssa.MakeClosure, infer *TypeInfer, ctx *Context) {
	ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
	ctx.F.Prog.closures[ctx.F.locals[instr]] = make(Captures, len(instr.Bindings))
	for i, binding := range instr.Bindings {
		ctx.F.Prog.closures[ctx.F.locals[instr]][i] = ctx.F.locals[binding]
	}
	infer.Logger.Print(ctx.F.Sprintf(NewSymbol+"%s = make closure", ctx.F.locals[instr]))
}

func visitMakeInterface(instr *ssa.MakeInterface, infer *TypeInfer, ctx *Context) {
	iface, ok := ctx.F.locals[instr.X]
	if !ok {
		if c, ok := instr.X.(*ssa.Const); ok {
			ctx.F.locals[instr.X] = &Const{c}
		} else {
			infer.Logger.Fatalf("make-iface: %s: %s", ErrUnknownValue, instr.X)
			return
		}
	}
	ctx.F.locals[instr] = iface
	infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = make-iface %s", ctx.F.locals[instr], instr.String()))
}

func visitMakeMap(instr *ssa.MakeMap, infer *TypeInfer, ctx *Context) {
	ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
	ctx.F.maps[ctx.F.locals[instr]] = make(map[Instance]Instance)
	infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = make-map", ctx.F.locals[instr]))
}

func visitMakeSlice(instr *ssa.MakeSlice, infer *TypeInfer, ctx *Context) {
	ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
	ctx.F.arrays[ctx.F.locals[instr]] = make(Elems)
	infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = make-slice", ctx.F.locals[instr]))
}

func visitMapUpdate(instr *ssa.MapUpdate, infer *TypeInfer, ctx *Context) {
	inst, ok := ctx.F.locals[instr.Map]
	if !ok {
		infer.Logger.Fatalf("map-update: %s: %s", ErrUnknownValue, instr.Map)
		return
	}
	m, ok := ctx.F.maps[inst]
	if !ok {
		ctx.F.maps[inst] = make(map[Instance]Instance) // XXX This shouldn't happen
		m = ctx.F.maps[inst]                           // The map must be defined somewhere we skipped
		infer.Logger.Printf("map-update: uninitialised map: %+v %s", instr.Map, instr.Map.String())
	}
	k, ok := ctx.F.locals[instr.Key]
	if !ok {
		k = &Value{instr.Key, ctx.F.InstanceID(), ctx.L.Index}
		ctx.F.locals[instr.Key] = k
	}
	v, ok := ctx.F.locals[instr.Value]
	if !ok {
		if c, ok := instr.Value.(*ssa.Const); ok {
			v = &Const{c}
		} else {
			v = &Value{instr.Value, ctx.F.InstanceID(), ctx.L.Index}
		}
		ctx.F.locals[instr.Value] = v
	}
	m[k] = v
	infer.Logger.Printf(ctx.F.Sprintf(SkipSymbol+"%s[%s] = %s", inst, k, v))
}

func visitNext(instr *ssa.Next, infer *TypeInfer, ctx *Context) {
	ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
	ctx.F.tuples[ctx.F.locals[instr]] = make(Tuples, 3) // { ok, k, v}
	infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s (ok, k, v) = next", ctx.F.locals[instr]))
}

func visitPhi(instr *ssa.Phi, infer *TypeInfer, ctx *Context) {
	loopDetectBounds(instr, infer, ctx)
}

func visitRecv(instr *ssa.UnOp, infer *TypeInfer, ctx *Context) {
	ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index} // received value
	ch, ok := ctx.F.locals[instr.X]
	if !ok { // Channel does not exist
		infer.Logger.Fatalf("recv: %s: %+v", ErrUnknownValue, instr.X)
		return
	}
	// Receive test.
	if instr.CommaOk {
		ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
		ctx.F.commaok[ctx.F.locals[instr]] = &CommaOk{Instr: instr, Result: ctx.F.locals[instr]}
		ctx.F.tuples[ctx.F.locals[instr]] = make(Tuples, 2) // { recvVal, recvOk }
	}
	pos := infer.SSA.DecodePos(ch.(*Value).Pos())
	infer.Logger.Print(ctx.F.Sprintf(RecvSymbol+"%s = %s @ %s", ctx.F.locals[instr], ch, fmtPos(pos)))
	if paramName, ok := ctx.F.revlookup[ch.String()]; ok {
		ctx.F.FuncDef.AddStmts(&migo.RecvStatement{Chan: paramName})
	} else {
		ctx.F.FuncDef.AddStmts(&migo.RecvStatement{Chan: ch.(*Value).Name()})
	}
	ctx.F.FuncDef.HasComm = true

	// Initialise received value if needed.
	initNestedRefVar(infer, ctx, ctx.F.locals[instr], false)
}

func visitReturn(ret *ssa.Return, infer *TypeInfer, ctx *Context) {
	switch len(ret.Results) {
	case 0:
		infer.Logger.Printf(ctx.F.Sprintf(ReturnSymbol))
	case 1:
		if c, ok := ret.Results[0].(*ssa.Const); ok {
			ctx.F.locals[ret.Results[0]] = &Const{c}
		}
		res, ok := ctx.F.locals[ret.Results[0]]
		if !ok {
			infer.Logger.Printf("Returning uninitialised value %s/%s", ret.Results[0].Name(), ctx.F.locals[ret.Results[0]])
			return
		}
		ctx.F.retvals = append(ctx.F.retvals, ctx.F.locals[ret.Results[0]])
		infer.Logger.Printf(ctx.F.Sprintf(ReturnSymbol+"return[1] %s %v", res, ctx.F.retvals))
	default:
		for _, res := range ret.Results {
			ctx.F.retvals = append(ctx.F.retvals, ctx.F.locals[res])
		}
		infer.Logger.Printf(ctx.F.Sprintf(ReturnSymbol+"return[%d] %v", len(ret.Results), ctx.F.retvals))
	}
}

func visitRunDefers(instr *ssa.RunDefers, infer *TypeInfer, ctx *Context) {
	for i := len(ctx.F.defers) - 1; i >= 0; i-- {
		common := ctx.F.defers[i].Common()
		if common.StaticCallee() != nil {
			callee := ctx.F.prepareCallFn(common, common.StaticCallee(), nil)
			visitFunc(callee.Fn, infer, callee)
			if callee.HasBody() {
				callStmt := &migo.CallStatement{Name: callee.Fn.String(), Params: []*migo.Parameter{}}
				for _, c := range common.Args {
					if _, ok := c.Type().(*types.Chan); ok {
						infer.Logger.Fatalf("channel in defer: %s", ErrUnimplemented)
					}
				}
				callee.FuncDef.AddStmts(callStmt)
			}
		}
	}
}

func visitSelect(instr *ssa.Select, infer *TypeInfer, ctx *Context) {
	ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
	ctx.F.selects[ctx.F.locals[instr]] = &Select{
		Instr:    instr,
		MigoStmt: &migo.SelectStatement{Cases: [][]migo.Statement{}},
	}
	selStmt := ctx.F.selects[ctx.F.locals[instr]].MigoStmt
	for _, sel := range instr.States {
		ch, ok := ctx.F.locals[sel.Chan]
		if !ok {
			infer.Logger.Print("Select found an unknown channel", sel.Chan.String())
		}
		var stmt migo.Statement
		//c := getChan(ch.Var(), infer)
		switch sel.Dir {
		case types.SendOnly:
			stmt = &migo.SendStatement{Chan: ch.(*Value).Name()}
		case types.RecvOnly:
			stmt = &migo.RecvStatement{Chan: ch.(*Value).Name()}
		}
		selStmt.Cases = append(selStmt.Cases, []migo.Statement{stmt})
	}
	// Default case exists.
	if !instr.Blocking {
		selStmt.Cases = append(selStmt.Cases, []migo.Statement{&migo.TauStatement{}})
	}
	ctx.F.tuples[ctx.F.locals[instr]] = make(Tuples, 2+len(selStmt.Cases)) // index + recvok + cases
	ctx.F.FuncDef.AddStmts(selStmt)
	ctx.F.FuncDef.HasComm = true
	infer.Logger.Print(ctx.F.Sprintf(SelectSymbol+" %d cases %s = %s", 2+len(selStmt.Cases), instr.Name(), instr.String()))
}

func visitSend(instr *ssa.Send, infer *TypeInfer, ctx *Context) {
	ch, ok := ctx.F.locals[instr.Chan]
	if !ok {
		infer.Logger.Fatalf("send: %s: %+v", ErrUnknownValue, instr.Chan)
	}
	pos := infer.SSA.DecodePos(ch.(*Value).Pos())
	infer.Logger.Printf(ctx.F.Sprintf(SendSymbol+"%s @ %s", ch, fmtPos(pos)))
	if paramName, ok := ctx.F.revlookup[ch.String()]; ok {
		ctx.F.FuncDef.AddStmts(&migo.SendStatement{Chan: paramName})
	} else {
		ctx.F.FuncDef.AddStmts(&migo.SendStatement{Chan: ch.(*Value).Name()})
	}
	ctx.F.FuncDef.HasComm = true
}

func visitSkip(instr ssa.Instruction, infer *TypeInfer, ctx *Context) {
	if v, isVal := instr.(ssa.Value); isVal {
		infer.Logger.Printf(ctx.F.Sprintf(SkipSymbol+"%T\t%s = %s", v, v.Name(), v.String()))
		return
	}
	infer.Logger.Printf(ctx.F.Sprintf(SkipSymbol+"%T\t%s", instr, instr))
}

func visitSlice(instr *ssa.Slice, infer *TypeInfer, ctx *Context) {
	ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
	if _, ok := ctx.F.locals[instr.X]; !ok {
		infer.Logger.Fatalf("slice: %s: %+v", ErrUnknownValue, instr.X)
		return
	}
	if basic, ok := instr.Type().Underlying().(*types.Basic); ok && basic.Kind() == types.String {
		infer.Logger.Printf(ctx.F.Sprintf(SkipSymbol+"%s = slice on string, skipping", ctx.F.locals[instr]))
		return
	}
	if slice, ok := instr.Type().Underlying().(*types.Slice); ok {
		if basic, ok := slice.Elem().Underlying().(*types.Basic); ok && basic.Kind() == types.Byte {
			infer.Logger.Printf(ctx.F.Sprintf(SkipSymbol+"%s = slice on byte, skipping", ctx.F.locals[instr]))
			return
		}
	}
	aInst, ok := ctx.F.arrays[ctx.F.locals[instr.X]]
	if !ok {
		aInst, ok = ctx.F.Prog.arrays[ctx.F.locals[instr.X]]
		if !ok {
			switch ctx.F.locals[instr.X].(type) {
			case *Value: // Continue
				infer.Logger.Fatalf("slice: %s: non-slice %+v", ErrUnknownValue, instr.X)
				return
			case *Const:
				ctx.F.arrays[ctx.F.locals[instr.X]] = make(Elems)
				aInst = ctx.F.arrays[ctx.F.locals[instr.X]]
				infer.Logger.Print(ctx.F.Sprintf("slice: const %s %s", instr.X.Name(), ctx.F.locals[instr.X]))
				return
			}
		}
		ctx.F.Prog.arrays[ctx.F.locals[instr]] = aInst
		infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s = slice %s", ctx.F.locals[instr], ctx.F.locals[instr.X]))
		return
	}
	ctx.F.arrays[ctx.F.locals[instr]] = aInst
	infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s = slice %s", ctx.F.locals[instr], ctx.F.locals[instr.X]))
}

func visitStore(instr *ssa.Store, infer *TypeInfer, ctx *Context) {
	source, dstPtr := instr.Val, instr.Addr
	// Globactx.L.
	if _, ok := dstPtr.(*ssa.Global); ok {
		dstInst, ok := ctx.F.Prog.globals[dstPtr]
		if !ok {
			infer.Logger.Fatalf("store (global): %s: %+v", ErrUnknownValue, dstPtr)
		}
		inst, ok := ctx.F.locals[source]
		if !ok {
			inst, ok = ctx.F.Prog.globals[source]
			if !ok {
				if c, ok := source.(*ssa.Const); ok {
					inst = &Const{c}
				} else {
					infer.Logger.Fatalf("store (global): %s: %+v", ErrUnknownValue, source)
				}
			}
		}
		ctx.F.Prog.globals[dstPtr] = inst
		switch source.Type().Underlying().(type) {
		case *types.Array:
			ctx.F.updateInstances(dstInst, inst)
		case *types.Slice:
			ctx.F.updateInstances(dstInst, inst)
		case *types.Struct:
			ctx.F.updateInstances(dstInst, inst)
		case *types.Map:
			ctx.F.updateInstances(dstInst, inst)
		default:
			// Nothing to update.
		}
		infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"%s = %s (global)", dstPtr.Name(), ctx.F.locals[source]))
		return
	}
	if basic, ok := derefType(dstPtr.Type()).Underlying().(*types.Basic); ok && basic.Kind() == types.Byte {
		infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"store: %+v is a byte", dstPtr))
		ctx.F.locals[dstPtr] = &Value{dstPtr, ctx.F.InstanceID(), ctx.L.Index}
	}
	// Locactx.L.
	dstInst, ok := ctx.F.locals[dstPtr]
	if !ok {
		infer.Logger.Fatalf("store: addr %s: %+v", ErrUnknownValue, dstPtr)
	}
	inst, ok := ctx.F.locals[source]
	if !ok {
		if c, ok := source.(*ssa.Const); ok {
			inst = &Const{c}
		} else {
			infer.Logger.Printf("store: val %s%s: %s", source.Name(), source.Type(), ErrUnknownValue)
		}
	}
	ctx.F.locals[dstPtr] = inst
	switch source.Type().Underlying().(type) {
	case *types.Array:
		ctx.F.updateInstances(dstInst, inst)
	case *types.Slice:
		ctx.F.updateInstances(dstInst, inst)
	case *types.Struct:
		ctx.F.updateInstances(dstInst, inst)
	case *types.Map:
		ctx.F.updateInstances(dstInst, inst)
	default:
		// Nothing to update.
	}
	infer.Logger.Print(ctx.F.Sprintf(ValSymbol+"*%s store= %s/%s", dstPtr.Name(), source.Name(), ctx.F.locals[source]))
	return
}

func visitTypeAssert(instr *ssa.TypeAssert, infer *TypeInfer, ctx *Context) {
	if iface, ok := instr.AssertedType.(*types.Interface); ok {
		if meth, _ := types.MissingMethod(instr.X.Type(), iface, true); meth == nil { // No missing methods
			inst, ok := ctx.F.locals[instr.X]
			if !ok {
				infer.Logger.Fatalf("typeassert: %s: iface X %+v", ErrUnknownValue, instr.X.Name())
				return
			}
			if instr.CommaOk {
				ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
				ctx.F.commaok[ctx.F.locals[instr]] = &CommaOk{Instr: instr, Result: ctx.F.locals[instr]}
				ctx.F.tuples[ctx.F.locals[instr]] = make(Tuples, 2)
				infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = typeassert iface %s commaok", ctx.F.locals[instr], inst))
				return
			}
			ctx.F.locals[instr] = inst
			infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = typeassert iface %s", ctx.F.locals[instr], inst))
			return
		}
		infer.Logger.Fatalf("typeassert: %s: %+v", ErrMethodNotFound, instr)
		return
	}
	inst, ok := ctx.F.locals[instr.X]
	if !ok {
		infer.Logger.Fatalf("typeassert: %s: assert from %+v", ErrUnknownValue, instr.X)
		return
	}
	if instr.CommaOk {
		ctx.F.locals[instr] = &Value{instr, ctx.F.InstanceID(), ctx.L.Index}
		ctx.F.commaok[ctx.F.locals[instr]] = &CommaOk{Instr: instr, Result: ctx.F.locals[instr]}
		ctx.F.tuples[ctx.F.locals[instr]] = make(Tuples, 2)
		infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = typeassert %s commaok", ctx.F.locals[instr], inst))
		return
	}
	ctx.F.locals[instr] = inst
	infer.Logger.Print(ctx.F.Sprintf(SkipSymbol+"%s = typeassert %s", ctx.F.locals[instr], ctx.F.locals[instr.X]))
	return
	//infer.Logger.Fatalf("typeassert: %s: %+v", ErrIncompatType, instr)
}
