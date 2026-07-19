package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"github.com/google/cel-go/cel"
	"google.golang.org/protobuf/types/known/structpb"
)

// legacyInterpolateRe matches leftover v1 `{{expr}}` interpolation in literal strings.
var legacyInterpolateRe = regexp.MustCompile(`(?s)\{\{.+?\}\}`)

// exprNode reports whether v is an expression node — a mapping with exactly
// one key "cel" — and returns its expression. A non-string "cel" value is an error.
func exprNode(v any) (expr string, ok bool, err error) {
	m, isMap := v.(map[string]any)
	if !isMap || len(m) != 1 {
		return "", false, nil
	}
	raw, hasKey := m["cel"]
	if !hasKey {
		return "", false, nil
	}
	s, isStr := raw.(string)
	if !isStr {
		return "", false, fmt.Errorf("cel expression must be a string, got %T", raw)
	}
	return s, true, nil
}

var structpbValueType = reflect.TypeFor[*structpb.Value]()

// evalExprTyped evaluates a CEL expression and converts the result to a
// JSON-compatible Go value (string, float64, bool, nil, []any, map[string]any).
func evalExprTyped(env *cel.Env, expr string, event any, evalCtx *EvalContext) (any, error) {
	prg, err := CompileCEL(env, expr)
	if err != nil {
		return nil, fmt.Errorf("expression %q: %w", expr, err)
	}
	out, _, err := prg.Eval(NewActivation(event, evalCtx))
	if err != nil {
		return nil, fmt.Errorf("expression %q: %w", expr, err)
	}
	native, err := out.ConvertToNative(structpbValueType)
	if err != nil {
		return nil, fmt.Errorf("expression %q: convert result to JSON: %w", expr, err)
	}
	return native.(*structpb.Value).AsInterface(), nil
}

// evalExprString evaluates a CEL expression for a string slot. Scalars are
// coerced to their JSON representation; null, lists, and maps are rejected.
func evalExprString(env *cel.Env, expr string, event any, evalCtx *EvalContext) (string, error) {
	v, err := evalExprTyped(env, expr, event, evalCtx)
	if err != nil {
		return "", err
	}
	switch val := v.(type) {
	case string:
		return val, nil
	case bool, float64:
		data, err := json.Marshal(val)
		if err != nil {
			return "", fmt.Errorf("expression %q: %w", expr, err)
		}
		return string(data), nil
	default:
		return "", fmt.Errorf("expression %q: result must be a string, got %T (use to_json(...))", expr, v)
	}
}

// ResolveValue recursively replaces every {cel: "<expr>"} node in a JSON-like
// value with its typed evaluation result. Plain strings are always literal.
func ResolveValue(env *cel.Env, v any, event any, evalCtx *EvalContext) (any, error) {
	if expr, ok, err := exprNode(v); err != nil {
		return nil, err
	} else if ok {
		return evalExprTyped(env, expr, event, evalCtx)
	}
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v2 := range val {
			resolved, err := ResolveValue(env, v2, event, evalCtx)
			if err != nil {
				return nil, err
			}
			result[k] = resolved
		}
		return result, nil
	case []any:
		result := make([]any, len(val))
		for i, v2 := range val {
			resolved, err := ResolveValue(env, v2, event, evalCtx)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	default:
		return v, nil
	}
}

// ResolveStringSlot resolves a config slot that must yield a string: either a
// literal string or a single {cel: "<expr>"} node.
func ResolveStringSlot(env *cel.Env, v any, event any, evalCtx *EvalContext) (string, error) {
	if expr, ok, err := exprNode(v); err != nil {
		return "", err
	} else if ok {
		return evalExprString(env, expr, event, evalCtx)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("must be a string or {cel: ...} node, got %T", v)
	}
	return s, nil
}
