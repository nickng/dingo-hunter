package main

import (
	"fmt"
	"os"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types"

	"github.com/nickng/dingo-hunter/sesstype"
	"github.com/nickng/dingo-hunter/utils"
)

// VarKind specifies the type of utils.VarDef used in frame.local[]
type VarKind int

// VarKind definitions
const (
	Nothing     VarKind = iota // Not in frame.local[]
	Array                      // Array in heap
	Chan                       // Channel
	Struct                     // Struct in heap
	LocalArray                 // Array in frame
	LocalStruct                // Struct in frame
	Untracked                  // Not a tracked VarKind but in frame.local[]
)

// Captures are lists of VarDefs for closure captures
type Captures []*utils.VarDef

// Tuples are lists of VarDefs from multiple return values
type Tuples []*utils.VarDef

// Elems are maps from array indices (variable) to VarDefs
type Elems map[ssa.Value]*utils.VarDef

// Fields are maps from struct fields (integer) to VarDefs
type Fields map[int]*utils.VarDef

// Frame holds variables in current function scope
type frame struct {
	fn      *ssa.Function               // Function ptr of callee
	locals  map[ssa.Value]*utils.VarDef // Holds definitions of local registers
	arrays  map[*utils.VarDef]Elems     // Array elements (Alloc local)
	structs map[*utils.VarDef]Fields    // Struct fields (Alloc local)
	tuples  map[ssa.Value]Tuples        // Multiple return values as tuple
	phi     map[ssa.Value][]ssa.Value   // Phis
	retvals Tuples                      //
	defers  []*ssa.Defer                // Deferred calls
	caller  *frame                      // Ptr to caller's frame, nil if main/ext
	env     *environ                    // Environment
	gortn   *goroutine                  // Current goroutine
}

// Environment: Variables/info available globally for all goroutines
type environ struct {
	session  *sesstype.Session
	globals  map[ssa.Value]*utils.VarDef      // Globals
	arrays   map[*utils.VarDef]Elems          // Array elements
	structs  map[*utils.VarDef]Fields         // Struct fields
	chans    map[*utils.VarDef]*sesstype.Chan // Channels
	extern   map[ssa.Value]types.Type         // Values that originates externally, we are only sure of its type
	closures map[ssa.Value]Captures           // Closure captures
	selNode  map[ssa.Value]*sesstype.Node     // Parent nodes of select
	selIdx   map[ssa.Value]ssa.Value          // Mapping from select index to select SSA Value
	selTest  map[ssa.Value]struct {           // Records test for select-branch index
		idx int       // The index of the branch
		tpl ssa.Value // The SelectState tuple which the branch originates from
	}
	ifparent *sesstype.NodeStack
}

func (env *environ) GetSessionChan(vd *utils.VarDef) *sesstype.Chan {
	if ch, ok := env.session.Chans[vd]; ok {
		return &ch
	}
	panic(fmt.Sprintf("Channel %s undefined in session", vd.String()))
}

func makeToplevelFrame() *frame {
	callee := &frame{
		fn:      nil,
		locals:  make(map[ssa.Value]*utils.VarDef),
		arrays:  make(map[*utils.VarDef]Elems),
		structs: make(map[*utils.VarDef]Fields),
		tuples:  make(map[ssa.Value]Tuples),
		phi:     make(map[ssa.Value][]ssa.Value),
		retvals: make(Tuples, 0),
		defers:  make([]*ssa.Defer, 0),
		caller:  nil,
		env: &environ{
			session:  session,
			globals:  make(map[ssa.Value]*utils.VarDef),
			arrays:   make(map[*utils.VarDef]Elems),
			structs:  make(map[*utils.VarDef]Fields),
			chans:    make(map[*utils.VarDef]*sesstype.Chan),
			extern:   make(map[ssa.Value]types.Type),
			closures: make(map[ssa.Value]Captures),
			selNode:  make(map[ssa.Value]*sesstype.Node),
			selIdx:   make(map[ssa.Value]ssa.Value),
			selTest: make(map[ssa.Value]struct {
				idx int
				tpl ssa.Value
			}),
			ifparent: sesstype.NewNodeStack(),
		},
		gortn: &goroutine{
			role:    session.GetRole("main"),
			root:    sesstype.MkLabelNode("main"),
			leaf:    nil,
			visited: make(map[*ssa.BasicBlock]sesstype.Node),
		},
	}
	callee.gortn.leaf = &callee.gortn.root

	return callee
}

