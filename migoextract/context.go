package migoextract

// Context captures variables (and invariants) of scopes during execution.
// Different contexts are used for different level of fine-grainedness.

import (
	"bytes"
	"fmt"
	"go/types"
	"log"

	"github.com/nickng/migo/v3"
	"golang.org/x/tools/go/ssa"
)

// LoopState indicate the state (in analysis) of loop currently in.
type LoopState int

//go:generate stringer -type=LoopState
const (
	NonLoop LoopState = iota
	Enter             // Loop initialisation and condition checking
	Body              // Loop body (repeat)
	Exit              // Loop exit
)

// LoopBound indicates if a loop is bounded or not.
type LoopBound int

//go:generate stringer -type=LoopBound
const (
	Unknown LoopBound = iota
	Static            // Static loop.
	Dynamic           // Dynamic loop.
)

// Program captures the program environment.
//
// A single inference has exactly one Program, and it contains all global
// data (and metadata) in the program.
type Program struct {
	FuncInstance map[*ssa.Function]int  // Count number of function instances.
	InitPkgs     map[*ssa.Package]bool  // Initialised packages.
	Infer        *TypeInfer             // Reference to inference.
	MigoProg     *migo.Program          // Core calculus of program.
	closures     map[Instance]Captures  // Closures.
	globals      map[ssa.Value]Instance // Global variables.
	*Storage                            // Storage.
}

// NewProgram creates a program for a type inference.
func NewProgram(infer *TypeInfer) *Program {
	return &Program{
		FuncInstance: make(map[*ssa.Function]int),
		InitPkgs:     make(map[*ssa.Package]bool),
		Infer:        infer,
		closures:     make(map[Instance]Captures),
		globals:      make(map[ssa.Value]Instance),
		Storage:      NewStorage(),
	}
}

// Function captures the function environment.
//
// Function environment stores local variable instances (as reference), return
// values, if-then-else parent, select-condition.
type Function struct {
	Fn          *ssa.Function           // Function callee (this).
	Caller      *Function               // Function caller (parent).
	Prog        *Program                // Program environment (global).
	Visited     map[*ssa.BasicBlock]int // Visited block tracking.
	Level       int                     // Call level (for indentation).
	FuncDef     *migo.Function          // Function definition.
	ChildBlocks map[int]*Block          // Map from index -> child SSA blocks.

	id        int                    // Instance identifier.
	hasBody   bool                   // True if function has body.
	commaok   map[Instance]*CommaOk  // CommaOK statements.
	defers    []*ssa.Defer           // Deferred calls.
	locals    map[ssa.Value]Instance // Local variable instances.
	revlookup map[string]string      // Reverse lookup names.
	extraargs []ssa.Value
	retvals   []Instance           // Return value instances.
	selects   map[Instance]*Select // Select cases mapping.
	tuples    map[Instance]Tuples  // Tuples.
	loopstack *LoopStack           // Stack of Loop.
	*Storage                       // Storage.
}

// NewMainFunction returns a new main() call context.
func NewMainFunction(prog *Program, mainFn *ssa.Function) *Function {
	return &Function{
		Fn:          mainFn,
		Prog:        prog,
		Visited:     make(map[*ssa.BasicBlock]int),
		FuncDef:     migo.NewFunction("main.main"),
		ChildBlocks: make(map[int]*Block),

		commaok:   make(map[Instance]*CommaOk),
		defers:    []*ssa.Defer{},
		locals:    make(map[ssa.Value]Instance),
		retvals:   []Instance{},
		extraargs: []ssa.Value{},
		revlookup: make(map[string]string),
		selects:   make(map[Instance]*Select),
		tuples:    make(map[Instance]Tuples),
		loopstack: NewLoopStack(),
		Storage:   NewStorage(),
	}
}

// NewFunction returns a new function call context, and takes the caller's
// context as parameter.
func NewFunction(caller *Function) *Function {
	return &Function{
		Caller:      caller,
		Prog:        caller.Prog,
		Visited:     make(map[*ssa.BasicBlock]int),
		FuncDef:     migo.NewFunction("__uninitialised__"),
		Level:       caller.Level + 1,
		ChildBlocks: make(map[int]*Block),

		commaok:   make(map[Instance]*CommaOk),
		defers:    []*ssa.Defer{},
		locals:    make(map[ssa.Value]Instance),
		revlookup: make(map[string]string),
		extraargs: []ssa.Value{},
		retvals:   []Instance{},
		selects:   make(map[Instance]*Select),
		tuples:    make(map[Instance]Tuples),
		loopstack: NewLoopStack(),
		Storage:   NewStorage(),
	}
}

// HasBody returns true if Function is user-defined or has source code and
// built in SSA program.
func (caller *Function) HasBody() bool { return caller.hasBody }

