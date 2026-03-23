package main

import (
	"testing"
)

func TestCELEvalBool(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		expr  string
		event any
		want  bool
	}{
		{
			expr:  `event.tool_input.command.contains("rm -rf")`,
			event: map[string]any{"tool_input": map[string]any{"command": "rm -rf /"}},
			want:  true,
		},
		{
			expr:  `event.tool_input.command.contains("rm -rf")`,
			event: map[string]any{"tool_input": map[string]any{"command": "ls -la"}},
			want:  false,
		},
		{
			expr:  `event.tool_input.file_path.endsWith(".go")`,
			event: map[string]any{"tool_input": map[string]any{"file_path": "/tmp/main.go"}},
			want:  true,
		},
		{
			expr:  `event.prompt.contains("deploy")`,
			event: map[string]any{"prompt": "please deploy to production"},
			want:  true,
		},
		{
			expr:  `true`,
			event: map[string]any{},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			prg, err := CompileCEL(env, tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			got, err := EvalCELBool(prg, tt.event, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCELCompileError(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}
	_, err = CompileCEL(env, "invalid syntax !!!")
	if err == nil {
		t.Fatal("expected compile error")
	}
}

func TestTwoVarComprehensions(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
	}{
		{
			"transformList with index",
			`["a", "b", "c"].transformList(i, v, string(i) + ":" + v) == ["0:a", "1:b", "2:c"]`,
		},
		{
			"transformList with filter",
			`[10, 20, 30, 40].transformList(i, v, i % 2 == 0, v) == [10, 30]`,
		},
		{
			"transformMap on map",
			`{"a": 1, "b": 2}.transformMap(k, v, v * 10) == {"a": 10, "b": 20}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			out, _, err := prg.Eval(NewActivation(map[string]any{}, nil))
			if err != nil {
				t.Fatal(err)
			}
			if out.Value() != true {
				t.Errorf("expected true, got %v", out.Value())
			}
		})
	}
}

func TestTwoVarComprehensionsWithEvent(t *testing.T) {
	env := mustNewCELEnv(t)
	expr := `event.files.transformList(i, v, string(i) + ":" + v) == ["0:main.go", "1:util.go"]`
	prg := mustCompile(t, env, expr)
	event := map[string]any{"files": []any{"main.go", "util.go"}}
	got, err := EvalCELBool(prg, event, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true")
	}
}
