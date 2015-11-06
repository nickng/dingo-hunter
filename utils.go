package main

// From golang.org/x/tools/go/ssa/interp/interp.go

import (
	"fmt"
	"go/token"
	"golang.org/x/tools/go/types"
)

func loc(fset *token.FileSet, pos token.Pos) string {
	if pos == token.NoPos {
		return ""
	}
	return " at " + fset.Position(pos).String()
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

func deref(typ types.Type) types.Type {
	if p, ok := typ.Underlying().(*types.Pointer); ok {
		return p.Elem()
	}
	return typ
}
