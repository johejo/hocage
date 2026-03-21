package main

import (
	"sort"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

type mapLib struct{}

func (l *mapLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("keys",
			cel.Overload("keys_map",
				[]*cel.Type{cel.DynType},
				cel.ListType(cel.StringType),
				cel.UnaryBinding(keysImpl),
			),
		),
		cel.Function("values",
			cel.Overload("values_map",
				[]*cel.Type{cel.DynType},
				cel.ListType(cel.DynType),
				cel.UnaryBinding(valuesImpl),
			),
		),
		cel.Function("to_entries",
			cel.Overload("to_entries_map",
				[]*cel.Type{cel.DynType},
				cel.ListType(cel.DynType),
				cel.UnaryBinding(toEntriesImpl),
			),
		),
		cel.Function("from_entries",
			cel.Overload("from_entries_list",
				[]*cel.Type{cel.ListType(cel.DynType)},
				cel.DynType,
				cel.UnaryBinding(fromEntriesImpl),
			),
		),
	}
}

func (l *mapLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

// sortedKeys extracts all string keys from a CEL map and returns them sorted.
func sortedKeys(m traits.Mapper) ([]string, ref.Val) {
	it := m.Iterator()
	var keys []string
	for it.HasNext() == types.True {
		k := it.Next()
		s, ok := k.Value().(string)
		if !ok {
			return nil, types.NewErr("non-string key %v", k)
		}
		keys = append(keys, s)
	}
	sort.Strings(keys)
	return keys, nil
}

func keysImpl(arg ref.Val) ref.Val {
	m, ok := arg.(traits.Mapper)
	if !ok {
		return types.NewErr("keys: expected map, got %s", arg.Type())
	}
	keys, errVal := sortedKeys(m)
	if errVal != nil {
		return errVal
	}
	result := make([]ref.Val, len(keys))
	for i, k := range keys {
		result[i] = types.String(k)
	}
	return types.DefaultTypeAdapter.NativeToValue(result)
}

func valuesImpl(arg ref.Val) ref.Val {
	m, ok := arg.(traits.Mapper)
	if !ok {
		return types.NewErr("values: expected map, got %s", arg.Type())
	}
	keys, errVal := sortedKeys(m)
	if errVal != nil {
		return errVal
	}
	result := make([]ref.Val, len(keys))
	for i, k := range keys {
		result[i] = m.Get(types.String(k))
	}
	return types.DefaultTypeAdapter.NativeToValue(result)
}

func toEntriesImpl(arg ref.Val) ref.Val {
	m, ok := arg.(traits.Mapper)
	if !ok {
		return types.NewErr("to_entries: expected map, got %s", arg.Type())
	}
	keys, errVal := sortedKeys(m)
	if errVal != nil {
		return errVal
	}
	entries := make([]ref.Val, len(keys))
	for i, k := range keys {
		v := m.Get(types.String(k))
		entry := map[string]any{
			"key":   k,
			"value": v.Value(),
		}
		entries[i] = types.DefaultTypeAdapter.NativeToValue(entry)
	}
	return types.DefaultTypeAdapter.NativeToValue(entries)
}

func fromEntriesImpl(arg ref.Val) ref.Val {
	list, ok := arg.(traits.Lister)
	if !ok {
		return types.NewErr("from_entries: expected list, got %s", arg.Type())
	}
	result := make(map[string]any)
	size := list.Size().(types.Int)
	for i := types.Int(0); i < size; i++ {
		entry := list.Get(i)
		entryMap, ok := entry.(traits.Mapper)
		if !ok {
			return types.NewErr("from_entries: entry %d is not a map", i)
		}
		keyVal := entryMap.Get(types.String("key"))
		k, ok := keyVal.Value().(string)
		if !ok {
			return types.NewErr("from_entries: entry %d key is not a string", i)
		}
		valVal := entryMap.Get(types.String("value"))
		result[k] = valVal.Value()
	}
	return types.DefaultTypeAdapter.NativeToValue(result)
}
