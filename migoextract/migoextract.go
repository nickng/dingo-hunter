// Package migoextract provides session type inference from Go code.
//
package migoextract // import "github.com/nickng/dingo-hunter/migoextract"

import (
	"go/types"
	"io"
	"log"
	"time"

	"github.com/nickng/dingo-hunter/ssabuilder"
	"github.com/nickng/migo"
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
		Logger: log.New(inferlog, "migoextract: ", ssainfo.BuildConf.LogFlags),

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
			switch value := memb.(type) {
			case *ssa.Global:
				ctx.Prog.globals[value] = &Value{Value: value}
				switch t := derefAllType(value.Type()).Underlying().(type) {
				case *types.Array:
					ctx.Prog.arrays[ctx.Prog.globals[value]] = make(Elems, t.Len())
				case *types.Slice:
					ctx.Prog.arrays[ctx.Prog.globals[value]] = make(Elems, 0)
				case *types.Struct:
					ctx.Prog.structs[ctx.Prog.globals[value]] = make(Fields, t.NumFields())
				default:
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
