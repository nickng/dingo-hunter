package main

// Types and Functions related to handling function calls.
//
// The main issue here is to keep track of the variables in the environment
// for the analysis (especially which ones are channels).

import (
	"fmt"
	"os"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types"

	"github.com/nickng/dingo-hunter/sesstype"
)

type ArrayElems map[ssa.Value]ssa.Value // Array elements.
type StructFields map[int]ssa.Value     // Struct fields.
type ElemDecomp struct {
	arr ssa.Value
	idx ssa.Value
}
type FieldDecomp struct {
	str ssa.Value
	idx int
}

// Call frame.
type frame struct {
	fn      *ssa.Function              // Function ptr of callee.
	locals  map[ssa.Value]ssa.Value    // Local variables to origin.
	arrays  map[ssa.Value]ArrayElems   // Arrays.
	structs map[ssa.Value]StructFields // Structs.
	fields  map[ssa.Value]FieldDecomp  // Field decomposition of structs (always in scope).
	elems   map[ssa.Value]ElemDecomp   // Elem decomposition of arrays (always in scope).
	retvals []ssa.Value                // Return values.
	defers  []*ssa.Defer               // Defers.
	caller  *frame                     // ptr to parent's frame, nil if main/ext.
	env     *environ                   // Environment.
	gortn   *goroutine                 // Current goroutine.
}

// Environment.
// Variables/information available globally for all goroutines.
type environ struct {
	session  *sesstype.Session            // For lookup channels and roles.
	globals  map[ssa.Value]ssa.Value      // Globals.
	arrays   map[ssa.Value]ArrayElems     // Arrays in heap.
	structs  map[ssa.Value]StructFields   // Structs in heap.
	extern   map[ssa.Value]types.Type     // Values that originates externally, we are only sure of its type.
	tuples   map[ssa.Value][]ssa.Value    // Maps return value to multi-value tuples.
	closures map[ssa.Value][]ssa.Value    // Closure captures.
	selNode  map[ssa.Value]*sesstype.Node // Parent nodes of select.
	selIdx   map[ssa.Value]ssa.Value      // Mapping from select index to select SSA Value.
	selTest  map[ssa.Value]struct {       // Records test for select-branch index.
		idx int       // The index of the branch.
		tpl ssa.Value // The SelectState tuple which the branch originates from.
	}
	ifparent *sesstype.NodeStack
}

// Locate an array in either stack or heap.
func (fr *frame) getArray(v ssa.Value) (array ArrayElems, isHeap bool, found bool) {
	if arrHeap, ok := fr.env.arrays[v]; ok {
		array = arrHeap
		isHeap = true
		found = true
	} else if arrStack, ok := fr.arrays[v]; ok {
		array = arrStack
		isHeap = false
		found = true
	} else {
		array = nil
		isHeap = false
		found = false
	}
	return
}

// Locate a struct in either stack or heap.
func (fr *frame) getStruct(v ssa.Value) (struc StructFields, isHeap bool, found bool) {
	if strHeap, ok := fr.env.structs[v]; ok {
		struc = strHeap
		isHeap = true
		found = true
	} else if strStack, ok := fr.structs[v]; ok {
		struc = strStack
		isHeap = false
		found = true
	} else {
		isHeap = true
		found = false
	}
	return
}

func (fr *frame) getFields(f ssa.Value) (fieldInfo FieldDecomp, found bool) {
	if fieldInfo_, ok := fr.fields[f]; ok {
		fieldInfo = fieldInfo_
		found = true
		return
	} else if fr.caller != nil {
		if fieldInfo_, ok := fr.caller.getFields(fr.get(f)); ok {
			fieldInfo = fieldInfo_
			found = true
			return
		}
	}
	found = false
	return
}

// Goroutine analysis info.
// Metadata that follows the analysis of a specific goroutine.
type goroutine struct {
	role    sesstype.Role // Role of current goroutine
	root    sesstype.Node
	leaf    *sesstype.Node
	visited map[*ssa.BasicBlock]sesstype.Node
	chans   map[ssa.Value]sesstype.Chan // Keeps track of defined channels.
	parent  *goroutine
}

func (fr *frame) printCallStack() {
	curFr := fr
	for curFr != nil && curFr.fn != nil {
		fmt.Fprintf(os.Stderr, "Called by: %s()\n", curFr.fn.String())
		curFr = curFr.caller
	}
}

