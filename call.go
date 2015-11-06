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

// Call frame.
type frame struct {
	fn     *ssa.Function           // Function ptr of callee
	locals map[ssa.Value]ssa.Value // Maps of local vars to SSA Values
	fields map[ssa.Value]struct {  // Records decomposition of SSA structs
		idx int
		str ssa.Value
	}
	elems map[ssa.Value]struct { // Records decomposition of SSA arrays
		idx ssa.Value
		arr ssa.Value
	}
	retvals []ssa.Value // Return values
	caller  *frame      // ptr to parent's frame, nil if main/ext
	env     *environ    // Environment
	gortn   *goroutine  // Detail of current goroutine
}

// Environment.
// Variables/information available globally for all goroutines.
type environ struct {
	session  *sesstype.Session                     // For lookup channels and roles.
	calls    map[*ssa.Call]bool                    // For detecting recursive calls.
	chans    map[ssa.Value]sesstype.Chan           // Checks currently defined channels.
	globals  map[ssa.Value]ssa.Value               // Maps of global vars to SSA Values
	structs  map[ssa.Value][]ssa.Value             // Maps of SSA values to structs.
	arrays   map[ssa.Value]map[ssa.Value]ssa.Value // Maps of SSA values to arrays.
	extern   map[ssa.Value]types.Type              // Values that originates externally, we are only sure of its type.
	tuples   map[ssa.Value][]ssa.Value             // Maps return value to multi-value tuples.
	closures map[ssa.Value][]ssa.Value             // Closure captures.
	selNode  map[ssa.Value]sesstype.Node           // Parent nodes of select.
	selIdx   map[ssa.Value]ssa.Value               // Mapping from select index to select SSA Value.
	selTest  map[ssa.Value]struct {                // Records test for select-branch index.
		idx int       // The index of the branch.
		tpl ssa.Value // The SelectState tuple which the branch originates from.
	}
}

// Goroutine analysis info.
// Metadata that follows the analysis of a specific goroutine.
type goroutine struct {
	role    sesstype.Role // Role of current goroutine
	root    sesstype.Node
	leaf    sesstype.Node
	visited map[*ssa.BasicBlock]sesstype.Node
}

// Find origin of ssa.Value
func (fr *frame) get(v ssa.Value) ssa.Value {
	if v == nil {
		panic("Cannot traceback nil ssa.Value!")
	}
	if prev, ok := fr.locals[v]; ok {
		if prev == nil || prev == v {
			return v
		}
		return fr.get(prev)
	} else if prev, ok := fr.env.globals[v]; ok {
		if prev == nil {
			return v
		}
		return fr.get(prev)
	}
	return fr.getCaller(v)
}

func (fr *frame) getCaller(v ssa.Value) ssa.Value {
	if fr.caller != nil {
		if prev, ok := fr.caller.locals[v]; ok {
			if prev == nil {
				return v
			}
			return fr.caller.get(prev)
		}
	}
	return v
}

// Append a session type node to current goroutine.
func (gortn *goroutine) AddNode(node sesstype.Node) {
	if gortn.leaf == nil {
		if gortn.root == nil {
			gortn.root = node
		}
		gortn.leaf = node
	} else {
		gortn.leaf.Append(node)
	}
}

