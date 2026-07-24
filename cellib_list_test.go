package main

import (
	"testing"
)

func TestMin(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want any
	}{
		{"int list", `min([3, 1, 2])`, int64(1)},
		{"single element", `min([42])`, int64(42)},
		{"double list", `min([3.5, 1.2, 2.8])`, 1.2},
		{"string list", `min(["c", "a", "b"])`, "a"},
		{"mixed numeric list", `min([2, 1.5])`, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got := mustEval(t, prg, map[string]any{})
			if got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestMax(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want any
	}{
		{"int list", `max([3, 1, 2])`, int64(3)},
		{"single element", `max([42])`, int64(42)},
		{"double list", `max([3.5, 1.2, 2.8])`, 3.5},
		{"string list", `max(["c", "a", "b"])`, "c"},
		{"mixed numeric list", `max([2, 2.5])`, 2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got := mustEval(t, prg, map[string]any{})
			if got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestMinMaxErrors(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
	}{
		{"min empty list", `min([])`},
		{"max empty list", `max([])`},
		{"min string then int", `min(["a", 1])`},
		{"min int then string", `min([1, "a"])`},
		{"max string then int", `max(["a", 1])`},
		{"max int then string", `max([1, "a"])`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			_, _, err := prg.Eval(NewActivation(map[string]any{}, nil))
			if err == nil {
				t.Errorf("expected error for %s", tt.expr)
			}
		})
	}
}
