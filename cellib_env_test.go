package main

import (
	"testing"
)

func TestEnv(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOCAGE_TEST_VAR", "hello")

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"existing var", `env("HOCAGE_TEST_VAR") == "hello"`, true},
		{"nonexistent var is empty", `env("HOCAGE_NONEXISTENT_VAR_XYZ") == ""`, true},
		{"comparison with value", `env("HOCAGE_TEST_VAR") != ""`, true},
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
