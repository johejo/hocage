package main

import (
	"path/filepath"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type globLib struct{}

func (l *globLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("glob_exists",
			cel.Overload("glob_exists_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(globExistsImpl),
			),
		),
	}
}

func (l *globLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

// globExistsImpl returns true if the glob pattern matches at least one file.
// Uses filepath.Glob which does not support ** (recursive) patterns.
func globExistsImpl(arg ref.Val) ref.Val {
	pattern, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(len(matches) > 0)
}
