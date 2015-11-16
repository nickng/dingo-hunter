package main

// From golang.org/x/tools/go/ssa/interp/interp.go

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types"
)

func loc(fset *token.FileSet, pos token.Pos) string {
	if pos == token.NoPos {
		return "(unknown)"
	}
	return fset.Position(pos).String()
}

func red(s string) string {
	return fmt.Sprintf("\033[31m%s\033[0m", s)
}

func orange(s string) string {
	return fmt.Sprintf("\033[33m%s\033[0m", s)
}

func green(s string) string {
	return fmt.Sprintf("\033[32m%s\033[0m", s)
}

func cyan(s string) string {
	return fmt.Sprintf("\033[36m%s\033[0m", s)
}

func reg(reg ssa.Value) string {
	if reg == nil {
		return "???.nil"
	}
	if reg.Parent() != nil {
		return fmt.Sprintf("%s.\033[4m%s\033[0m", reg.Parent().String(), reg.Name())
	}
	return fmt.Sprintf("???.\033[4m%s\033[0m", reg.Name())
}

func deref(typ types.Type) types.Type {
	t := typ
	for {
		if p, ok := t.Underlying().(*types.Pointer); ok {
			t = p.Elem()
		} else {
			return t
		}
	}
	return t
}
