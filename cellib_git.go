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
		cel.Function("git_branch",
			cel.Overload("git_branch",
				[]*cel.Type{},
				cel.StringType,
				cel.FunctionBinding(gitBranchImpl),
			),
		),
		cel.Function("git_ignored",
			cel.Overload("git_ignored_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(gitIgnoredImpl),
			),
		),
		cel.Function("git_modified",
			cel.Overload("git_modified_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(gitModifiedImpl),
			),
		),
		cel.Function("git_staged",
			cel.Overload("git_staged_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(gitStagedImpl),
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

func gitBranchImpl(args ...ref.Val) ref.Val {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return types.String("")
	}
	return types.String(strings.TrimSpace(string(out)))
}

func gitIgnoredImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	err := exec.CommandContext(ctx, "git", "check-ignore", "-q", path).Run()
	return types.Bool(err == nil)
}

func gitModifiedImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "diff", "--name-only", "--", path).Output()
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(strings.TrimSpace(string(out)) != "")
}

func gitStagedImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "diff", "--cached", "--name-only", "--", path).Output()
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(strings.TrimSpace(string(out)) != "")
}
