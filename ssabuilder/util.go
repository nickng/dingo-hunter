package ssabuilder

import (
	"golang.org/x/tools/go/ssa"
)

// GetMainPkg returns main package of a command.
func MainPkg(prog *ssa.Program) *ssa.Package {
	pkgs := prog.AllPackages()
	for _, pkg := range pkgs {
		if pkg.Pkg.Name() == "main" {
			return pkg
		}
	}
	return nil // Not found
}
