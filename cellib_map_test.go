package main

import (
	"testing"
)

func TestKeys(t *testing.T) {
	env := mustNewCELEnv(t)
	event := map[string]any{
		"tool_input": map[string]any{
			"command":   "ls",
			"file_path": "/tmp/foo",
		},
	}
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"size", `keys(event.tool_input).size() == 2`, true},
		{"sorted first", `keys(event.tool_input)[0] == "command"`, true},
		{"sorted second", `keys(event.tool_input)[1] == "file_path"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got, err := EvalCELBool(prg, event, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValues(t *testing.T) {
	env := mustNewCELEnv(t)
	event := map[string]any{
		"data": map[string]any{
			"a": "x",
			"b": "y",
		},
	}
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"size", `values(event.data).size() == 2`, true},
		{"sorted by key first", `values(event.data)[0] == "x"`, true},
		{"sorted by key second", `values(event.data)[1] == "y"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got, err := EvalCELBool(prg, event, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToEntries(t *testing.T) {
	env := mustNewCELEnv(t)
	event := map[string]any{
		"data": map[string]any{
			"name": "test",
		},
	}
	prg := mustCompile(t, env, `to_entries(event.data).size() == 1`)
	got, err := EvalCELBool(prg, event, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true")
	}
}

func TestFromEntries(t *testing.T) {
	env := mustNewCELEnv(t)
	event := map[string]any{
		"data": map[string]any{
			"a": "x",
			"b": "y",
		},
	}
	// round-trip: from_entries(to_entries(m)) should give back same keys
	prg := mustCompile(t, env, `keys(from_entries(to_entries(event.data))) == keys(event.data)`)
	got, err := EvalCELBool(prg, event, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true")
	}
}

func TestToEntriesNested(t *testing.T) {
	env := mustNewCELEnv(t)
	event := map[string]any{
		"data": map[string]any{
			"nested": map[string]any{"x": 1},
		},
	}
	// Verify to_entries works with nested maps and the entry structure is accessible
	prg := mustCompile(t, env, `to_entries(event.data)[0].key == "nested"`)
	got, err := EvalCELBool(prg, event, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true")
	}
}

func TestHasKey(t *testing.T) {
	env := mustNewCELEnv(t)
	event := map[string]any{
		"tool_input": map[string]any{
			"command":   "ls",
			"file_path": "/tmp/foo",
		},
	}
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"exists", `has_key(event.tool_input, "command")`, true},
		{"not exists", `has_key(event.tool_input, "nonexistent")`, false},
		{"empty key", `has_key(event.tool_input, "")`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got, err := EvalCELBool(prg, event, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeysWithExists(t *testing.T) {
	env := mustNewCELEnv(t)
	event := map[string]any{
		"tool_input": map[string]any{
			"command": "rm -rf /",
		},
	}
	prg := mustCompile(t, env, `keys(event.tool_input).exists(k, k == "command")`)
	got, err := EvalCELBool(prg, event, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true")
	}
}
