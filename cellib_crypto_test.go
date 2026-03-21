package main

import (
	"testing"
)

func TestSHA256Sum(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"empty", `sha256sum("")`, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello", `sha256sum("hello")`, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
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

func TestSHA256SumDeterministic(t *testing.T) {
	env := mustNewCELEnv(t)
	prg := mustCompile(t, env, `sha256sum("test") == sha256sum("test")`)
	got, err := EvalCELBool(prg, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true")
	}
}
