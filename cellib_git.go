package main

import (
	"os/exec"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type gitLib struct{}

func (l *gitLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("git_tracked",
			cel.Overload("git_tracked_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(gitTrackedImpl),
			),
		),
	}
}

func (l *gitLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

func gitTrackedImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	out, err := exec.Command("git", "ls-files", "--", path).Output()
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(strings.TrimSpace(string(out)) != "")
}
