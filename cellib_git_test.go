package main

import (
	"testing"
)

func TestGitTracked(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"tracked file", `git_tracked("go.mod")`, true},
		{"untracked file", `git_tracked("nonexistent_file_xyz.txt")`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg, err := CompileCEL(env, tt.expr)
			if err != nil {
				t.Fatal(err)
			}
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
