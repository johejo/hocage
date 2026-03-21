package main

import (
	"testing"
)

func TestToJSON(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"string", `to_json("hello")`, `"hello"`},
		{"int", `to_json(42)`, `42`},
		{"bool", `to_json(true)`, `true`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got := mustEval(t, prg, map[string]any{})
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToJSONMap(t *testing.T) {
	env := mustNewCELEnv(t)
	event := map[string]any{
		"data": map[string]any{"key": "value"},
	}
	prg := mustCompile(t, env, `to_json(event.data)`)
	got := mustEval(t, prg, event)
	if got != `{"key":"value"}` {
		t.Errorf("got %v", got)
	}
}

func TestFromJSON(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"object key", `from_json("{\"a\":1}").a == 1.0`, true},
		{"array size", `from_json("[1,2,3]").size() == 3`, true},
		{"string", `from_json("\"hello\"") == "hello"`, true},
		{"nested", `from_json("{\"x\":{\"y\":2}}").x.y == 2.0`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got, err := EvalCELBool(prg, map[string]any{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToJSONCELMapLiteral(t *testing.T) {
	env := mustNewCELEnv(t)
	prg := mustCompile(t, env, `from_json(to_json({"a": 1})).a == 1.0`)
	got, err := EvalCELBool(prg, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true")
	}
}

func TestFromJSONToJSON(t *testing.T) {
	env := mustNewCELEnv(t)
	prg := mustCompile(t, env, `to_json(from_json("{\"a\":1}"))`)
	got := mustEval(t, prg, map[string]any{})
	if got != `{"a":1}` {
		t.Errorf("got %v", got)
	}
}
