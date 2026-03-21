package main

import (
	"fmt"

	"github.com/google/cel-go/cel"
)

func NewCELEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("event", cel.DynType),
	)
}

func CompileCEL(env *cel.Env, expr string) (cel.Program, error) {
	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile CEL: %w", issues.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("program CEL: %w", err)
	}
	return prg, nil
}

func EvalCELBool(prg cel.Program, event any) (bool, error) {
	out, _, err := prg.Eval(map[string]any{
		"event": event,
	})
	if err != nil {
		return false, fmt.Errorf("eval CEL: %w", err)
	}
	b, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression must return bool, got %T", out.Value())
	}
	return b, nil
}
