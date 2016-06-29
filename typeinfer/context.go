package typeinfer

// Context captures variables (and invariants) of scopes during execution.
// Different contexts are used for different level of fine-grainedness.

import (
	"bytes"
	"fmt"
	"log"

	"github.com/nickng/dingo-hunter/typeinfer/migo"
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
	FuncInstance map[*ssa.Function]int // Count number of function instances.
	InitPkgs     map[*ssa.Package]bool // Initialised packages.
	Infer        *TypeInfer            // Reference to inference.
	MigoProg     *migo.Program         // Core calculus of program.

	arrays   map[VarInstance]Elems     // Array elements.
	closures map[VarInstance]Captures  // Closures.
	globals  map[ssa.Value]VarInstance // Global variables.
	structs  map[VarInstance]Fields    // Heap allocated struct fields.
}

// NewProgram creates a program for a type inference.
func NewProgram(infer *TypeInfer) *Program {
	return &Program{
		FuncInstance: make(map[*ssa.Function]int),
		InitPkgs:     make(map[*ssa.Package]bool),
		Infer:        infer,
		arrays:       make(map[VarInstance]Elems),
		closures:     make(map[VarInstance]Captures),
		globals:      make(map[ssa.Value]VarInstance),
		structs:      make(map[VarInstance]Fields),
	}
}

// Function captures the function environment.
//
// Function environment stores local variable instances (as reference), return
// values, if-then-else parent, select-condition.
type Function struct {
	Fn      *ssa.Function           // Function callee (this).
	Caller  *Function               // Function caller (parent).
	Prog    *Program                // Program environment (global).
	Visited map[*ssa.BasicBlock]int // Visited block tracking.
	Level   int                     // Call level (for indentation).
	FuncDef *migo.Function          // Function definition.

	id        int                                         // Instance identifier.
	hasBody   bool                                        // True if function has body.
	arrays    map[VarInstance]Elems                       // Array elements.
	commaok   map[VarInstance]*CommaOk                    // CommaOK statements.
	defers    []*ssa.Defer                                // Deferred calls.
	locals    map[ssa.Value]VarInstance                   // Local variable instances.
	maps      map[VarInstance]map[VarInstance]VarInstance // Map instances (just an approximate).
	retvals   []VarInstance                               // Return value instances.
	selects   map[VarInstance]*Select                     // Select cases mapping.
	structs   map[VarInstance]Fields                      // Stack allocated struct fields.
	tuples    map[VarInstance]Tuples                      // Tuples.
	loopstack *LoopStack                                  // Stack of Loop.
}

// NewMainFunction returns a new main() call context.
func NewMainFunction(prog *Program, mainFn *ssa.Function) *Function {
	return &Function{
		Fn:      mainFn,
		Prog:    prog,
		Visited: make(map[*ssa.BasicBlock]int),
		FuncDef: migo.NewFunction("main.main"),

		arrays:    make(map[VarInstance]Elems),
		commaok:   make(map[VarInstance]*CommaOk),
		defers:    []*ssa.Defer{},
		locals:    make(map[ssa.Value]VarInstance),
		maps:      make(map[VarInstance]map[VarInstance]VarInstance),
		retvals:   []VarInstance{},
		selects:   make(map[VarInstance]*Select),
		structs:   make(map[VarInstance]Fields),
		tuples:    make(map[VarInstance]Tuples),
		loopstack: NewLoopStack(),
	}
}

// NewFunction returns a new function call context, and takes the caller's
// context as parameter.
func NewFunction(caller *Function) *Function {
	return &Function{
		Caller:  caller,
		Prog:    caller.Prog,
		Visited: make(map[*ssa.BasicBlock]int),
		FuncDef: migo.NewFunction("__uninitialised__"),
		Level:   caller.Level + 1,

		arrays:    make(map[VarInstance]Elems),
		commaok:   make(map[VarInstance]*CommaOk),
		defers:    []*ssa.Defer{},
		locals:    make(map[ssa.Value]VarInstance),
		maps:      make(map[VarInstance]map[VarInstance]VarInstance),
		retvals:   []VarInstance{},
		selects:   make(map[VarInstance]*Select),
		structs:   make(map[VarInstance]Fields),
		tuples:    make(map[VarInstance]Tuples),
		loopstack: NewLoopStack(),
	}
}

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
	callee.FuncDef.Name = fmt.Sprintf("%s_%d", fn.String(), callee.Prog.FuncInstance[callee.Fn])
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
		if inst, ok := caller.locals[argCaller]; ok {
			callee.locals[param] = inst

			// Copy array and struct from parent.
			if elems, ok := caller.arrays[inst]; ok {
				callee.arrays[inst] = elems
			}
			if fields, ok := caller.structs[inst]; ok {
				callee.structs[inst] = fields
			}
		} else if c, ok := argCaller.(*ssa.Const); ok {
			callee.locals[param] = &ConstInstance{c}
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

func (caller *Function) updateInstances(old, new VarInstance) {
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
}

// Block captures information about SSA block.
type Block struct {
	Function *Function      // Parent function context.
	MigoDef  *migo.Function // MiGo Function for the block.
	Pred     int            // Immediate predecessor trace.
	Index    int            // Current block index.
}

func NewBlock(parent *Function, block *ssa.BasicBlock, curr int) *Block {
	return &Block{
		Function: parent,
		MigoDef:  migo.NewFunction(fmt.Sprintf("%s#%d", parent.Fn.String(), block.Index)),
		Pred:     curr,
		Index:    block.Index,
	}
}

// Loop captures information about loop.
//
// A Loop context exists within a function, inside the scope of a for loop.
// Nested loops should be captured externally.
type Loop struct {
	Parent *Function // Enclosing function.
	Bound  LoopBound // Loop bound type.
	State  LoopState // Loop/Body/Done.

	IndexVar  ssa.Value // Variable holding the index (phi).
	CondVar   ssa.Value // Variable holding the cond expression.
	Index     int64     // Current index value.
	Start     int64     // Lower bound of index.
	Step      int64     // Increment (can be negative).
	End       int64     // Upper bound of index.
	LoopBlock int       // Block number of loop (with for.loop label).
}

// SetInit sets the loop index initial value (int).
func (l *Loop) SetInit(index ssa.Value, init int64) {
	l.IndexVar = index
	l.Start = init
	l.Index = init
}

// SetStep sets the loop index step value (int).
func (l *Loop) SetStep(step int64) {
	l.Step = step
}

// SetCond sets the loop exit condition (int).
func (l *Loop) SetCond(cond ssa.Value, max int64) {
	l.CondVar = cond
	l.End = max
}

// Next performs an index increment (e.g. i++) if possible.
func (l *Loop) Next() {
	if l.Bound == Static {
		l.Index += l.Step
	}
}

// Cond returns true if the loop should continue.
func (l *Loop) HasNext() bool {
	if l.Bound == Static {
		return l.Start <= l.Index && l.Index <= l.End
	}
	return false
}

func (l *Loop) String() string {
	if l.Bound != Unknown && l.State != NonLoop {
		return fmt.Sprintf("%s: bound %s [%d..%d..%d] Step:%d", l.State, l.Bound, l.Start, l.Index, l.End, l.Step)
	}
	return fmt.Sprintf("%s: bound %s", l.State, l.Bound)
}