func (caller *frame) callBuiltin(common *ssa.CallCommon) {
	builtin := common.Value.(*ssa.Builtin)
	if builtin.Name() == "close" {
		if len(common.Args) == 1 {
			if ch, ok := caller.env.chans[caller.locals[common.Args[0]]]; ok {
				fmt.Fprintf(os.Stderr, "++ call builtin %s(%s channel %s)\n", orange(builtin.Name()), green(common.Args[0].Name()), ch.Name())
				visitClose(*ch, caller)
			} else {
				panic("Builtin close() called with non-channel\n")
			}
		}
	} else if builtin.Name() == "copy" {
		dst := common.Args[0]
		src := common.Args[1]
		fmt.Fprintf(os.Stderr, "++ call builtin %s(%s <- %s)\n", orange("copy"), dst.Name(), src.Name())
		caller.locals[dst] = caller.locals[src]
		return
	} else {
		fmt.Fprintf(os.Stderr, "++ call builtin %s(", builtin.Name())
		for _, arg := range common.Args {
			fmt.Fprintf(os.Stderr, "%s", arg.Name())
		}
		fmt.Fprintf(os.Stderr, ") # TODO (handle builtin)\n")
	}
}

func (caller *frame) call(c *ssa.Call) {
	caller.callCommon(c, c.Common())
}

