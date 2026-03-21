package main

import (
	"encoding/json"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

type encodingLib struct{}

func (l *encodingLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("to_json",
			cel.Overload("to_json_dyn",
				[]*cel.Type{cel.DynType},
				cel.StringType,
				cel.UnaryBinding(toJSONImpl),
			),
		),
		cel.Function("from_json",
			cel.Overload("from_json_string",
				[]*cel.Type{cel.StringType},
				cel.DynType,
				cel.UnaryBinding(fromJSONImpl),
			),
		),
	}
}

func (l *encodingLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func toJSONImpl(arg ref.Val) ref.Val {
	v := celToNative(arg)
	b, err := json.Marshal(v)
	if err != nil {
		return types.NewErr("to_json: %v", err)
	}
	return types.String(string(b))
}

// celToNative converts a CEL ref.Val to a native Go value suitable for json.Marshal.
func celToNative(v ref.Val) any {
	if m, ok := v.(traits.Mapper); ok {
		result := make(map[string]any)
		it := m.Iterator()
		for it.HasNext() == types.True {
			k := it.Next()
			val := m.Get(k)
			key, ok := k.Value().(string)
			if !ok {
				continue
			}
			result[key] = celToNative(val)
		}
		return result
	}
	if list, ok := v.(traits.Lister); ok {
		size := list.Size().(types.Int)
		result := make([]any, size)
		for i := types.Int(0); i < size; i++ {
			result[i] = celToNative(list.Get(i))
		}
		return result
	}
	return v.Value()
}

func fromJSONImpl(arg ref.Val) ref.Val {
	s, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("from_json: argument must be string")
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return types.NewErr("from_json: %v", err)
	}
	return types.DefaultTypeAdapter.NativeToValue(v)
}
