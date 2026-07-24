package main

import (
	"os"
	"testing"
)

func TestBuildEvalContext(t *testing.T) {
	ctx, err := BuildEvalContext(true)
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

func TestHookReferencesProjectRoot(t *testing.T) {
	tests := map[string]struct {
		hook *Hook
		want bool
	}{
		"when references it": {
			hook: &Hook{When: `ctx.project_root == "/foo"`},
			want: true,
		},
		"command list cel node references it": {
			hook: &Hook{
				When: "true",
				Action: Action{
					Command: []any{"sh", "-c", map[string]any{"cel": "ctx.project_root"}},
				},
			},
			want: true,
		},
		"env expression references it": {
			hook: &Hook{
				When: "true",
				Action: Action{
					Env: map[string]string{"ROOT": "ctx.project_root"},
				},
			},
			want: true,
		},
		"stdin cel node references it": {
			hook: &Hook{
				When: "true",
				Action: Action{
					Stdin: map[string]any{"cel": "ctx.project_root"},
				},
			},
			want: true,
		},
		"http url references it": {
			hook: &Hook{
				When: "true",
				Action: Action{
					HTTP: &HTTPAction{URL: map[string]any{"cel": "ctx.project_root + \"/x\""}},
				},
			},
			want: true,
		},
		"http header references it": {
			hook: &Hook{
				When: "true",
				Action: Action{
					HTTP: &HTTPAction{
						URL:     "https://example.com",
						Headers: map[string]any{"X-Root": map[string]any{"cel": "ctx.project_root"}},
					},
				},
			},
			want: true,
		},
		"respond references it": {
			hook: &Hook{
				When: "true",
				Action: Action{
					Respond: map[string]any{"root": map[string]any{"cel": "ctx.project_root"}},
				},
			},
			want: true,
		},
		"no reference anywhere": {
			hook: &Hook{
				When: `ctx.cwd == "/foo"`,
				Action: Action{
					Command: []any{"echo", "hi"},
					Env:     map[string]string{"BRANCH": "git_branch()"},
					Stdin:   "hello",
				},
			},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := HookReferencesProjectRoot(tc.hook); got != tc.want {
				t.Errorf("HookReferencesProjectRoot() = %v, want %v", got, tc.want)
			}
		})
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