func (caller *frame) callCommon(call *ssa.Call, common *ssa.CallCommon) {
	switch fn := common.Value.(type) {
	case *ssa.Builtin:
		caller.callBuiltin(common)

	case *ssa.MakeClosure:
		// TODO(nickng) Handle calling closure
		fmt.Fprintf(os.Stderr, "   # TODO (handle closure) %s\n", fn.String())

	case *ssa.Function:
		if common.StaticCallee() == nil {
			panic("Call with nil CallCommon!")
		}

		callee := &frame{
			fn:      common.StaticCallee(),
			locals:  make(map[ssa.Value]*utils.VarDef),
			arrays:  make(map[*utils.VarDef]Elems),
			structs: make(map[*utils.VarDef]Fields),
			tuples:  make(map[ssa.Value]Tuples),
			phi:     make(map[ssa.Value][]ssa.Value),
			retvals: make(Tuples, common.Signature().Results().Len()),
			defers:  make([]*ssa.Defer, 0),
			caller:  caller,
			env:     caller.env,   // Use the same env as caller
			gortn:   caller.gortn, // Use the same role as caller
		}

		fmt.Fprintf(os.Stderr, "++ call %s(", orange(common.StaticCallee().String()))
		callee.translate(common)
		fmt.Fprintf(os.Stderr, ")\n")

		if callee.isRecursive() {
			fmt.Fprintf(os.Stderr, "-- Recursive %s()\n", orange(common.StaticCallee().String()))
			callee.printCallStack()
		} else {
			if hasCode := visitFunc(callee.fn, callee); hasCode {
				caller.handleRetvals(call.Value(), callee)
			} else {
				caller.handleExtRetvals(call.Value(), callee)
			}
			fmt.Fprintf(os.Stderr, "-- return from %s (%d retvals)\n", orange(common.StaticCallee().String()), len(callee.retvals))
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown call type %v\n", common)
	}
}

func (caller *frame) callGo(g *ssa.Go) {
	common := g.Common()
	goname := fmt.Sprintf("%s_%d", common.Value.Name(), int(g.Pos()))
	gorole := caller.env.session.GetRole(goname)

	callee := &frame{
		fn:      common.StaticCallee(),
		locals:  make(map[ssa.Value]*utils.VarDef),
		arrays:  make(map[*utils.VarDef]Elems),
		structs: make(map[*utils.VarDef]Fields),
		tuples:  make(map[ssa.Value]Tuples),
		phi:     make(map[ssa.Value][]ssa.Value),
		retvals: make(Tuples, common.Signature().Results().Len()),
		defers:  make([]*ssa.Defer, 0),
		caller:  caller,
		env:     caller.env, // Use the same env as caller
		gortn: &goroutine{
			role:    gorole,
			root:    sesstype.MkLabelNode(goname),
			leaf:    nil,
			visited: make(map[*ssa.BasicBlock]sesstype.Node),
		},
	}
	callee.gortn.leaf = &callee.gortn.root

	fmt.Fprintf(os.Stderr, "@@ queue go %s(", common.StaticCallee().String())
	callee.translate(common)
	fmt.Fprintf(os.Stderr, ")\n")

	// TODO(nickng) Does not stop at recursive call.
	goQueue = append(goQueue, callee)
}

func (callee *frame) translate(common *ssa.CallCommon) {
	for i, param := range common.StaticCallee().Params {
		argParent := common.Args[i]
		if param != argParent {
			if vd, ok := callee.caller.locals[argParent]; ok {
				callee.locals[param] = vd
			}
		}

		if i > 0 {
			fmt.Fprintf(os.Stderr, ", ")
		}

		fmt.Fprintf(os.Stderr, "%s:caller[%s] = %s", orange(param.Name()), reg(common.Args[i]), callee.locals[param].String())

		// if argument is a channel
		if ch, ok := callee.env.chans[callee.locals[param]]; ok {
			fmt.Fprintf(os.Stderr, "channel %s", (*ch).Name())
		} else if _, ok := callee.env.structs[callee.locals[param]]; ok {
			fmt.Fprintf(os.Stderr, "struct")
		} else if _, ok := callee.env.arrays[callee.locals[param]]; ok {
			fmt.Fprintf(os.Stderr, "array")
		}
	}

	// Closure capture (copy from env.closures assigned in MakeClosure).
	if captures, isClosure := callee.env.closures[common.Value]; isClosure {
		for idx, fv := range callee.fn.FreeVars {
			callee.locals[fv] = captures[idx]
			fmt.Fprintf(os.Stderr, ", capture %s = %s", fv.Name(), captures[idx].String())
		}
	}
}

// handleRetvals looks up and stores return value from function calls.
// Nothing will be done if there are no return values from the function.
func (caller *frame) handleRetvals(returned ssa.Value, callee *frame) {
	if len(callee.retvals) > 0 {
		if len(callee.retvals) == 1 {
			// Single return value (callee.retvals[0])
			caller.locals[returned] = callee.retvals[0]
		} else {
			// Multiple return values (callee.retvals tuple)
			caller.tuples[returned] = callee.retvals
		}
	}
}

func (callee *frame) get(v ssa.Value) (*utils.VarDef, VarKind) {
	if vd, ok := callee.locals[v]; ok {
		if _, ok := callee.env.arrays[vd]; ok {
			return vd, Array
		}
		if _, ok := callee.arrays[vd]; ok {
			return vd, LocalArray
		}
		if _, ok := callee.env.chans[vd]; ok {
			return vd, Chan
		}
		if _, ok := callee.env.structs[vd]; ok {
			return vd, Struct
		}
		if _, ok := callee.structs[vd]; ok {
			return vd, LocalStruct
		}
		return vd, Untracked
	} else if vs, ok := callee.phi[v]; ok {
		for i := len(vs) - 1; i >= 0; i-- {
			if chVd, defined := callee.locals[vs[i]]; defined {
				return chVd, Chan
			}
		}
	}
	return nil, Nothing
}

// handleExtRetvals looks up and stores return value from (ext) function calls.
// Ext functions have no code (no body to analyse) and unlike normal values,
// the return values/tuples are stored until they are referenced.
func (caller *frame) handleExtRetvals(returned ssa.Value, callee *frame) {
	// Since there are no code for the function, we use the function
	// signature to see if any of these are channels.
	// XXX We don't know where these come from so we put them in extern.
	resultsLen := callee.fn.Signature.Results().Len()
	if resultsLen > 0 {
		caller.env.extern[returned] = callee.fn.Signature.Results()
		if resultsLen == 1 {
			fmt.Fprintf(os.Stderr, "-- Return from %s (builtin/ext) with a single value\n", callee.fn.String())
			if _, ok := callee.fn.Signature.Results().At(0).Type().(*types.Chan); ok {
				vardef := utils.NewVarDef(returned)
				ch := caller.env.session.MakeExtChan(vardef, caller.gortn.role)
				caller.env.chans[vardef] = &ch
				fmt.Fprintf(os.Stderr, "-- Return value from %s (builtin/ext) is a channel %s (ext)\n", callee.fn.String(), (*caller.env.chans[vardef]).Name())
			}
		} else {
			fmt.Fprintf(os.Stderr, "-- Return from %s (builtin/ext) with %d-tuple\n", callee.fn.String(), resultsLen)
		}
	}
}

func (callee *frame) isRecursive() bool {
	var tracebackFns []*ssa.Function
	foundFr := callee
	for fr := callee.caller; fr != nil; fr = fr.caller {
		tracebackFns = append(tracebackFns, fr.fn)
		if fr.fn == callee.fn {
			foundFr = fr
			break
		}
	}
	// If same function is not found, not recursive
	if foundFr == callee {
		return false
	}

	// Otherwise try to trace back with foundFr and is recursive if all matches
	for _, fn := range tracebackFns {
		if foundFr == nil || foundFr.fn != fn {
			return false
		}
		foundFr = foundFr.caller
	}
	return true
}

func (callee *frame) printCallStack() {
	curFr := callee
	for curFr != nil && curFr.fn != nil {
		fmt.Fprintf(os.Stderr, "Called by: %s()\n", curFr.fn.String())
		curFr = curFr.caller
	}
}

func (callee *frame) updateDefs(vdOld, vdNew *utils.VarDef) {
	for def, array := range callee.arrays {
		for k, v := range array {
			if v == vdOld {
				callee.arrays[def][k] = vdNew
			}
		}
	}
	for def, array := range callee.env.arrays {
		for k, v := range array {
			if v == vdOld {
				callee.env.arrays[def][k] = vdNew
			}
		}
	}
	for def, struc := range callee.structs {
		for i, field := range struc {
			if field == vdOld {
				callee.structs[def][i] = vdNew
			}
		}
	}
	for def, struc := range callee.env.structs {
		for i, field := range struc {
			if field == vdOld {
				callee.env.structs[def][i] = vdNew
			}
		}
	}
}