// prepareCallFn prepares a caller Function to visit performing necessary context switching and returns a new callee Function.
// rcvr is non-nil if invoke call
func (caller *Function) prepareCallFn(common *ssa.CallCommon, fn *ssa.Function, rcvr ssa.Value) *Function {
	callee := NewFunction(caller)
	callee.Fn = fn
	// This function was called before
	if _, ok := callee.Prog.FuncInstance[callee.Fn]; ok {
		callee.Prog.FuncInstance[callee.Fn]++
	} else {
		callee.Prog.FuncInstance[callee.Fn] = 0
	}
	callee.FuncDef.Name = fn.String()
	callee.id = callee.Prog.FuncInstance[callee.Fn]
	for i, param := range callee.Fn.Params {
		var argCaller ssa.Value
		if rcvr != nil {
			if i == 0 {
				argCaller = rcvr
			} else {
				argCaller = common.Args[i-1]
			}
		} else {
			argCaller = common.Args[i]
		}
		if _, ok := argCaller.Type().(*types.Chan); ok {
			callee.FuncDef.AddParams(&migo.Parameter{Caller: argCaller, Callee: param})
		}
		if inst, ok := caller.locals[argCaller]; ok {
			callee.locals[param] = inst
			callee.revlookup[argCaller.Name()] = param.Name()

			// Copy array and struct from parent.
			if elems, ok := caller.arrays[inst]; ok {
				callee.arrays[inst] = elems
			}
			if fields, ok := caller.structs[inst]; ok {
				callee.structs[inst] = fields
			}
			if maps, ok := caller.maps[inst]; ok {
				callee.maps[inst] = maps
			}
		} else if c, ok := argCaller.(*ssa.Const); ok {
			callee.locals[param] = &Const{c}
		}
	}

	if inst, ok := caller.locals[common.Value]; ok {
		if cap, ok := caller.Prog.closures[inst]; ok {
			for i, fv := range callee.Fn.FreeVars {
				callee.locals[fv] = cap[i]
				if _, ok := derefType(fv.Type()).(*types.Chan); ok {
					callee.FuncDef.AddParams(&migo.Parameter{Caller: fv, Callee: fv})
				}
			}
		}
	}
	return callee
}

// InstanceID returns the current function instance number (numbers of times
// function called).
func (caller *Function) InstanceID() int {
	if caller.id < 0 {
		log.Fatal(ErrUnitialisedFunc)
	}
	return caller.id
}

func (caller *Function) String() string {
	var buf bytes.Buffer
	buf.WriteString("--- Context ---\n")
	if caller.Fn == nil {
		log.Fatal(ErrUnitialisedFunc)
	}
	buf.WriteString(fmt.Sprintf("\t- Fn:\t%s_%d\n", caller.Fn, caller.id))
	if caller.Caller != nil {
		buf.WriteString(fmt.Sprintf("\t- Parent:\t%s\n", caller.Caller.Fn.String()))
	} else {
		buf.WriteString("\t- Parent: main.main\n")
	}
	for val, instance := range caller.locals {
		buf.WriteString(fmt.Sprintf("\t\t- %s = %s\n", val.Name(), instance))
	}
	buf.WriteString(fmt.Sprintf("\t- Retvals: %d\n", len(caller.retvals)))
	return buf.String()
}

func (caller *Function) updateInstances(old, new Instance) {
	for inst, array := range caller.arrays {
		for k, v := range array {
			if v == old {
				caller.arrays[inst][k] = new
			}
		}
	}
	for inst, array := range caller.Prog.arrays {
		for k, v := range array {
			if v == old {
				caller.Prog.arrays[inst][k] = new
			}
		}
	}
	for inst, struc := range caller.structs {
		for i, field := range struc {
			if field == old {
				caller.structs[inst][i] = new
			}
		}
	}
	for inst, struc := range caller.Prog.structs {
		for i, field := range struc {
			if field == old {
				caller.Prog.structs[inst][i] = new
			}
		}
	}
	for inst, mmap := range caller.maps {
		for k, v := range mmap {
			if v == old {
				caller.maps[inst][k] = new
			}
		}
	}
}

// Block captures information about SSA block.
type Block struct {
	Function *Function      // Parent function context.
	MigoDef  *migo.Function // MiGo Function for the block.
	Pred     int            // Immediate predecessor trace.
	Index    int            // Current block index.
}

// NewBlock creates a new block enclosed by the given function.
func NewBlock(parent *Function, block *ssa.BasicBlock, curr int) *Block {
	blockFn := fmt.Sprintf("%s#%d", parent.Fn.String(), block.Index)
	parent.ChildBlocks[block.Index] = &Block{
		Function: parent,
		MigoDef:  migo.NewFunction(blockFn),
		Pred:     curr,
		Index:    block.Index,
	}
	return parent.ChildBlocks[block.Index]
}

// Context is a grouping of different levels of context.
type Context struct {
	F *Function // Function context.
	B *Block    // Block context.
	L *Loop     // Loop context.
}
