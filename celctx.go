package main

import (
	"os"
	"slices"
	"strings"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// EvalContext holds execution environment information available as `ctx` in CEL expressions.
type EvalContext struct {
	CWD         string
	ProjectRoot string
	Transcript  []any // nil when transcript.load is false

	// TranscriptLoader, if set, defers the file read until CEL actually
	// resolves `transcript`. Takes precedence over Transcript.
	TranscriptLoader func() ([]any, error)
}

// BuildEvalContext creates an EvalContext from the current execution environment.
// needProjectRoot gates the git subprocess spawned by detectProjectRoot.
func BuildEvalContext(needProjectRoot bool) (*EvalContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	var projectRoot string
	if needProjectRoot {
		projectRoot = detectProjectRoot()
	}
	return &EvalContext{CWD: cwd, ProjectRoot: projectRoot}, nil
}

// HookReferencesProjectRoot is a static substring scan for "project_root" over
// every CEL slot in the hook; a false positive just costs one extra git call.
func HookReferencesProjectRoot(hook *Hook) bool {
	if strings.Contains(hook.When, "project_root") {
		return true
	}
	return actionReferencesProjectRoot(&hook.Action)
}

func actionReferencesProjectRoot(action *Action) bool {
	if valueReferencesProjectRoot(action.Respond) {
		return true
	}
	if valueReferencesProjectRoot(action.Command) {
		return true
	}
	for _, expr := range action.Env {
		if strings.Contains(expr, "project_root") {
			return true
		}
	}
	if valueReferencesProjectRoot(action.Stdin) {
		return true
	}
	if action.HTTP != nil {
		if valueReferencesProjectRoot(action.HTTP.URL) {
			return true
		}
		for _, v := range action.HTTP.Headers {
			if valueReferencesProjectRoot(v) {
				return true
			}
		}
	}
	return false
}

func valueReferencesProjectRoot(v any) bool {
	switch val := v.(type) {
	case string:
		return strings.Contains(val, "project_root")
	case map[string]any:
		for _, v2 := range val {
			if valueReferencesProjectRoot(v2) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(val, valueReferencesProjectRoot)
	}
	return false
}

// detectProjectRoot returns the git repository root, or empty string if not in a git repo.
func detectProjectRoot() string {
	out, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return out
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
		switch {
		case evalCtx.TranscriptLoader != nil:
			loader := evalCtx.TranscriptLoader
			m["transcript"] = func() ref.Val {
				transcript, err := loader()
				if err != nil {
					return types.NewErrFromString(err.Error())
				}
				return types.DefaultTypeAdapter.NativeToValue(transcript)
			}
		case evalCtx.Transcript != nil:
			m["transcript"] = evalCtx.Transcript
		default:
			m["transcript"] = []any{}
		}
	} else {
		m["ctx"] = map[string]any{}
		m["transcript"] = []any{}
	}
	return m
}
