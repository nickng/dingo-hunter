package cfsmextract

// +build go1.6

// This file contains only the functions needed to start the analysis
//  - Handle command line flags
//  - Set up session variables

import (
	"flag"
	"fmt"
	"go/build"
	"go/types"
	"os"
	"time"

	"github.com/nickng/dingo-hunter/cfsmextract/sesstype"
	"github.com/nickng/dingo-hunter/cfsmextract/sesstype/generator"
	"github.com/nickng/dingo-hunter/cfsmextract/utils"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var (
	session *sesstype.Session // Keeps track of the all session
	ssaflag = ssa.BareInits
	goQueue = make([]*frame, 0)
)

func init() { flag.Var(&ssaflag, "ssa", "ssa mode (default: BareInits)") }

const usage = "Usage dingo-hunter <main.go> ...\n"

// main function analyses the program in four steps
//
// (1) Load program as SSA
// (2) Analyse main.main()
// (3) Analyse goroutines found in (2)
// (4) Output results
func Extract(files []string, prefix string, outdir string) {
	var prog *ssa.Program
	var err error

	startTime := time.Now()

	prog, err = loadSSA(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading files: %s\n", err)
	}

	mainPkg := findMainPkg(prog)
	if mainPkg == nil {
		fmt.Fprintf(os.Stderr, "Error: 'main' package not found\n")
		os.Exit(1)
	}

	session = sesstype.CreateSession() // init needs Session
	init := mainPkg.Func("init")
	main := mainPkg.Func("main")

	fr := makeToplevelFrame()
	for _, pkg := range prog.AllPackages() {
		for _, memb := range pkg.Members {
			switch val := memb.(type) {
			case *ssa.Global:
				switch derefAll(val.Type()).(type) {
				case *types.Array:
					vd := utils.NewDef(val)
					fr.env.globals[val] = vd
					fr.env.arrays[vd] = make(Elems)

				case *types.Struct:
					vd := utils.NewDef(val)
					fr.env.globals[val] = vd
					fr.env.structs[vd] = make(Fields)

				case *types.Chan:
					var c *types.Chan
					vd := utils.NewDef(utils.EmptyValue{T: c})
					fr.env.globals[val] = vd

				default:
					fr.env.globals[val] = utils.NewDef(val)
				}
			}
		}
	}

	fmt.Fprintf(os.Stderr, "++ call.toplevel %s()\n", orange("init"))
	visitFunc(init, fr)
	if main == nil {
		fmt.Fprintf(os.Stderr, "Error: 'main()' function not found in 'main' package\n")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "++ call.toplevel %s()\n", orange("main"))
	visitFunc(main, fr)

	fr.env.session.Types[fr.gortn.role] = fr.gortn.root

	var goFrm *frame
	for len(goQueue) > 0 {
		goFrm, goQueue = goQueue[0], goQueue[1:]
		fmt.Fprintf(os.Stderr, "\n%s\nLOCATION: %s%s\n", goFrm.fn.Name(), goFrm.gortn.role.Name(), loc(goFrm, goFrm.fn.Pos()))
		visitFunc(goFrm.fn, goFrm)
		goFrm.env.session.Types[goFrm.gortn.role] = goFrm.gortn.root
	}

	elapsedTime := time.Since(startTime)

	fmt.Printf("Analysis time: %f\n", elapsedTime.Seconds())

	fmt.Printf(" ----- Results ----- \n%s\n", session.String())

	sesstype.PrintNodeSummary(session)

	dotFile, err := os.OpenFile(fmt.Sprintf("%s.dot", prefix), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer dotFile.Close()

	if _, err = generator.GenDot(session, dotFile); err != nil {
		panic(err)
	}

	os.MkdirAll(outdir, 0750)
	cfsmPath := fmt.Sprintf("%s/%s_cfsms", outdir, prefix)
	cfsmFile, err := os.OpenFile(cfsmPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer cfsmFile.Close()

	if _, err = generator.GenCFSMs(session, cfsmFile); err != nil {
		panic(err)
	}

	fmt.Fprintf(os.Stderr, "CFSMs written to %s\n", cfsmPath)
	generator.PrintCFSMSummary()
}

// Load command line arguments as SSA program for analysis
func loadSSA(files []string) (*ssa.Program, error) {
	if len(files) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	var conf = loader.Config{Build: &build.Default}

	// Use the initial packages from the command line.
	if _, err := conf.FromArgs(files /*test?*/, false); err != nil {
		return nil, err
	}

	// Load, parse and type-check the whole program.
	prog, err := conf.Load()
	if err != nil {
		return nil, err
	}
	progSSA := ssautil.CreateProgram(prog, ssaflag) // If ssabuild specified

	// Build and display only the initial packages (and synthetic wrappers),
	// unless -run is specified.
	//
	// Adapted from golang.org/x/tools/go/ssa
	for _, info := range prog.InitialPackages() {
		progSSA.Package(info.Pkg).Build()
	}

	// Don't load these packages.
	for _, info := range prog.AllPackages {
		if info.Pkg.Name() != "fmt" && info.Pkg.Name() != "reflect" && info.Pkg.Name() != "strings" && info.Pkg.Name() != "runtime" && info.Pkg.Name() != "sync" {
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
