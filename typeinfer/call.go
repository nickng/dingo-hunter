package typeinfer

// Functions for handling function call-like instructions
// i.e. builtin, call, closure, defer, go.

import (
	"go/types"

	"github.com/nickng/dingo-hunter/typeinfer/migo"
	"golang.org/x/tools/go/ssa"
)

// Call performs call on a given unprepared call context.
func (caller *Function) Call(call *ssa.Call, infer *TypeInfer, b *Block, l *Loop) {
	if call == nil {
		infer.Logger.Fatal("Call is nil")
		return
	}
	common := call.Common()
	switch fn := common.Value.(type) {
	case *ssa.Builtin:
		switch fn.Name() {
		case "close":
			ch, ok := caller.locals[common.Args[0]]
			if !ok {
				infer.Logger.Fatalf("call close: %s: %s", common.Args[0].Name(), ErrUnknownValue)
				return
			}
			if paramName, ok := caller.revlookup[ch.String()]; ok {
				caller.FuncDef.AddStmts(&migo.CloseStatement{Chan: paramName})
			} else {
				caller.FuncDef.AddStmts(&migo.CloseStatement{Chan: ch.String()})
			}
			infer.Logger.Print(caller.Sprintf("close %s", common.Args[0]))
			return
		case "len":
			if l.State == Enter {
				len, err := caller.callLen(common, infer)
				if err == ErrRuntimeLen {
					l.Bound = Dynamic
					return
				}
				l.Bound, l.End = Static, len
				return
			}
			caller.locals[call] = &Value{call, caller.InstanceID(), l.Index}
			infer.Logger.Printf(caller.Sprintf("  builtin.%s", common.String()))
		default:
			infer.Logger.Printf(caller.Sprintf("  builtin.%s", common.String()))
		}
	case *ssa.MakeClosure:
		infer.Logger.Printf(caller.Sprintf(SkipSymbol+" make closure %s", fn.String()))
		caller.callClosure(common, fn, infer, b, l)
	case *ssa.Function:
		if common.StaticCallee() == nil {
			infer.Logger.Fatal("Call with nil CallCommon")
		}
		callee := caller.callFn(common, infer, b, l)
		if callee != nil {
			caller.storeRetvals(infer, call.Value(), callee)
		}
	default:
		if !common.IsInvoke() {
			infer.Logger.Print("Unknown call type", common.String(), common.Description())
			return
		}
		callee := caller.invoke(common, infer, b, l)
		if callee != nil {
			caller.storeRetvals(infer, call.Value(), callee)
		} else {
			// Mock out the return values.
			switch common.Signature().Results().Len() {
			case 0:
			case 1:
				caller.locals[call.Value()] = &External{
					parent: caller.Fn,
					typ:    call.Value().Type().Underlying(),
				}
			case 2:
				caller.locals[call.Value()] = &External{typ: call.Value().Type().Underlying()}
				caller.tuples[caller.locals[call.Value()]] = make(Tuples, common.Signature().Results().Len())
			}
		}
	}
}

// Go handles Go statements.
func (caller *Function) Go(instr *ssa.Go, infer *TypeInfer) {
	common := instr.Common()
	callee := caller.prepareCallFn(common, common.StaticCallee(), nil)
	spawnStmt := &migo.SpawnStatement{Name: callee.Fn.String(), Params: []*migo.Parameter{}}
	for i, c := range common.Args {
		if _, ok := c.Type().(*types.Chan); ok {
			ch := getChan(c, infer)
			spawnStmt.AddParams(&migo.Parameter{Caller: ch, Callee: callee.Fn.Params[i]})
		}
	}
	if inst, ok := caller.locals[common.Value]; ok {
		if bindings, ok := caller.Prog.closures[inst]; ok {
			for _, b := range bindings {
				if v, ok := b.(*Value); ok {
					if _, ok := derefType(v.Type()).(*types.Chan); ok {
						spawnStmt.AddParams(&migo.Parameter{Caller: v, Callee: v})
					}
				}
			}
		}
	}
	caller.FuncDef.AddStmts(spawnStmt)
	caller.FuncDef.HasComm = true
	// Don't actually call/visit the function but enqueue it.
	infer.GQueue = append(infer.GQueue, callee)
}

// callLen computes the length of a given data structure (if statically known).
func (caller *Function) callLen(common *ssa.CallCommon, infer *TypeInfer) (int64, error) {
	arg0 := common.Args[0]
	switch t := arg0.Type().(type) {
	case *types.Array:
		infer.Logger.Printf(caller.Sprintf("  len(%s %s) = %d", arg0.Name(), arg0.Type(), t.Len()))
		return t.Len(), nil
	default:
		// String = runtime length of string
		// Map    = runtime size of map
		// Slice  = runtime size of slice
		// Chan   = elements in queue
		infer.Logger.Printf(caller.Sprintf("  len(%s %s) = ?", arg0.Name(), arg0.Type()))
	}
	return 0, ErrRuntimeLen
}

