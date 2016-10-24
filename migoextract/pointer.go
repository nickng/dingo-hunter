package migoextract

// Utility functions for dealing with pointers.

import (
	"go/types"
)

// derefType dereferences a pointer type once.
func derefType(t types.Type) types.Type {
	if p, ok := t.Underlying().(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

// derefAllType dereferences a pointer type until its base type.
func derefAllType(t types.Type) types.Type {
	baseT := t
	for {
		if p, ok := baseT.Underlying().(*types.Pointer); ok {
			baseT = p.Elem()
		} else {
			return baseT
		}
	}
}
