package main

// Types and Functions related to handling function calls.
//
// The main issue here is to keep track of the variables in the environment
// for the analysis (especially which ones are channels).

import (
	"fmt"
	"go/token"
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
	session *sesstype.Session           // For lookup channels and roles.
	chans   map[ssa.Value]sesstype.Chan // Checks currently defined channels.
	extern  map[ssa.Value]types.Type    // Values that originates externally, we are only sure of its type.
	tuples  map[ssa.Value][]ssa.Value   // Maps return value to multi-value tuples
	globals map[string]ssa.Value        // Maps of global varnames to SSA Values.
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
func call(c ssa.CallCommon, caller *frame) *frame {
	switch fn := c.Value.(type) {
	case *ssa.Builtin:
		// TODO(nickng) Handle builtin functions.
		fmt.Fprintf(os.Stderr, " # TODO (handle builtin) %s\n", fn.String())
	case *ssa.MakeClosure:
		// TODO(nickng) Handle calling closure
		fmt.Fprintf(os.Stderr, " # TODO (handle closure) %s\n", fn.String())
	case *ssa.Function:
		if fn.Signature.Recv() != nil {
			// TODO(nickng) Handle method call with receiver
			fmt.Fprintf(os.Stderr, " # TODO (handle method call) (%s) %s\n", fn.Signature.Recv().String(), fn.String())
		}
	}
	if c.StaticCallee() == nil {
		panic("Call with nil CallCommon!")
	}

	fr := &frame{
		fn:      c.StaticCallee(),
		locals:  make(map[ssa.Value]ssa.Value),
		retvals: make([]ssa.Value, 0),
		caller:  caller,
		env:     caller.env,   // Use the same env as caller (i.e. ptr)
		gortn:   caller.gortn, // Use the same role as caller
	}

	fmt.Fprintf(os.Stderr, "  ++ call %s(", c.StaticCallee().String())
	// Do parameter translation
	for i, param := range c.StaticCallee().Params {
		fr.locals[param] = c.Args[i]

		if ch, ok := fr.env.chans[fr.get(c.Args[i])]; ok {
			fmt.Fprintf(os.Stderr, "\n    #%d: "+green("%s")+" channel %s", i, param.Name(), ch.Name())
		} else {
			fmt.Fprintf(os.Stderr, "\n    #%d: %s", i, param.Name())
		}
	}
	fmt.Fprintf(os.Stderr, ")\n")

	return fr
}

func callgo(c ssa.CallCommon, caller *frame, pos token.Pos) {
	// XXX A unique name for the goroutine invocation using position in code
	gorole := caller.env.session.GetRole(fmt.Sprintf("%s_%d", c.Value.Name(), int(pos)))

	fr := &frame{
		fn:      c.StaticCallee(),
		locals:  make(map[ssa.Value]ssa.Value),
		retvals: make([]ssa.Value, 0),
		caller:  caller,
		env:     caller.env,
		gortn: &goroutine{
			role:  gorole,
			begin: nil,
			end:   nil,
		},
	}

	fmt.Fprintf(os.Stderr, "  ++ queue go %s(", c.StaticCallee().String())
	// Do parameter translation
	for i, param := range c.StaticCallee().Params {
		fr.locals[param] = c.Args[i]

		if ch, ok := fr.env.chans[fr.get(c.Args[i])]; ok {
			fmt.Fprintf(os.Stderr, "\n    #%d: "+green("%s")+" channel %s", i, param.Name(), ch.Name())
		} else {
			fmt.Fprintf(os.Stderr, "\n    #%d: %s", i, param.Name())
		}
	}
	fmt.Fprintf(os.Stderr, ")\n")

	// TODO(nickng) Does not stop at recursive call.
	goQueue = append(goQueue, fr)
}
