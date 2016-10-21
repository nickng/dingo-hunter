package cfsmextract

// This file contains only the functions needed to start the analysis
//  - Handle command line flags
//  - Set up session variables

import (
	"fmt"
	"go/types"
	"log"
	"os"
	"time"

	"github.com/nickng/dingo-hunter/cfsmextract/sesstype"
	"github.com/nickng/dingo-hunter/cfsmextract/utils"
	"github.com/nickng/dingo-hunter/ssabuilder"
	"golang.org/x/tools/go/ssa"
)

type CFSMExtract struct {
	SSA   *ssabuilder.SSAInfo
	Time  time.Duration
	Done  chan struct{}
	Error chan error

	session *sesstype.Session
	goQueue []*frame
	prefix  string
	outdir  string
}

func New(ssainfo *ssabuilder.SSAInfo, prefix, outdir string) *CFSMExtract {
	return &CFSMExtract{
		SSA:   ssainfo,
		Done:  make(chan struct{}),
		Error: make(chan error),

		session: sesstype.CreateSession(),
		goQueue: []*frame{},
		prefix:  prefix,
		outdir:  outdir,
	}
}

// Run function analyses main.main() then all the goroutines collected, and
// finally output the analysis results.
func (extract *CFSMExtract) Run() {
	startTime := time.Now()
	mainPkg := ssabuilder.MainPkg(extract.SSA.Prog)
	if mainPkg == nil {
		fmt.Fprintf(os.Stderr, "Error: 'main' package not found\n")
		os.Exit(1)
	}
	init := mainPkg.Func("init")
	main := mainPkg.Func("main")
	fr := makeToplevelFrame(extract)
	for _, pkg := range extract.SSA.Prog.AllPackages() {
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
	for len(extract.goQueue) > 0 {
		goFrm, extract.goQueue = extract.goQueue[0], extract.goQueue[1:]
		fmt.Fprintf(os.Stderr, "\n%s\nLOCATION: %s%s\n", goFrm.fn.Name(), goFrm.gortn.role.Name(), loc(goFrm, goFrm.fn.Pos()))
		visitFunc(goFrm.fn, goFrm)
		goFrm.env.session.Types[goFrm.gortn.role] = goFrm.gortn.root
	}

	extract.Time = time.Since(startTime)
	extract.Done <- struct{}{}
}

// Session returns the session after extraction.
func (extract *CFSMExtract) Session() *sesstype.Session {
	return extract.session
}

func (extract *CFSMExtract) WriteOutput() {
	fmt.Printf(" ----- Results ----- \n%s\n", extract.session.String())

	sesstype.PrintNodeSummary(extract.session)

	dotFile, err := os.OpenFile(fmt.Sprintf("%s.dot", extract.prefix), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer dotFile.Close()

	dot := sesstype.NewGraphvizDot(extract.session)
	if _, err = dot.WriteTo(dotFile); err != nil {
		panic(err)
	}

	os.MkdirAll(extract.outdir, 0750)
	cfsmPath := fmt.Sprintf("%s/%s_cfsms", extract.outdir, extract.prefix)
	cfsmFile, err := os.OpenFile(cfsmPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer cfsmFile.Close()

	cfsms := sesstype.NewCFSMs(extract.session)
	if _, err := cfsms.WriteTo(cfsmFile); err != nil {
		log.Fatalf("Cannot write CFSMs to file: %v", err)
	}

	fmt.Fprintf(os.Stderr, "CFSMs written to %s\n", cfsmPath)
	cfsms.PrintSummary()
}
