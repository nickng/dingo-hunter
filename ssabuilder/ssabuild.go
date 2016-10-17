// Package ssabuilder provides a wrapper for building SSA IR from Go source code.
//
package ssabuilder // import "github.com/nickng/dingo-hunter/ssabuilder"

import (
	"fmt"
	"go/build"
	"go/token"
	"io"
	"io/ioutil"
	"log"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/nickng/dingo-hunter/ssabuilder/callgraph"
)

// A Mode value is a flag indicating how the source code are supplied.
type Mode uint

const (
	// FromFiles is option to use a list of filenames for initial packages.
	FromFiles Mode = 1 << iota

	// FromString is option to use a string as body of initial package.
	FromString
)

// Config holds the configuration for building SSA IR.
type Config struct {
	BuildMode Mode
	Files     []string          // (Initial) files to load.
	Source    string            // Source code.
	BuildLog  io.Writer         // Build log.
	PtaLog    io.Writer         // Pointer analysis log.
	LogFlags  int               // Flags for build/pta log.
	BadPkgs   map[string]string // Packages not to load (with reasons).
}

// SSAInfo is the SSA IR + metainfo built from a given Config.
type SSAInfo struct {
	BuildConf   *Config  // Build configuration (initial files, logs).
	IgnoredPkgs []string // Packages not loaded (respects BuildConf.BadPkgs).

	FSet    *token.FileSet  // FileSet for parsed source files.
	Prog    *ssa.Program    // SSA IR for whole program.
	PtaConf *pointer.Config // Pointer analysis config.

	Logger *log.Logger // Build logger.
}

var (
	// Packages that should not be loaded (and reasons) by default
	badPkgs = map[string]string{
		"fmt":     "Recursive calls unrelated to communication",
		"reflect": "Reflection not supported for static analyser",
		"runtime": "Runtime contains threads that are not user related",
		"strings": "Strings function does not have communication",
		"sync":    "Atomics confuse analyser",
		"time":    "Time not supported",
		"rand":    "Math does not use channels",
	}
)

// NewConfig creates a new default build configuration.
func NewConfig(files []string) (*Config, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no files specified or analysis")
	}
	return &Config{
		BuildMode: FromFiles,
		Files:     files,
		BuildLog:  ioutil.Discard,
		PtaLog:    ioutil.Discard,
		LogFlags:  log.LstdFlags,
		BadPkgs:   badPkgs,
	}, nil
}

// NewConfigFromString creates a new default build configuration.
func NewConfigFromString(s string) (*Config, error) {
	return &Config{
		BuildMode: FromString,
		Source:    s,
		BuildLog:  ioutil.Discard,
		PtaLog:    ioutil.Discard,
		LogFlags:  log.LstdFlags,
		BadPkgs:   badPkgs,
	}, nil
}

// Build constructs the SSA IR using given config, and sets up pointer analysis.
func (conf *Config) Build() (*SSAInfo, error) {
	var lconf = loader.Config{Build: &build.Default}
	buildLog := log.New(conf.BuildLog, "ssabuild: ", conf.LogFlags)

	if conf.BuildMode == FromFiles {
		args, err := lconf.FromArgs(conf.Files, false /* No tests */)
		if err != nil {
			return nil, err
		}
		if len(args) > 0 {
			return nil, fmt.Errorf("surplus arguments: %q", args)
		}
	} else if conf.BuildMode == FromString {
		f, err := lconf.ParseFile("", conf.Source)
		if err != nil {
			return nil, err
		}
		lconf.CreateFromFiles("", f)
	} else {
		buildLog.Fatal("Unknown build mode")

	}

	// Load, parse and type-check program
	lprog, err := lconf.Load()
	if err != nil {
		return nil, err
	}
	buildLog.Print("Program loaded and type checked")

	prog := ssautil.CreateProgram(lprog, ssa.GlobalDebug|ssa.BareInits)

	// Prepare Config for whole-program pointer analysis.
	ptaConf, err := setupPTA(prog, lprog, conf.PtaLog)

	ignoredPkgs := []string{}
	if len(conf.BadPkgs) == 0 {
		prog.Build()
	} else {
		for _, info := range lprog.AllPackages {
			if reason, badPkg := conf.BadPkgs[info.Pkg.Name()]; badPkg {
				buildLog.Printf("Skip package: %s (%s)", info.Pkg.Name(), reason)
				ignoredPkgs = append(ignoredPkgs, info.Pkg.Name())
			} else {
				prog.Package(info.Pkg).Build()
			}
		}
	}

	return &SSAInfo{
		BuildConf:   conf,
		IgnoredPkgs: ignoredPkgs,
		FSet:        lprog.Fset,
		Prog:        prog,
		PtaConf:     ptaConf,
		Logger:      buildLog,
	}, nil
}

// CallGraph builds the call graph from the 'main.main' function.
//
// The call graph is rooted at 'main.main', all nodes appear only once in the
// graph. A side-effect of building the call graph is obtaining a list of
// functions used in a program (as functions not called will not appear in the
// CallGraph).
// TODO(nickng) cache previously built CallGraph.
func (info *SSAInfo) CallGraph() *callgraph.Node {
	mainPkg := GetMainPkg(info.Prog)
	if mainFunc := mainPkg.Func("main"); mainFunc != nil {
		return callgraph.Build(mainFunc)
	}
	return nil // No main pkg --> nothing is called
}

// DecodePos converts a token.Pos (offset) to an actual token.Position.
//
// This is just a shortcut to .FSet.Position.
func (info *SSAInfo) DecodePos(pos token.Pos) token.Position {
	return info.FSet.Position(pos)
}

// NewPta performs a custom pointer analysis on given values.
func (info *SSAInfo) NewPta(vals ...ssa.Value) *pointer.Result {
	for _, val := range vals {
		info.PtaConf.AddQuery(val)
	}
	result, err := pointer.Analyze(info.PtaConf)
	if err != nil {
		info.Logger.Print("NewPta:", ErrPtaInternal)
	}
	return result
}

// FindChan performs a ptr analysis on a given chan ssa.Value, returns a list of
// related ChanOp on the chan.
func (info *SSAInfo) FindChan(ch ssa.Value) []ChanOp {
	chanOps := purgeChanOps(progChanOps(info.Prog), ch)
	for _, op := range chanOps {
		info.PtaConf.AddQuery(op.Value)
	}
	result, err := pointer.Analyze(info.PtaConf)
	if err != nil {
		info.Logger.Print("FindChan failed:", ErrPtaInternal)
	}
	queryCh := result.Queries[ch]
	var ops []ChanOp
	for _, label := range queryCh.PointsTo().Labels() {
		// Add MakeChan to result
		ops = append(ops, ChanOp{label.Value(), ChanMake, label.Pos()})
	}
	for _, op := range chanOps {
		if ptr, ok := result.Queries[op.Value]; ok && ptr.MayAlias(queryCh) {
			ops = append(ops, op)
		}
	}
	return ops
}
