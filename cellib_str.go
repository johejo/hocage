package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

type strLib struct{}

func (l *strLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("trim_prefix",
			cel.Overload("trim_prefix_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.StringType,
				cel.BinaryBinding(trimPrefixImpl),
			),
		),
		cel.Function("trim_suffix",
			cel.Overload("trim_suffix_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.StringType,
				cel.BinaryBinding(trimSuffixImpl),
			),
		),
		cel.Function("path_base",
			cel.Overload("path_base_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(pathBaseImpl),
			),
		),
		cel.Function("path_dir",
			cel.Overload("path_dir_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(pathDirImpl),
			),
		),
		cel.Function("path_ext",
			cel.Overload("path_ext_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(pathExtImpl),
			),
		),
		cel.Function("path_clean",
			cel.Overload("path_clean_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(pathCleanImpl),
			),
		),
		cel.Function("path_join",
			cel.Overload("path_join_list",
				[]*cel.Type{cel.ListType(cel.StringType)},
				cel.StringType,
				cel.UnaryBinding(pathJoinImpl),
			),
		),
		cel.Function("quote",
			cel.Overload("quote_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(quoteImpl),
			),
		),
		cel.Function("squote",
			cel.Overload("squote_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(squoteImpl),
			),
		),
		cel.Function("indent",
			cel.Overload("indent_int_string",
				[]*cel.Type{cel.IntType, cel.StringType},
				cel.StringType,
				cel.BinaryBinding(indentImpl),
			),
		),
	}
}

func (l *strLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func trimPrefixImpl(lhs, rhs ref.Val) ref.Val {
	s, ok := lhs.Value().(string)
	if !ok {
		return types.NewErr("trimPrefix: first argument must be string")
	}
	prefix, ok := rhs.Value().(string)
	if !ok {
		return types.NewErr("trimPrefix: second argument must be string")
	}
	return types.String(strings.TrimPrefix(s, prefix))
}

func trimSuffixImpl(lhs, rhs ref.Val) ref.Val {
	s, ok := lhs.Value().(string)
	if !ok {
		return types.NewErr("trimSuffix: first argument must be string")
	}
	suffix, ok := rhs.Value().(string)
	if !ok {
		return types.NewErr("trimSuffix: second argument must be string")
	}
	return types.String(strings.TrimSuffix(s, suffix))
}

func pathBaseImpl(arg ref.Val) ref.Val {
	s, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("path_base: argument must be string")
	}
	return types.String(filepath.Base(s))
}

func pathDirImpl(arg ref.Val) ref.Val {
	s, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("path_dir: argument must be string")
	}
	return types.String(filepath.Dir(s))
}

func pathExtImpl(arg ref.Val) ref.Val {
	s, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("path_ext: argument must be string")
	}
	return types.String(filepath.Ext(s))
}

func pathCleanImpl(arg ref.Val) ref.Val {
	s, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("path_clean: argument must be string")
	}
	return types.String(filepath.Clean(s))
}

func pathJoinImpl(arg ref.Val) ref.Val {
	list, ok := arg.(traits.Lister)
	if !ok {
		return types.NewErr("path_join: argument must be list")
	}
	size := list.Size().(types.Int)
	parts := make([]string, size)
	for i := types.Int(0); i < size; i++ {
		v := list.Get(i)
		s, ok := v.Value().(string)
		if !ok {
			return types.NewErr("path_join: element %d must be string", i)
		}
		parts[i] = s
	}
	return types.String(filepath.Join(parts...))
}

func quoteImpl(arg ref.Val) ref.Val {
	s, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("quote: argument must be string")
	}
	return types.String(fmt.Sprintf("%q", s))
}

func squoteImpl(arg ref.Val) ref.Val {
	s, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("squote: argument must be string")
	}
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return types.String("'" + escaped + "'")
}

func indentImpl(lhs, rhs ref.Val) ref.Val {
	n, ok := lhs.Value().(int64)
	if !ok {
		return types.NewErr("indent: first argument must be int")
	}
	if n < 0 {
		return types.NewErr("indent: spaces must be non-negative")
	}
	s, ok := rhs.Value().(string)
	if !ok {
		return types.NewErr("indent: second argument must be string")
	}
	pad := strings.Repeat(" ", int(n))
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return types.String(strings.Join(lines, "\n"))
}