// Find the origin of ssa.Value
func (fr *frame) get(v ssa.Value) ssa.Value {
	//defer func() { fmt.Fprintf(os.Stderr, "\tGET %s = %s (%s)\n", reg(v), v.String(), fr.fn.String()) }()
	if v == nil {
		panic(fmt.Sprintf("get: cannot deal with nil ssa.Value at %s\n", loc(fr.fn.Pkg.Prog.Fset, v.Pos())))
	}

	if prev, ok := fr.env.globals[v]; ok {
		return prev
	}

	if prev, ok := fr.locals[v]; ok {
		if v == prev {
			panic(fmt.Sprintf("get: invalid - local[%s] points to itself\n", reg(v)))
		}
		if fr.caller == nil {
			return prev // This is the result
		}
		return fr.get(prev) // Keep searching parent
	} else if fr.caller != nil {
		return fr.caller.get(v)
	}

	return v
}

// Append a session type node to current goroutine.
func (gortn *goroutine) AddNode(node sesstype.Node) {
	if gortn.leaf == nil {
		panic("AddNode: leaf cannot be nil")
	}

	newLeaf := (*gortn.leaf).Append(node)
	gortn.leaf = &newLeaf
}

func (fr *frame) findChan(ch ssa.Value) (sesstype.Chan, sesstype.Role) {
	g := fr.gortn
	for g != nil {
		if c, ok := g.chans[ch]; ok {
			return c, g.role
		}
		g = g.parent
	}
	return nil, fr.gortn.role
}

func call(c *ssa.Call, caller *frame) {
	callcommon(c, c.Common(), caller)
}

