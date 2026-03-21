package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"existing file", `file_exists("` + filePath + `")`, true},
		{"nonexistent file", `file_exists("` + filepath.Join(dir, "nope.txt") + `")`, false},
		{"directory is not a file", `file_exists("` + dir + `")`, false},
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

func TestDirExists(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"existing dir", `dir_exists("` + dir + `")`, true},
		{"nonexistent dir", `dir_exists("` + filepath.Join(dir, "nope") + `")`, false},
		{"file is not a dir", `dir_exists("` + filePath + `")`, false},
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
