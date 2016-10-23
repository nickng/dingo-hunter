package ssabuilder

// Pointer analysis helper functions.
// Most of the functions in this file are modified from golan.org/x/tools/oracle

import (
	"fmt"
	"io"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

// Create a pointer.Config whose scope is the initial packages of lprog
// and their dependencies.
func setupPTA(prog *ssa.Program, lprog *loader.Program, ptaLog io.Writer) (*pointer.Config, error) {
	// TODO(adonovan): the body of this function is essentially
	// duplicated in all go/pointer clients.  Refactor.

	// For each initial package (specified on the command line),
	// if it has a main function, analyze that,
	// otherwise analyze its tests, if any.
	var testPkgs, mains []*ssa.Package
	for _, info := range lprog.InitialPackages() {
		initialPkg := prog.Package(info.Pkg)

		// Add package to the pointer analysis scope.
		if initialPkg.Func("main") != nil {
			mains = append(mains, initialPkg)
		} else {
			testPkgs = append(testPkgs, initialPkg)
		}
	}
	if testPkgs != nil {
		for _, testPkg := range testPkgs {
			if p := prog.CreateTestMainPackage(testPkg); p != nil {
				mains = append(mains, p)
			}
		}
	}
	if mains == nil {
		return nil, fmt.Errorf("analysis scope has no main and no tests")
	}
	return &pointer.Config{
		Log:        ptaLog,
		Mains:      mains,
		Reflection: false, // We don't consider reflection in our analysis.
	}, nil
}