// call Converts a caller-perspective frame into a callee frame.
func callcommon(c *ssa.Call, common *ssa.CallCommon, caller *frame) {
	switch fn := common.Value.(type) {
	case *ssa.Builtin:
		if fn.Name() == "close" {
			if len(common.Args) == 1 {
				if ch, _ := caller.findChan(caller.get(common.Args[0])); ch != nil {
					fmt.Fprintf(os.Stderr, "++ call builtin %s("+green("%s")+" channel %s)\n", orange(fn.Name()), common.Args[0].Name(), ch.Name())
					visitClose(ch, caller)
				} else {
					panic("Builtin close() called with non-channel\n")
				}
			} else {
				panic("Builtin close() called with wrong number of parameters\n")
			}
		} else if fn.Name() == "copy" {
			dst := common.Args[0]
			src := common.Args[1]
			fmt.Fprintf(os.Stderr, "++ call builtin %s(%s <- %s)\n", orange("copy"), dst.Name(), src.Name())
			caller.locals[dst] = src
			return
		} else {
			// TODO(nickng) Handle builtin functions.
			fmt.Fprintf(os.Stderr, "++ call builtin %s(", fn.Name())
			// Do parameter translation
			for i, arg := range common.Args {
				fmt.Fprintf(os.Stderr, "#%d: %s", i, arg.Name())
			}
			fmt.Fprintf(os.Stderr, ") # TODO (handle builtin)\n")
		}

	case *ssa.MakeClosure:
		// TODO(nickng) Handle calling closure
		fmt.Fprintf(os.Stderr, "   # TODO (handle closure) %s\n", fn.String())

	case *ssa.Function:
		if common.StaticCallee() == nil {
			panic("Call with nil CallCommon!")
		}

		callee := &frame{
			fn:      common.StaticCallee(),
			locals:  make(map[ssa.Value]ssa.Value),
			arrays:  make(map[ssa.Value]ArrayElems),
			structs: make(map[ssa.Value]StructFields),
			elems:   make(map[ssa.Value]ElemDecomp),
			fields:  make(map[ssa.Value]FieldDecomp),
			defers:  make([]*ssa.Defer, 0),
			retvals: make([]ssa.Value, 0),
			caller:  caller,
			env:     caller.env,   // Use the same env as caller (i.e. ptr)
			gortn:   caller.gortn, // Use the same role as caller
		}

		fmt.Fprintf(os.Stderr, "++ call %s(", orange(common.StaticCallee().String()))
		translateParams(callee, common)
		fmt.Fprintf(os.Stderr, ")\n")

		if !isRecursiveCall(callee) {
			if hasCode := visitFunc(callee.fn, callee); hasCode {
				handleRetvals(c, caller, callee)
			} else {
				handleExtRetvals(c, caller, callee)
			}
			fmt.Fprintf(os.Stderr, "-- return from %s\n", orange(common.StaticCallee().String()))
		} else {
			fmt.Fprintf(os.Stderr, "-- Recursive %s()\n", orange(common.StaticCallee().String()))
			callee.printCallStack()
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown call type %v\n", c)
	}
}

func callgo(g *ssa.Go, caller *frame) {
	// XXX A unique name for the goroutine invocation using position in code
	common := g.Common()
	goname := fmt.Sprintf("%s_%d", common.Value.Name(), int(g.Pos()))
	gorole := caller.env.session.GetRole(goname)

	callee := &frame{
		fn:      common.StaticCallee(),
		locals:  make(map[ssa.Value]ssa.Value),
		arrays:  make(map[ssa.Value]ArrayElems),
		structs: make(map[ssa.Value]StructFields),
		elems:   make(map[ssa.Value]ElemDecomp),
		fields:  make(map[ssa.Value]FieldDecomp),
		retvals: make([]ssa.Value, 0),
		caller:  caller,
		env:     caller.env,
		gortn: &goroutine{
			role:    gorole,
			root:    sesstype.MkLabelNode(goname),
			leaf:    nil,
			visited: make(map[*ssa.BasicBlock]sesstype.Node),
			chans:   make(map[ssa.Value]sesstype.Chan),
			parent:  caller.gortn,
		},
	}
	callee.gortn.leaf = &callee.gortn.root

	fmt.Fprintf(os.Stderr, "@@ queue go %s(", common.StaticCallee().String())
	translateParams(callee, common)
	fmt.Fprintf(os.Stderr, ")\n")

	// TODO(nickng) Does not stop at recursive call.
	goQueue = append(goQueue, callee)
}

// handleRetvals looks up and stores return value from function calls.
// Nothing will be done if there are no return values from the function.
func handleRetvals(call *ssa.Call, caller, callee *frame) {
	if len(callee.retvals) > 0 {
		if len(callee.retvals) == 1 {
			// Single return value (callee.retvals[0])
			caller.locals[call.Value()] = callee.retvals[0]
		} else {
			// Multiple return values (callee.retvals tuple)
			caller.env.tuples[call.Value()] = callee.retvals
		}
	}
}

// handleExtRetvals looks up and stores return value from (ext) function calls.
// Ext functions have no code (no body to analyse) and unlike normal values,
// the return values/tuples are stored until they are referenced.
func handleExtRetvals(call *ssa.Call, caller, callee *frame) {
	// Since there are no code for the function, we use the function
	// signature to see if any of these are channels.
	// XXX We don't know where these come from so we put them in extern.
	resultsLen := callee.fn.Signature.Results().Len()
	if resultsLen > 0 {
		caller.env.extern[call.Value()] = callee.fn.Signature.Results()
		if resultsLen == 1 {
			fmt.Fprintf(os.Stderr, "-- Return from %s (builtin/ext) with a single value\n", callee.fn.String())
			if _, ok := callee.fn.Signature.Results().At(0).Type().(*types.Chan); ok {
				ch := caller.env.session.MakeExtChan(call, caller.gortn.role)
				caller.gortn.chans[caller.get(call.Value())] = ch
				fmt.Fprintf(os.Stderr, "-- Return value from %s (builtin/ext) is a channel %s (ext)\n", callee.fn.String(), caller.gortn.chans[call.Value()].Name())
			}
		} else {
			fmt.Fprintf(os.Stderr, "-- Return from %s (builtin/ext) with %d-tuple\n", callee.fn.String(), resultsLen)
		}
	}
}

func translateParams(callee *frame, common *ssa.CallCommon) {
	// Do parameter translation
	for i, param := range common.StaticCallee().Params {
		argParent := common.Args[i]
		if param != argParent {
			callee.locals[param] = argParent
		}

		if i > 0 {
			fmt.Fprintf(os.Stderr, ", ")
		}

		if ch, ok := callee.gortn.chans[callee.get(param)]; ok {
			fmt.Fprintf(os.Stderr, "%s channel %s", green(param.Name()), ch.Name())
		} else {
			fmt.Fprintf(os.Stderr, "%s = caller[%s]", orange(param.Name()), reg(callee.caller.get(common.Args[i])))
		}

		// If argument is a field-access temp, copy them to this frame too.
		if fieldInfo, ok := callee.caller.fields[argParent]; ok {
			callee.fields[param] = fieldInfo
		}

		// if argument is a elem-access temp, copy them to this frame too.
		if elemInfo, ok := callee.caller.elems[argParent]; ok {
			callee.elems[param] = elemInfo
		}
	}

	// Closure capture (copy from env.closures assigned in MakeClosure).
	if captures, isClosure := callee.env.closures[common.Value]; isClosure {
		for idx, fv := range callee.fn.FreeVars {
			callee.locals[fv] = captures[idx]
			fmt.Fprintf(os.Stderr, ", capture %s = %s", fv.Name(), reg(captures[idx]))
		}
	}
}

func isRecursiveCall(callee *frame) bool {
	tracebackFns := make([]*ssa.Function, 0)
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
