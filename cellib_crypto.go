package main

import (
	"crypto/sha256"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type cryptoLib struct{}

func (l *cryptoLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("sha256sum",
			cel.Overload("sha256sum_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(sha256sumImpl),
			),
		),
	}
}

func (l *cryptoLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func sha256sumImpl(arg ref.Val) ref.Val {
	s, ok := arg.Value().(string)
	if !ok {
		return types.NewErr("sha256sum: argument must be string")
	}
	h := sha256.Sum256([]byte(s))
	return types.String(fmt.Sprintf("%x", h))
}
