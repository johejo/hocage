package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/google/cel-go/cel"
)

// ExecAction executes the action (respond or command) and writes output to w.
func ExecAction(env *cel.Env, action *Action, event any, evalCtx *EvalContext, w io.Writer) error {
	if action.Respond != nil {
		return execRespond(env, action.Respond, event, evalCtx, w)
	}
	return execCommand(env, action.Command, event, evalCtx, w)
}

func execRespond(env *cel.Env, respond any, event any, evalCtx *EvalContext, w io.Writer) error {
	// Convert respond to map[string]any for interpolation
	normalized, err := normalizeToJSON(respond)
	if err != nil {
		return fmt.Errorf("normalize respond: %w", err)
	}
	interpolated, err := InterpolateValue(env, normalized, event, evalCtx)
	if err != nil {
		return fmt.Errorf("interpolate respond: %w", err)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(interpolated)
}

func execCommand(env *cel.Env, command string, event any, evalCtx *EvalContext, w io.Writer) error {
	interpolated, err := Interpolate(env, command, event, evalCtx)
	if err != nil {
		return fmt.Errorf("interpolate command: %w", err)
	}
	cmd := exec.Command("sh", "-c", interpolated)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// normalizeToJSON round-trips through JSON to get consistent types (e.g. map[string]any).
func normalizeToJSON(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
