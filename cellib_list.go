package main

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

type listLib struct{}

func (l *listLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("min",
			cel.Overload("min_list",
				[]*cel.Type{cel.ListType(cel.DynType)},
				cel.DynType,
				cel.UnaryBinding(minImpl),
			),
		),
		cel.Function("max",
			cel.Overload("max_list",
				[]*cel.Type{cel.ListType(cel.DynType)},
				cel.DynType,
				cel.UnaryBinding(maxImpl),
			),
		),
	}
}

func (l *listLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func minImpl(arg ref.Val) ref.Val {
	list, ok := arg.(traits.Lister)
	if !ok {
		return types.NewErr("min: expected list, got %s", arg.Type())
	}
	size := list.Size().(types.Int)
	if size == 0 {
		return types.NewErr("min: empty list")
	}
	result := list.Get(types.Int(0))
	for i := types.Int(1); i < size; i++ {
		v := list.Get(i)
		cmpr, ok := result.(traits.Comparer)
		if !ok {
			return types.NewErr("min: element is not comparable")
		}
		if cmpr.Compare(v) == types.IntOne {
			result = v
		}
	}
	return result
}

func maxImpl(arg ref.Val) ref.Val {
	list, ok := arg.(traits.Lister)
	if !ok {
		return types.NewErr("max: expected list, got %s", arg.Type())
	}
	size := list.Size().(types.Int)
	if size == 0 {
		return types.NewErr("max: empty list")
	}
	result := list.Get(types.Int(0))
	for i := types.Int(1); i < size; i++ {
		v := list.Get(i)
		cmpr, ok := result.(traits.Comparer)
		if !ok {
			return types.NewErr("max: element is not comparable")
		}
		if cmpr.Compare(v) == types.IntNegOne {
			result = v
		}
	}
	return result
}