// call Converts a caller-perspective frame into a callee frame.
func call(c *ssa.Call, caller *frame) {
	common := c.Common()
	switch fn := common.Value.(type) {
	case *ssa.Builtin:
		if fn.Name() == "close" {
			if len(common.Args) == 1 {
				if ch, ok := caller.env.chans[caller.get(common.Args[0])]; ok {
					fmt.Fprintf(os.Stderr, "   ++call builtin close("+green("%s")+" channel %s)\n", common.Args[0].Name(), ch.Name())
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
			fmt.Fprintf(os.Stderr, "   ++call builtin copy(%s <- %s)\n", dst.Name(), src.Name())
			caller.locals[dst] = src
			return
		} else {
			// TODO(nickng) Handle builtin functions.
			fmt.Fprintf(os.Stderr, "   # TODO (handle builtin) %s\n", fn.String())
			fmt.Fprintf(os.Stderr, "   ++call builtin %s(", fn.Name())

			// Do parameter translation
			for i, arg := range common.Args {
				fmt.Fprintf(os.Stderr, "\n    #%d: %s", i, arg.Name())
			}
			fmt.Fprintf(os.Stderr, ")\n")
		}

	case *ssa.MakeClosure:
		// TODO(nickng) Handle calling closure
		fmt.Fprintf(os.Stderr, "   # TODO (handle closure) %s\n", fn.String())

	case *ssa.Function:
		if common.StaticCallee() == nil {
			panic("Call with nil CallCommon!")
		}

		callee := &frame{
			fn:     common.StaticCallee(),
			locals: make(map[ssa.Value]ssa.Value),
			fields: make(map[ssa.Value]struct {
				idx int
				str ssa.Value
			}),
			elems: make(map[ssa.Value]struct {
				idx ssa.Value
				arr ssa.Value
			}),
			retvals: make([]ssa.Value, 0),
			caller:  caller,
			env:     caller.env,   // Use the same env as caller (i.e. ptr)
			gortn:   caller.gortn, // Use the same role as caller
		}

		fmt.Fprintf(os.Stderr, "   ++call %s(", common.StaticCallee().Name())
		translateParams(callee, common)
		fmt.Fprintf(os.Stderr, ")\n")

		if hasCode := visitFunc(callee.fn, callee); hasCode {
			handleRetvals(c, caller, callee)
		} else {
			handleExtRetvals(c, caller, callee)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown call type %v\n", c)
	}
}

func callgo(g *ssa.Go, caller *frame) {
	// XXX A unique name for the goroutine invocation using position in code
	common := g.Common()
	gorole := caller.env.session.GetRole(fmt.Sprintf("%s_%d", common.Value.Name(), int(g.Pos())))

	callee := &frame{
		fn:     common.StaticCallee(),
		locals: make(map[ssa.Value]ssa.Value),
		fields: make(map[ssa.Value]struct {
			idx int
			str ssa.Value
		}),
		elems: make(map[ssa.Value]struct {
			idx ssa.Value
			arr ssa.Value
		}),
		retvals: make([]ssa.Value, 0),
		caller:  caller,
		env:     caller.env,
		gortn: &goroutine{
			role:    gorole,
			root:    nil,
			leaf:    nil,
			visited: make(map[*ssa.BasicBlock]sesstype.Node),
		},
	}

	fmt.Fprintf(os.Stderr, "   ++ queue go %s(", common.StaticCallee().String())
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
			fmt.Fprintf(os.Stderr, "  -- Return from %s with a single value %s\n", callee.fn.String(), callee.retvals[0].Name())
			caller.locals[call.Value()] = callee.retvals[0]
		} else {
			fmt.Fprintf(os.Stderr, "  -- Return from %s with %d-tuple\n", callee.fn.String(), len(callee.retvals))
			caller.env.tuples[call.Value()] = callee.retvals
			for _, retval := range callee.retvals {
				fmt.Fprintf(os.Stderr, "    - %s\n", retval.String())
			}
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
			fmt.Fprintf(os.Stderr, "  -- Return from %s (builtin/ext) with a single value\n", callee.fn.String())
			if t, ok := callee.fn.Signature.Results().At(0).Type().(*types.Chan); ok {
				ch := caller.env.session.MakeExtChan(call.Name(), caller.gortn.role, t.Elem())
				caller.env.chans[call.Value()] = ch
				fmt.Fprintf(os.Stderr, "  -- Return value from %s (builtin/ext) is a channel %s (ext)\n", callee.fn.String(), caller.env.chans[call.Value()].Name())
			}
		} else {
			fmt.Fprintf(os.Stderr, "  -- Return from %s (builtin/ext) with %d-tuple\n", callee.fn.String(), resultsLen)
			// Do not assign new channels here, only when accessed.
		}
	}
}

func translateParams(callee *frame, common *ssa.CallCommon) {
	// Do parameter translation
	for i, param := range common.StaticCallee().Params {
		callee.locals[param] = common.Args[i]

		if ch, ok := callee.env.chans[callee.get(common.Args[i])]; ok {
			fmt.Fprintf(os.Stderr, "\n    #%d: "+green("%s")+" channel %s", i, param.Name(), ch.Name())
		} else {
			fmt.Fprintf(os.Stderr, "\n    #%d: %s = caller[%s]", i, param.Name(), common.Args[i].Name())
		}
	}

	// Do closure capture translation
	if captures, isClosure := callee.env.closures[common.Value]; isClosure {
		for idx, fv := range callee.fn.FreeVars {
			callee.locals[fv] = captures[idx]
			fmt.Fprintf(os.Stderr, "\n    capture %s = %s", fv.Name(), captures[idx].Name())
		}
	}
}
