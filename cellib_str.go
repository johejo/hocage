package main

import (
	"path/filepath"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
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
