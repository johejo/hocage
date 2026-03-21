package main

import (
	"testing"
)

func TestTrimPrefix(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"basic", `trim_prefix("/home/user/file.go", "/home/user/")`, "file.go"},
		{"no match", `trim_prefix("hello", "xyz")`, "hello"},
		{"empty prefix", `trim_prefix("hello", "")`, "hello"},
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

func TestTrimSuffix(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"basic", `trim_suffix("file.go", ".go")`, "file"},
		{"no match", `trim_suffix("hello", ".go")`, "hello"},
		{"empty suffix", `trim_suffix("hello", "")`, "hello"},
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

func TestPathBase(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"basic", `path_base("/a/b/c.go")`, "c.go"},
		{"root", `path_base("/")`, "/"},
		{"no dir", `path_base("file.go")`, "file.go"},
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

func TestPathDir(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"basic", `path_dir("/a/b/c.go")`, "/a/b"},
		{"root file", `path_dir("/file.go")`, "/"},
		{"no dir", `path_dir("file.go")`, "."},
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

func TestPathExt(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"go file", `path_ext("main.go")`, ".go"},
		{"no ext", `path_ext("Makefile")`, ""},
		{"double ext", `path_ext("archive.tar.gz")`, ".gz"},
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
