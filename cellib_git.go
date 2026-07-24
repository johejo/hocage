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

// gitOutput executes git with the given args under gitTimeout and returns its
// trimmed stdout.
func gitOutput(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitTrackedImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	out, err := gitOutput("ls-files", "--", path)
	return types.Bool(err == nil && out != "")
}

func gitBranchImpl(args ...ref.Val) ref.Val {
	out, err := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return types.String("")
	}
	return types.String(out)
}

func gitIgnoredImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	_, err := gitOutput("check-ignore", "-q", path)
	return types.Bool(err == nil)
}

func gitModifiedImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	out, err := gitOutput("diff", "--name-only", "--", path)
	return types.Bool(err == nil && out != "")
}

func gitStagedImpl(arg ref.Val) ref.Val {
	path, ok := arg.Value().(string)
	if !ok {
		return types.Bool(false)
	}
	out, err := gitOutput("diff", "--cached", "--name-only", "--", path)
	return types.Bool(err == nil && out != "")
}
