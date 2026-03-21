package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/google/cel-go/cel"
)

// ExecAction executes the action (respond or command) and writes output to w.
func ExecAction(env *cel.Env, action *Action, event any, evalCtx *EvalContext, w io.Writer) error {
	if action.Respond != nil {
		return execRespond(env, action.Respond, event, evalCtx, w)
	}
	return execCommand(env, action, event, evalCtx, w)
}

// interpolateRespond normalises and interpolates a respond value, returning the result.
func interpolateRespond(env *cel.Env, respond any, event any, evalCtx *EvalContext) (any, error) {
	normalized, err := normalizeToJSON(respond)
	if err != nil {
		return nil, fmt.Errorf("normalize respond: %w", err)
	}
	interpolated, err := InterpolateValue(env, normalized, event, evalCtx)
	if err != nil {
		return nil, fmt.Errorf("interpolate respond: %w", err)
	}
	return interpolated, nil
}

func execRespond(env *cel.Env, respond any, event any, evalCtx *EvalContext, w io.Writer) error {
	interpolated, err := interpolateRespond(env, respond, event, evalCtx)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(interpolated)
}

// interpolateCommand interpolates command and optional stdin strings.
func interpolateCommand(env *cel.Env, action *Action, event any, evalCtx *EvalContext) (cmd string, stdin string, err error) {
	cmd, err = Interpolate(env, action.Command, event, evalCtx)
	if err != nil {
		return "", "", fmt.Errorf("interpolate command: %w", err)
	}
	if action.Stdin != "" {
		stdin, err = Interpolate(env, action.Stdin, event, evalCtx)
		if err != nil {
			return "", "", fmt.Errorf("interpolate stdin: %w", err)
		}
	}
	return cmd, stdin, nil
}

func execCommand(env *cel.Env, action *Action, event any, evalCtx *EvalContext, w io.Writer) error {
	interpolated, stdinStr, err := interpolateCommand(env, action, event, evalCtx)
	if err != nil {
		return err
	}
	cmd := exec.Command("sh", "-c", interpolated)
	cmd.Stdout = w
	cmd.Stderr = w
	if stdinStr != "" {
		cmd.Stdin = strings.NewReader(stdinStr)
	}
	return cmd.Run()
}

// DryRunAction previews the action without executing it.
func DryRunAction(env *cel.Env, action *Action, eventName string, event any, evalCtx *EvalContext, w io.Writer) error {
	if action.Respond != nil {
		interpolated, err := interpolateRespond(env, action.Respond, event, evalCtx)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(interpolated, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal respond: %w", err)
		}
		fmt.Fprintf(w, "[dry-run] respond: %s\n", data)
		if m, ok := interpolated.(map[string]any); ok {
			for _, warning := range ValidateRespondOutput(eventName, m) {
				fmt.Fprintf(w, "[dry-run] WARN: %s\n", warning)
			}
		}
		return nil
	}

	interpolated, stdinStr, err := interpolateCommand(env, action, event, evalCtx)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "[dry-run] command: %s\n", interpolated)
	if stdinStr != "" {
		fmt.Fprintf(w, "[dry-run] stdin: %s\n", stdinStr)
	}
	return nil
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
