package typeinfer

// Data structure utilities.

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Elems are maps from array indices (variable) to VarInstances of elements.
type Elems map[ssa.Value]Instance

// Fields is a slice of variable instances.
type Fields []Instance

func (caller *Function) getStructField(struc ssa.Value, idx int) (Instance, error) {
	if instance, ok := caller.locals[struc]; ok {
		if fields, ok := caller.structs[instance]; ok {
			return fields[idx], nil
		} else if fields, ok := caller.Prog.structs[instance]; ok {
			return fields[idx], nil
		}
	}
	return nil, ErrInvalidVarRead
}

func (caller *Function) setStructField(struc ssa.Value, idx int, instance Instance) {
	if instance, ok := caller.locals[struc]; ok {
		if _, ok := caller.structs[instance]; ok {
			caller.structs[instance][idx] = instance
			return
		} else if _, ok := caller.Prog.structs[instance]; ok {
			caller.Prog.structs[instance][idx] = instance
			return
		}
	}
}

// initNestedRefVar initialises empty reference data structures {array,slice,struct} not used
// before
func initNestedRefVar(infer *TypeInfer, ctx *Context, inst Instance, heap bool) {
	var s *Storage
	if heap {
		s = ctx.F.Prog.Storage
	} else {
		s = ctx.F.Storage
	}
	v, ok := inst.(*Value)
	if !ok {
		return
	}
	switch t := derefAllType(v.Type()).Underlying().(type) {
	case *types.Array:
		if _, ok := s.arrays[inst]; !ok {
			s.arrays[inst] = make(Elems, t.Len())
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"initialised %s as array (type: %s)", inst, v.Type()))
		}
	case *types.Slice:
		if _, ok := s.arrays[inst]; !ok {
			s.arrays[inst] = make(Elems, 0)
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"initialised %s as slice (type: %s)", inst, v.Type()))
		}
	case *types.Struct:
		if _, ok := s.structs[inst]; !ok {
			s.structs[inst] = make(Fields, t.NumFields())
			infer.Logger.Print(ctx.F.Sprintf(SubSymbol+"initialised %s as struct (type: %s)", inst, v.Type()))
		}
	default:
	}
}
