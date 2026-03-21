package main

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

const gitTimeout = 10 * time.Second

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
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "ls-files", "--", path).Output()
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(strings.TrimSpace(string(out)) != "")
}
