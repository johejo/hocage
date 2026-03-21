package main

import (
	"os"
	"testing"
)

func TestBuildEvalContext(t *testing.T) {
	ctx, err := BuildEvalContext()
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if ctx.CWD != cwd {
		t.Errorf("CWD = %q, want %q", ctx.CWD, cwd)
	}
	// Running inside a git repo, so ProjectRoot should be non-empty.
	if ctx.ProjectRoot == "" {
		t.Error("ProjectRoot should not be empty in a git repo")
	}
}

func TestNewActivation(t *testing.T) {
	event := map[string]any{"foo": "bar"}
	evalCtx := &EvalContext{CWD: "/tmp/test", ProjectRoot: "/tmp/project"}

	act := NewActivation(event, evalCtx)
	if act["event"] == nil {
		t.Error("event not set")
	}
	ctxMap, ok := act["ctx"].(map[string]any)
	if !ok {
		t.Fatal("ctx is not map[string]any")
	}
	if ctxMap["cwd"] != "/tmp/test" {
		t.Errorf("ctx.cwd = %q", ctxMap["cwd"])
	}
	if ctxMap["project_root"] != "/tmp/project" {
		t.Errorf("ctx.project_root = %q", ctxMap["project_root"])
	}
}

func TestCtxCWDInCEL(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}
	evalCtx := &EvalContext{CWD: "/home/user/project"}
	prg, err := CompileCEL(env, `ctx.cwd == "/home/user/project"`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := EvalCELBool(prg, map[string]any{}, evalCtx)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected ctx.cwd to match")
	}
}

func TestCtxProjectRootInCEL(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}
	evalCtx := &EvalContext{CWD: "/home/user/project/sub", ProjectRoot: "/home/user/project"}
	prg, err := CompileCEL(env, `ctx.project_root == "/home/user/project"`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := EvalCELBool(prg, map[string]any{}, evalCtx)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected ctx.project_root to match")
	}
}

func TestNewActivationNilContext(t *testing.T) {
	act := NewActivation(map[string]any{}, nil)
	ctxMap, ok := act["ctx"].(map[string]any)
	if !ok {
		t.Fatal("ctx is not map[string]any")
	}
	if len(ctxMap) != 0 {
		t.Errorf("expected empty ctx map, got %v", ctxMap)
	}
}
