package main

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

// EvalContext holds execution environment information available as `ctx` in CEL expressions.
type EvalContext struct {
	CWD         string
	ProjectRoot string
}

// BuildEvalContext creates an EvalContext from the current execution environment.
func BuildEvalContext() (*EvalContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	projectRoot := detectProjectRoot()
	return &EvalContext{CWD: cwd, ProjectRoot: projectRoot}, nil
}

// detectProjectRoot returns the git repository root, or empty string if not in a git repo.
func detectProjectRoot() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// NewActivation builds the CEL activation map containing both event data and execution context.
func NewActivation(event any, evalCtx *EvalContext) map[string]any {
	m := map[string]any{
		"event": event,
	}
	if evalCtx != nil {
		m["ctx"] = map[string]any{
			"cwd":          evalCtx.CWD,
			"project_root": evalCtx.ProjectRoot,
		}
	} else {
		m["ctx"] = map[string]any{}
	}
	return m
}