// storeRetvals takes retval (SSA value from caller storing return value(s)) and
// stores the return value of function (callee).
func (caller *Function) storeRetvals(infer *TypeInfer, retval ssa.Value, callee *Function) {
	if !callee.HasBody() {
		switch callee.Fn.Signature.Results().Len() {
		case 0:
			// Nothing.
		case 1:
			// Creating external instance because return value may be used.
			caller.locals[retval] = &External{caller.Fn, retval.Type().Underlying(), caller.InstanceID()}
			infer.Logger.Print(caller.Sprintf(ExitSymbol + "external"))
		default:
			caller.locals[retval] = &External{caller.Fn, retval.Type().Underlying(), caller.InstanceID()}
			caller.tuples[caller.locals[retval]] = make(Tuples, callee.Fn.Signature.Results().Len())
			infer.Logger.Print(caller.Sprintf(ExitSymbol+"external len=%d", callee.Fn.Signature.Results().Len()))
		}
		return
	}
	switch len(callee.retvals) {
	case 0:
		// Nothing.
	case 1:
		// XXX Pick the last return value from the exit paths
		//     This assumes idiomatic Go for error path to return early
		//     https://golang.org/doc/effective_go.html#if
		caller.locals[retval] = callee.retvals[len(callee.retvals)-1]
		if a, ok := callee.arrays[caller.locals[retval]]; ok {
			caller.arrays[caller.locals[retval]] = a
		}
		if s, ok := callee.structs[caller.locals[retval]]; ok {
			caller.structs[caller.locals[retval]] = s
		}
		if m, ok := callee.maps[caller.locals[retval]]; ok {
			caller.maps[caller.locals[retval]] = m
		}
		if a, ok := callee.Prog.arrays[caller.locals[retval]]; ok {
			caller.arrays[caller.locals[retval]] = a
		}
		if s, ok := callee.Prog.structs[caller.locals[retval]]; ok {
			caller.structs[caller.locals[retval]] = s
		}
		switch inst := caller.locals[retval].(type) {
		case *Value:
			infer.Logger.Print(caller.Sprintf(ExitSymbol+"[1] %s", inst))
			return
		case *External:
			infer.Logger.Print(caller.Sprintf(ExitSymbol+"[1] (ext) %s", inst))
			return
		case *Const:
			infer.Logger.Print(caller.Sprintf(ExitSymbol+"[1] constant %s", inst))
			return
		default:
			infer.Logger.Fatalf("return[1]: %s: not an instance %+v", ErrUnknownValue, retval)
		}
	default:
		caller.locals[retval] = &Value{retval, caller.InstanceID(), int64(0)}
		if callee.Fn.Signature.Results().Len() == 1 {
			caller.locals[retval] = callee.retvals[len(callee.retvals)-1]
			if a, ok := callee.arrays[caller.locals[retval]]; ok {
				caller.arrays[caller.locals[retval]] = a
			}
			if s, ok := callee.structs[caller.locals[retval]]; ok {
				caller.structs[caller.locals[retval]] = s
			}
			if m, ok := callee.maps[caller.locals[retval]]; ok {
				caller.maps[caller.locals[retval]] = m
			}
			if a, ok := callee.Prog.arrays[caller.locals[retval]]; ok {
				caller.arrays[caller.locals[retval]] = a
			}
			if s, ok := callee.Prog.structs[caller.locals[retval]]; ok {
				caller.structs[caller.locals[retval]] = s
			}
		} else {
			caller.tuples[caller.locals[retval]] = make(Tuples, callee.Fn.Signature.Results().Len())
			for i := range callee.retvals {
				tupleIdx := i % callee.Fn.Signature.Results().Len()
				if callee.retvals[i] != nil {
					caller.tuples[caller.locals[retval]][tupleIdx] = callee.retvals[i]
				}
				if a, ok := callee.arrays[callee.retvals[i]]; ok {
					caller.arrays[callee.retvals[i]] = a
				}
				if s, ok := callee.structs[callee.retvals[i]]; ok {
					caller.structs[callee.retvals[i]] = s
				}
				if m, ok := callee.maps[callee.retvals[i]]; ok {
					caller.maps[callee.retvals[i]] = m
				}
				if a, ok := callee.Prog.arrays[callee.retvals[i]]; ok {
					caller.arrays[callee.retvals[i]] = a
				}
				if s, ok := callee.Prog.structs[callee.retvals[i]]; ok {
					caller.structs[callee.retvals[i]] = s
				}
			}
		}
		// XXX Pick the return values from the last exit path
		//     This assumes idiomatic Go for error path to return early
		//     https://golang.org/doc/effective_go.html#if
		infer.Logger.Print(caller.Sprintf(ExitSymbol+"[%d/%d] %v", callee.Fn.Signature.Results().Len(), len(callee.retvals), caller.tuples[caller.locals[retval]]))
	}
}

