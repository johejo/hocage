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
			got, err := EvalCELBool(prg, tt.event)
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
