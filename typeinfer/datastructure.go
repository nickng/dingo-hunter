package typeinfer

// Data structue utilities.

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Elems are maps from array indices (variable) to VarInstances of elements.
type Elems map[ssa.Value]VarInstance

// Fields is a slice of variable instances.
type Fields []VarInstance

func (caller *Function) getStructField(struc ssa.Value, idx int) (VarInstance, error) {
	if instance, ok := caller.locals[struc]; ok {
		if fields, ok := caller.structs[instance]; ok {
			return fields[idx], nil
		} else if fields, ok := caller.Prog.structs[instance]; ok {
			return fields[idx], nil
		}
	}
	return nil, ErrInvalidVarRead
}

func (caller *Function) setStructField(struc ssa.Value, idx int, instance VarInstance) {
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
func initNestedRefVar(infer *TypeInfer, f *Function, b *Block, l *Loop, inst VarInstance, heap bool) {
	var Arrays map[VarInstance]Elems
	var Structs map[VarInstance]Fields
	if heap {
		Arrays, Structs = f.Prog.arrays, f.Prog.structs
	} else {
		Arrays, Structs = f.arrays, f.structs
	}
	v, ok := inst.(*Instance)
	if !ok {
		return
	}
	switch t := derefAllType(v.Var().Type()).Underlying().(type) {
	case *types.Array:
		if _, ok := Arrays[inst]; !ok {
			Arrays[inst] = make(Elems, t.Len())
			infer.Logger.Print(f.Sprintf(SubSymbol+"initialised %s as array (type: %s)", inst, inst.Var().Type()))
		}
	case *types.Slice:
		if _, ok := Arrays[inst]; !ok {
			Arrays[inst] = make(Elems, 0)
			infer.Logger.Print(f.Sprintf(SubSymbol+"initialised %s as slice (type: %s)", inst, inst.Var().Type()))
		}
	case *types.Struct:
		if _, ok := Structs[inst]; !ok {
			Structs[inst] = make(Fields, t.NumFields())
			infer.Logger.Print(f.Sprintf(SubSymbol+"initialised %s as struct (type: %s)", inst, inst.Var().Type()))
		}
	default:
	}
}