// IsRecursiveCall checks if current function context is a recursive call and marks
// the context recursive (with pointer to the original context).
func (caller *Function) IsRecursiveCall() bool {
	for parentCtx := caller.Caller; parentCtx.Caller != nil; parentCtx = parentCtx.Caller {
		if caller.Fn == parentCtx.Fn { // is identical function?
			return true
		}
	}
	return false
}

func (caller *Function) invoke(common *ssa.CallCommon, infer *TypeInfer, b *Block, l *Loop) *Function {
	iface, ok := common.Value.Type().Underlying().(*types.Interface)
	if !ok {
		infer.Logger.Fatalf("invoke: %s is not an interface", common.String())
		return nil
	}
	ifaceInst, ok := caller.locals[common.Value] // SSA value initialised
	if !ok {
		infer.Logger.Fatalf("invoke: %s: %s", common.Value.Name(), ErrUnknownValue)
		return nil
	}
	switch inst := ifaceInst.(type) {
	case *Value: // OK
	case *Const:
		if inst.Const.IsNil() {
			return nil
		}
		infer.Logger.Fatalf("invoke: %+v is not nil nor concrete", ifaceInst)
	case *External:
		infer.Logger.Printf(caller.Sprintf("invoke: %+v external", ifaceInst))
		return nil
	default:
		infer.Logger.Printf(caller.Sprintf("invoke: %+v unknown", ifaceInst))
		return nil
	}
	meth, _ := types.MissingMethod(ifaceInst.(*Value).Type(), iface, true) // static
	if meth != nil {
		meth, _ = types.MissingMethod(ifaceInst.(*Value).Type(), iface, false) // non-static
		if meth != nil {
			infer.Logger.Printf("invoke: missing method %s: %s", meth.String(), ErrIfaceIncomplete)
			return nil
		}
	}
	fn := findMethod(common.Value.Parent().Prog, common.Method, ifaceInst.(*Value).Type(), infer)
	if fn == nil {
		if meth == nil {
			infer.Logger.Printf("invoke: cannot locate concrete method")
		} else {
			infer.Logger.Printf("invoke: cannot locate concrete method: %s", meth.String())
		}
		return nil
	}
	return caller.call(common, fn, common.Value, infer, b, l)
}

func (caller *Function) callFn(common *ssa.CallCommon, infer *TypeInfer, b *Block, l *Loop) *Function {
	return caller.call(common, common.StaticCallee(), nil, infer, b, l)
}

func (caller *Function) call(common *ssa.CallCommon, fn *ssa.Function, rcvr ssa.Value, infer *TypeInfer, b *Block, l *Loop) *Function {
	callee := caller.prepareCallFn(common, fn, rcvr)
	if callee.IsRecursiveCall() {
		return callee
	}
	visitFunc(callee.Fn, infer, callee)
	if callee.HasBody() {
		callStmt := &migo.CallStatement{Name: callee.Fn.String(), Params: []*migo.Parameter{}}
		for i, c := range common.Args {
			if _, ok := c.Type().(*types.Chan); ok {
				ch := getChan(c, infer)
				callStmt.AddParams(&migo.Parameter{Caller: ch, Callee: callee.Fn.Params[i]})
			}
		}
		if inst, ok := caller.locals[common.Value]; ok {
			if bindings, ok := caller.Prog.closures[inst]; ok {
				for _, b := range bindings {
					if v, ok := b.(*Value); ok {
						if _, ok := derefType(v.Type()).(*types.Chan); ok {
							callStmt.AddParams(&migo.Parameter{Caller: v, Callee: v})
						}
					}
				}
			}
		}
		caller.FuncDef.AddStmts(callStmt)
	}
	return callee
}

func (caller *Function) callClosure(common *ssa.CallCommon, closure *ssa.MakeClosure, infer *TypeInfer, b *Block, l *Loop) {
	callee := caller.prepareCallFn(common, closure.Fn.(*ssa.Function), nil)
	for _, b := range closure.Bindings {
		if inst, ok := caller.locals[b]; ok {
			callee.locals[b] = inst
		}
	}
	callee.call(common, common.StaticCallee(), nil, infer, b, l)
}

func findMethod(prog *ssa.Program, meth *types.Func, typ types.Type, infer *TypeInfer) *ssa.Function {
	if meth != nil {
		return prog.LookupMethod(typ, meth.Pkg(), meth.Name())
	}
	infer.Logger.Fatal(ErrMethodNotFound)
	return nil
}
