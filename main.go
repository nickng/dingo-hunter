// This tool analyses go code and extract its session typing from channel usages.
//
// The tool currently only works for executables as the analysis uses the main
// function as entry point.
package main

// This file contains only the functions needed to start the analysis
//  - Handle command line flags
//  - Set up session variables

import (
	"flag"
	"fmt"
	"go/build"
	"os"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"golang.org/x/tools/go/types"

	"github.com/nickng/dingo-hunter/sesstype"
)

var (
	session *sesstype.Session // Keeps track of the all session

	ssaflag = ssa.BuilderModeFlag(flag.CommandLine, "ssabuild", 0)
	goQueue = make([]*frame, 0)
)

const usage = "Usage dingo-hunter <main.go> ...\n"

// main function analyses the program in four steps
//
// (1) Load program as SSA
// (2) Analyse main.main()
// (3) Analyse goroutines found in (2)
// (4) Output results
func main() {
	var prog *ssa.Program
	var err error

	prog, err = loadSSA()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading files: %s\n", err)
	}

	mainPkg := findMainPkg(prog)
	if mainPkg == nil {
		fmt.Fprintf(os.Stderr, "Error: 'main' package not found\n")
		os.Exit(1)
	}

	session = sesstype.CreateSession() // init needs Session to determine
	initFunc := mainPkg.Func("init")
	mainFrm := newFrame(initFunc)
	mainFrm.gortn.role = session.GetRole("main")
	if initFunc != nil {
		visitFunc(initFunc, mainFrm)
	}

	mainFunc := mainPkg.Func("main")
	if mainFunc == nil {
		fmt.Fprintf(os.Stderr, "Error: 'main()' function not found in 'main' package\n")
		os.Exit(1)
	}

	visitFunc(mainFunc, mainFrm)
	session.SetType(mainFrm.gortn.role, mainFrm.gortn.root)

	var goFrm *frame
	for len(goQueue) > 0 {
		goFrm, goQueue = goQueue[0], goQueue[1:]
		fmt.Fprintf(os.Stderr, "\n%s\n\n", goFrm.fn.Name())
		visitFunc(goFrm.fn, goFrm)
		goFrm.env.session.SetType(goFrm.gortn.role, goFrm.gortn.root)
	}

	fmt.Printf(" ----- Results ----- \n %s\n", session.String())

	sesstype.GenDot(session)
}

// Load command line arguments as SSA program for analysis
func loadSSA() (*ssa.Program, error) {
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	var conf = loader.Config{Build: &build.Default}

	// Use the initial packages from the command line.
	if _, err := conf.FromArgs(args /*test?*/, false); err != nil {
		return nil, err
	}

	// Load, parse and type-check the whole program.
	prog, err := conf.Load()
	if err != nil {
		return nil, err
	}
	progSSA := ssautil.CreateProgram(prog, *ssaflag) // If ssabuild specified

	// Build and display only the initial packages (and synthetic wrappers),
	// unless -run is specified.
	//
	// Adapted from golang.org/x/tools/go/ssa
	for _, info := range prog.InitialPackages() {
		progSSA.Package(info.Pkg).Build()
	}

	// Don't load these packages.
	for _, info := range prog.AllPackages {
		if info.Pkg.Name() != "fmt" {
			progSSA.Package(info.Pkg).Build()
		}
	}

	return progSSA, nil
}

func findMainPkg(prog *ssa.Program) *ssa.Package {
	pkgs := prog.AllPackages()
	for _, pkg := range pkgs {
		if pkg.Pkg.Name() == "main" {
			return pkg
		}
	}

	return nil
}

// Create a new frame from toplevel function
func newFrame(fn *ssa.Function) *frame {
	return &frame{
		fn:     fn,
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
		caller:  nil,
		env: &environ{
			session:  session,
			calls:    make(map[*ssa.Call]bool),
			chans:    make(map[ssa.Value]sesstype.Chan),
			globals:  make(map[ssa.Value]ssa.Value),
			structs:  make(map[ssa.Value][]ssa.Value),
			arrays:   make(map[ssa.Value]map[ssa.Value]ssa.Value),
			extern:   make(map[ssa.Value]types.Type),
			tuples:   make(map[ssa.Value][]ssa.Value),
			closures: make(map[ssa.Value][]ssa.Value),
			selNode:  make(map[ssa.Value]sesstype.Node),
			selIdx:   make(map[ssa.Value]ssa.Value),
			selTest: make(map[ssa.Value]struct {
				idx int
				tpl ssa.Value
			}),
		},
		gortn: &goroutine{
			role:    session.GetRole(fn.Name()),
			root:    nil,
			leaf:    nil,
			visited: make(map[*ssa.BasicBlock]sesstype.Node),
		},
	}
}
