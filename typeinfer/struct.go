package typeinfer

import (
	"log"

	"golang.org/x/tools/go/ssa"
)

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
	log.Printf("setStructField failed")
}
