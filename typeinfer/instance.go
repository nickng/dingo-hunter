package typeinfer

import (
	"fmt"
	"go/constant"

	"golang.org/x/tools/go/ssa"
)

// VarInstance is an interface for an instance of a defined value.
type VarInstance interface {
	Var() ssa.Value
	IsConcrete() bool
	GetInstance() int
	GetIndex() int64
	String() string
}

// Instance captures a specific instance of a SSA value by counting the
// number of instantiations.
type Instance struct {
	ssa.Value
	Instance int   // Instance number (default: 0)
	Index    int64 // Loop index associated with var (default: 0)
}

// Var returns the SSA value.
func (i *Instance) Var() ssa.Value { return i.Value }

func (i *Instance) IsConcrete() bool { return true }

// GetInstance() returns the instance number.
func (i *Instance) GetInstance() int { return i.Instance }

// GetIndex() returns the loop index.
func (i *Instance) GetIndex() int64 { return i.Index }
func (i *Instance) String() string {
	var parent string
	if i.Parent() != nil {
		parent = i.Parent().String()
	} else {
		parent = "__main__"
	}
	return fmt.Sprintf("%s.%s_%d_%d", parent, i.Name(), i.Instance, i.Index)
}

// ExtInstance captures an external instance of a SSA value by counting the
// instantiations.
//
// An external instance is one without ssa.Value, usually if the creating body
// is in runtime or not built as SSA.
type ExtInstance struct {
	parent   *ssa.Function
	Instance int
	Index    int64
}

// Var returns the SSA value (always nil).
func (i *ExtInstance) Var() ssa.Value { return nil }

func (i *ExtInstance) IsConcrete() bool { return false }

// GetInstance() returns the instance number.
func (i *ExtInstance) GetInstance() int { return i.Instance }

// GetIndex() returns the loop index.
func (i *ExtInstance) GetIndex() int64 { return i.Index }

func (i *ExtInstance) String() string {
	return fmt.Sprintf("%s.%s_%d_%d", i.parent, "_retval_", i.Instance, i.Index)
}

// ConstInstance captures a constant value.
//
// This is just a wrapper.
type ConstInstance struct {
	Const *ssa.Const
}

// Var returns the SSA value (always nil).
func (i *ConstInstance) Var() ssa.Value { return i.Const }

// IsConcrete always return true.
func (i *ConstInstance) IsConcrete() bool { return true }

// GetInstance returns the instance number.
func (i *ConstInstance) GetInstance() int { return 0 }

// GetIndex returns the loop index.
func (i *ConstInstance) GetIndex() int64 { return 0 }

func (i *ConstInstance) String() string {
	switch i.Const.Value.Kind() {
	case constant.Int:
		return fmt.Sprintf("%d", i.Const.Int64())
	case constant.Bool:
		return fmt.Sprintf("%s", i.Const.String())
	case constant.String:
		return fmt.Sprintf("%s", i.Const.String())
	default:
		return fmt.Sprintf("__unimplemented__ %s", i.Const.Type())
	}
}
