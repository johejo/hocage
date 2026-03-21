package main

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

type defaultLib struct{}

func (l *defaultLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("default",
			cel.Overload("default_dyn_dyn",
				[]*cel.Type{cel.DynType, cel.DynType},
				cel.DynType,
				cel.BinaryBinding(defaultImpl),
			),
		),
	}
}

func (l *defaultLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func defaultImpl(lhs, rhs ref.Val) ref.Val {
	if isEmpty(rhs) {
		return lhs
	}
	return rhs
}

func isEmpty(v ref.Val) bool {
	if types.IsUnknownOrError(v) {
		return true
	}
	switch val := v.Value().(type) {
	case string:
		return val == ""
	case bool:
		return !val
	case int64:
		return val == 0
	case uint64:
		return val == 0
	case float64:
		return val == 0
	case nil:
		return true
	}
	if list, ok := v.(traits.Lister); ok {
		return list.Size().(types.Int) == 0
	}
	if m, ok := v.(traits.Mapper); ok {
		return m.Size().(types.Int) == 0
	}
	return false
}
