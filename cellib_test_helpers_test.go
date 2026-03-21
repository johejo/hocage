package main

import (
	"testing"

	"github.com/google/cel-go/cel"
)

func mustNewCELEnv(t *testing.T) *cel.Env {
	t.Helper()
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func mustCompile(t *testing.T, env *cel.Env, expr string) cel.Program {
	t.Helper()
	prg, err := CompileCEL(env, expr)
	if err != nil {
		t.Fatal(err)
	}
	return prg
}

func mustEval(t *testing.T, prg cel.Program, event any) any {
	t.Helper()
	out, _, err := prg.Eval(NewActivation(event, nil))
	if err != nil {
		t.Fatal(err)
	}
	return out.Value()
}
