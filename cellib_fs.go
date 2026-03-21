package main

import (
	"os"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type fsLib struct{}

func (l *fsLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("file_exists",
			cel.Overload("file_exists_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(fileExistsImpl),
			),
		),
		cel.Function("dir_exists",
			cel.Overload("dir_exists_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(dirExistsImpl),
			),
		),
	}
}

func (l *fsLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func fileExistsImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	info, err := os.Stat(path)
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(!info.IsDir())
}

func dirExistsImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	info, err := os.Stat(path)
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(info.IsDir())
}
