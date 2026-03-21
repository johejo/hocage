package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGlobExists(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte("package foo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bar.go"), []byte("package bar"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"matching pattern", `glob_exists("` + dir + `/*.go")`, true},
		{"no match", `glob_exists("` + dir + `/*.proto")`, false},
		{"single file match", `glob_exists("` + dir + `/readme.md")`, true},
		{"nonexistent dir", `glob_exists("/nonexistent_dir_xyz/*.go")`, false},
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
