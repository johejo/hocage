package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/cel-go/cel"
)

var interpolateRe = regexp.MustCompile(`\{\{(.+?)\}\}`)

// Interpolate replaces {{expr}} placeholders in s with CEL evaluation results.
func Interpolate(env *cel.Env, s string, event any, evalCtx *EvalContext) (string, error) {
	var lastErr error
	result := interpolateRe.ReplaceAllStringFunc(s, func(match string) string {
		expr := extractExpr(match)
		prg, err := CompileCEL(env, expr)
		if err != nil {
			lastErr = err
			return match
		}
		out, _, err := prg.Eval(NewActivation(event, evalCtx))
		if err != nil {
			lastErr = err
			return match
		}
		return fmt.Sprintf("%v", out.Value())
	})
	if lastErr != nil {
		return "", lastErr
	}
	return result, nil
}

// InterpolateValue recursively interpolates string values in an arbitrary object.
func InterpolateValue(env *cel.Env, v any, event any, evalCtx *EvalContext) (any, error) {
	return walkValue(v, func(s string) (string, error) {
		return Interpolate(env, s, event, evalCtx)
	})
}

// walkValue recursively walks an arbitrary JSON-like object and applies fn to every string value.
func walkValue(v any, fn func(string) (string, error)) (any, error) {
	switch val := v.(type) {
	case string:
		return fn(val)
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v2 := range val {
			walked, err := walkValue(v2, fn)
			if err != nil {
				return nil, err
			}
			result[k] = walked
		}
		return result, nil
	case []any:
		result := make([]any, len(val))
		for i, v2 := range val {
			walked, err := walkValue(v2, fn)
			if err != nil {
				return nil, err
			}
			result[i] = walked
		}
		return result, nil
	default:
		return v, nil
	}
}

// extractExpr extracts the CEL expression from a {{expr}} match string.
func extractExpr(match string) string {
	return strings.TrimSpace(match[2 : len(match)-2])
}
