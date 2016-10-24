package migoextract

// Wrapper of values and constants in an SSA program.
// Designed for tracking usage and instances of values used in SSA program.

import (
	"bytes"
	"fmt"
	"go/constant"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Instance is an interface for an instance of a defined value.
type Instance interface {
	Instance() (int, int)
	String() string
}

// Value captures a specific instance of a SSA value by counting the number of
// instantiations.
type Value struct {
	ssa.Value       // Storing the ssa.Value this instance is for.
	instID    int   // Instance number (default: 0).
	loopIdx   int64 // Loop index associated with var (default: 0).
}

// Instance returns the instance identifier pair.
func (i *Value) Instance() (int, int) {
	return i.instID, int(i.loopIdx)
}

func (i *Value) String() string {
	var prefix bytes.Buffer
	if i.Parent() != nil {
		prefix.WriteString(i.Parent().String())
	} else {
		prefix.WriteString("__main__")
	}
	return fmt.Sprintf("%s.%s_%d_%d", prefix.String(), i.Name(), i.instID, i.loopIdx)
}

// Placeholder is a temporary stand in for actual SSA Value.
type Placeholder struct {
}

// Instance returns the instance number.
func (i *Placeholder) Instance() (int, int) {
	return -1, -1
}

func (i *Placeholder) String() string {
	return fmt.Sprintf("placeholder instance")
}

// External captures an external instance of an SSA value.
//
// An external instance is one without ssa.Value, usually if the creating body
// is in runtime or not built as SSA.
type External struct {
	parent *ssa.Function // Parent (enclosing) function.
	typ    types.Type    // Type of returned instance.
	instID int           // Instance number (default: 0).
}

// Instance returns the instance number.
func (i *External) Instance() (int, int) {
	return i.instID, 0
}

func (i *External) String() string {
	var prefix bytes.Buffer
	if i.parent != nil {
		prefix.WriteString(i.parent.String())
	} else {
		prefix.WriteString("__unknown__")
	}
	return fmt.Sprintf("%b.%s_%d:%s", prefix, "__ext", i.instID, i.typ.String())
}

// Const captures a constant value.
//
// This is just a wrapper.
type Const struct {
	*ssa.Const
}

// Instance returns the instance identifier pair.
func (c *Const) Instance() (int, int) { return 0, 0 }

func (c *Const) String() string {
	switch c.Const.Value.Kind() {
	case constant.Bool:
		return fmt.Sprintf("%s", c.Const.String())
	case constant.Complex:
		return fmt.Sprintf("%v", c.Const.Complex128())
	case constant.Float:
		return fmt.Sprintf("%f", c.Const.Float64())
	case constant.Int:
		return fmt.Sprintf("%d", c.Const.Int64())
	case constant.String:
		return fmt.Sprintf("%s", c.Const.String())
	default:
		panic("unknown constant type")
	}
}
