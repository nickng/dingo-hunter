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
	fn      *ssa.Function           // Function ptr of callee
	locals  map[ssa.Value]ssa.Value // Maps of local vars to SSA Values
	retvals []ssa.Value             // Return values
	caller  *frame                  // ptr to parent's frame, nil if main/ext
	env     *environ                // Environment
	gortn   *goroutine              // Detail of current goroutine
}

// Environment.
// Variables/information available globally for all goroutines.
type environ struct {
	session  *sesstype.Session           // For lookup channels and roles.
	chans    map[ssa.Value]sesstype.Chan // Checks currently defined channels.
	extern   map[ssa.Value]types.Type    // Values that originates externally, we are only sure of its type.
	tuples   map[ssa.Value][]ssa.Value   // Maps return value to multi-value tuples.
	globals  map[string]ssa.Value        // Maps of global varnames to SSA Values.
	closures map[ssa.Value][]ssa.Value   // Closure captures.
	selNode  map[ssa.Value]sesstype.Node // Parent nodes of select.
	selIdx   map[ssa.Value]ssa.Value     // Mapping from select index to select SSA Value.
	selTest  map[ssa.Value]struct {      // Records test for select-branch index.
		idx int       // The index of the branch.
		tpl ssa.Value // The SelectState tuple which the branch originates from.
	}
}

// Goroutine analysis info.
// Metadata that follows the analysis of a specific goroutine.
type goroutine struct {
	role    sesstype.Role // Role of current goroutine
	begin   sesstype.Node
	end     sesstype.Node
	visited map[*ssa.BasicBlock]sesstype.Node
}

// Find origin of ssa.Value
func (fr *frame) get(v ssa.Value) ssa.Value {
	if prev, ok := fr.locals[v]; ok {
		return fr.get(prev)
	}
	return fr.getEnv(v)
}

func (fr *frame) getEnv(v ssa.Value) ssa.Value {
	if fr.caller != nil {
		if prev, ok := fr.caller.locals[v]; ok {
			return fr.caller.get(prev)
		}
	}
	return v
}

// Append a session type node to current goroutine.
func (gortn *goroutine) append(node sesstype.Node) {
	if gortn.begin == nil {
		gortn.begin = node
		gortn.visited = make(map[*ssa.BasicBlock]sesstype.Node)
	}
	if gortn.end == nil {
		gortn.end = node
	} else {
		gortn.end = gortn.end.Append(node)
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
					fmt.Fprintf(os.Stderr, " ++ builtin close("+green("%s")+" channel %s)\n", common.Args[0].Name(), ch.Name())
					visitClose(ch, caller)
				} else {
					panic("Builtin close() called with non-channel\n")
				}
			} else {
				panic("Builtin close() called with wrong number of parameters\n")
			}
		} else {
			// TODO(nickng) Handle builtin functions.
			fmt.Fprintf(os.Stderr, " # TODO (handle builtin) %s\n", fn.String())
			fmt.Fprintf(os.Stderr, "  ++ builtin %s(", fn.Name())

			// Do parameter translation
			for i, arg := range common.Args {
				fmt.Fprintf(os.Stderr, "\n    #%d: %s", i, arg.Name())
			}
			fmt.Fprintf(os.Stderr, ")\n")
		}

	case *ssa.MakeClosure:
		// TODO(nickng) Handle calling closure
		fmt.Fprintf(os.Stderr, " # TODO (handle closure) %s\n", fn.String())

	case *ssa.Function:
		if fn.Signature.Recv() != nil {
			// TODO(nickng) Handle method call with receiver
			fmt.Fprintf(os.Stderr, " # TODO (handle method call) (%s) %s\n", fn.Signature.Recv().String(), fn.String())
		}
		if common.StaticCallee() == nil {
			panic("Call with nil CallCommon!")
		}

		callee := &frame{
			fn:      common.StaticCallee(),
			locals:  make(map[ssa.Value]ssa.Value),
			retvals: make([]ssa.Value, 0),
			caller:  caller,
			env:     caller.env,   // Use the same env as caller (i.e. ptr)
			gortn:   caller.gortn, // Use the same role as caller
		}

		fmt.Fprintf(os.Stderr, "  ++ call %s(", common.StaticCallee().Name())
		translateParams(callee, common)
		fmt.Fprintf(os.Stderr, ")\n")

		if hasCode := visitFunc(callee.fn, callee); hasCode {
			handleRetvals(c, caller, callee)
		} else {
			handleExtRetvals(c, caller, callee)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown call type \n")
	}
}

func callgo(g *ssa.Go, caller *frame) {
	// XXX A unique name for the goroutine invocation using position in code
	common := g.Common()
	gorole := caller.env.session.GetRole(fmt.Sprintf("%s_%d", common.Value.Name(), int(g.Pos())))

	callee := &frame{
		fn:      common.StaticCallee(),
		locals:  make(map[ssa.Value]ssa.Value),
		retvals: make([]ssa.Value, 0),
		caller:  caller,
		env:     caller.env,
		gortn: &goroutine{
			role:    gorole,
			begin:   nil,
			end:     nil,
			visited: make(map[*ssa.BasicBlock]sesstype.Node),
		},
	}

	fmt.Fprintf(os.Stderr, "  ++ queue go %s(", common.StaticCallee().String())
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
			fmt.Fprintf(os.Stderr, "\n    #%d: %s", i, param.Name())
		}
	}

	// Do closure capture translation
	if captures, isClosure := callee.env.closures[common.Value]; isClosure {
		for idx, fv := range callee.fn.FreeVars {
			callee.locals[fv] = captures[idx]
			fmt.Fprintf(os.Stderr, "\n    capture %s = %s\n", fv.Name(), captures[idx].Name())
		}
	}
}
