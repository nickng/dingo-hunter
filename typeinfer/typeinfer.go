// Package typeinfer provides session type inference from Go code.
//
package typeinfer // import "github.com/nickng/dingo-hunter/typeinfer"

import (
	"go/types"
	"io"
	"log"
	"time"

	"github.com/nickng/dingo-hunter/ssabuilder"
	"github.com/nickng/dingo-hunter/typeinfer/migo"
	"golang.org/x/tools/go/ssa"
)

// TypeInfer contains the metadata for a type inference.
type TypeInfer struct {
	SSA    *ssabuilder.SSAInfo // SSA IR of program.
	Env    *Program            // Analysed program.
	GQueue []*Function         // Goroutines to be analysed.

	Time   time.Duration
	Logger *log.Logger
	Done   chan struct{}
	Error  chan error
}

// New creates a new session type infer analysis.
func New(ssainfo *ssabuilder.SSAInfo, inferlog io.Writer) (*TypeInfer, error) {
	infer := &TypeInfer{
		SSA:    ssainfo,
		Logger: log.New(inferlog, "typeinfer: ", ssainfo.BuildConf.LogFlags),

		Done:  make(chan struct{}),
		Error: make(chan error, 1),
	}

	return infer, nil
}

// Run executes the analysis.
func (infer *TypeInfer) Run() {
	infer.Logger.Println("---- Start Analysis ----")
	// Initialise session.
	infer.Env = NewProgram(infer)
	infer.Env.MigoProg = migo.NewProgram()

	startTime := time.Now()
	mainPkg := ssabuilder.GetMainPkg(infer.SSA.Prog)
	if mainPkg == nil {
		infer.Error <- ErrNoMainPkg
	}
	defer close(infer.Done)

	initFn := mainPkg.Func("init")
	mainFn := mainPkg.Func("main")

	ctx := NewMainFunction(infer.Env, mainFn)
	// TODO(nickng): inline initialisation of var declarations
	for _, pkg := range infer.SSA.Prog.AllPackages() {
		for _, memb := range pkg.Members {
			switch val := memb.(type) {
			case *ssa.Global:
				switch t := derefAllType(val.Type()).Underlying().(type) {
				case *types.Array:
					ctx.Prog.globals[val] = &Instance{Value: val}
					ctx.Prog.arrays[ctx.Prog.globals[val]] = make(Elems, t.Len())
				case *types.Slice:
					ctx.Prog.globals[val] = &Instance{Value: val}
					ctx.Prog.arrays[ctx.Prog.globals[val]] = make(Elems, 0)
				case *types.Struct:
					ctx.Prog.globals[val] = &Instance{Value: val}
					ctx.Prog.structs[ctx.Prog.globals[val]] = make(Fields, t.NumFields())
				default:
					ctx.Prog.globals[val] = &Instance{Value: val}
				}
			}
		}
	}
	visitFunc(initFn, infer, ctx)
	visitFunc(mainFn, infer, ctx)

	infer.RunQueue()
	infer.Time = time.Now().Sub(startTime)
}

// RunQueue executes the analysis on spawned (queued) goroutines.
func (infer *TypeInfer) RunQueue() {
	for _, ctx := range infer.GQueue {
		infer.Logger.Printf("----- Goroutine %s -----", ctx.Fn.String())
		visitFunc(ctx.Fn, infer, ctx)
	}
}
