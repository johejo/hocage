package main

import (
	"os"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type envLib struct{}

func (l *envLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("env",
			cel.Overload("env_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(envImpl),
			),
		),
	}
}

func (l *envLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func envImpl(arg ref.Val) ref.Val {
	name, ok := arg.Value().(string)
	if !ok {
		return types.String("")
	}
	return types.String(os.Getenv(name))
}
