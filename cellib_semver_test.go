package main

import (
	"testing"
)

func TestSemverCompare(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"gte match", `semver_compare(">=1.21", "1.22.0")`, true},
		{"gte exact", `semver_compare(">=1.21", "1.21.0")`, true},
		{"gte fail", `semver_compare(">=1.21", "1.20.0")`, false},
		{"range match", `semver_compare(">=1.0, <2.0", "1.5.3")`, true},
		{"range fail", `semver_compare(">=1.0, <2.0", "2.0.0")`, false},
		{"exact match", `semver_compare("1.2.3", "1.2.3")`, true},
		{"exact fail", `semver_compare("1.2.3", "1.2.4")`, false},
		{"tilde", `semver_compare("~1.2.0", "1.2.5")`, true},
		{"caret", `semver_compare("^1.2.0", "1.9.0")`, true},
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
