package main

import (
	"testing"
)

func TestInterpolate(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	event := map[string]any{
		"tool_input": map[string]any{
			"command":   "rm -rf /",
			"file_path": "/tmp/main.go",
		},
	}

	tests := []struct {
		input string
		want  string
	}{
		{"no interpolation", "no interpolation"},
		{"{{event.tool_input.command}} is bad", "rm -rf / is bad"},
		{"gofmt -w {{event.tool_input.file_path}}", "gofmt -w /tmp/main.go"},
		{"{{event.tool_input.command}} and {{event.tool_input.file_path}}", "rm -rf / and /tmp/main.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Interpolate(env, tt.input, event)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInterpolateValue(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	event := map[string]any{
		"tool_input": map[string]any{"command": "rm -rf /"},
	}

	input := map[string]any{
		"decision": "block",
		"reason":   "{{event.tool_input.command}} is not allowed",
		"count":    float64(42),
	}

	result, err := InterpolateValue(env, input, event)
	if err != nil {
		t.Fatal(err)
	}

	m := result.(map[string]any)
	if m["reason"] != "rm -rf / is not allowed" {
		t.Errorf("reason = %q", m["reason"])
	}
	if m["decision"] != "block" {
		t.Errorf("decision = %q", m["decision"])
	}
	if m["count"] != float64(42) {
		t.Errorf("count = %v", m["count"])
	}
}
