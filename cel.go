package main

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
)

// baseEnvOptions returns the environment options minus HocageLibrary. The docs
// generator diffs an env built from these against the full env to enumerate
// the custom functions hocage adds.
func baseEnvOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Variable("event", cel.DynType),
		cel.Variable("ctx", cel.DynType),
		cel.Variable("transcript", cel.ListType(cel.DynType)),
		cel.OptionalTypes(),
		ext.Strings(),
		ext.Lists(ext.ListsVersion(3)),
		ext.Sets(),
		ext.Math(),
		ext.Encoders(),
		ext.Regex(),
		ext.Bindings(),
		ext.TwoVarComprehensions(),
	}
}

func NewCELEnv() (*cel.Env, error) {
	return cel.NewEnv(append(baseEnvOptions(), HocageLibrary())...)
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

func EvalCELBool(prg cel.Program, event any, evalCtx *EvalContext) (bool, error) {
	out, _, err := prg.Eval(NewActivation(event, evalCtx))
	if err != nil {
		return false, fmt.Errorf("eval CEL: %w", err)
	}
	b, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression must return bool, got %T", out.Value())
	}
	return b, nil
}
