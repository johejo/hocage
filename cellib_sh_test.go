package main

import (
	"testing"
)

func TestShCommands(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want any
	}{
		{"single", `sh_commands("rm -rf /")`, []string{"rm"}},
		{"pipeline", `sh_commands("a | b | c")`, []string{"a", "b", "c"}},
		{"and list", `sh_commands("git push && rm -rf /")`, []string{"git", "rm"}},
		{"quoted arg is not a command", `sh_commands("echo \"rm -rf\"")`, []string{"echo"}},
		{"command substitution", `sh_commands("x $(rm y)")`, []string{"x", "rm"}},
		// sudo rm is a single simple command: sudo is the program, rm its argument.
		{"argument is not a command", `sh_commands("sudo   rm   -rf /tmp")`, []string{"sudo"}},
		{"empty", `sh_commands("")`, []string{}},
		{"unparsable yields empty", `sh_commands("foo (")`, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got := mustEval(t, prg, map[string]any{})
			assertStringList(t, got, tt.want.([]string))
		})
	}
}

func TestShCommandsContains(t *testing.T) {
	env := mustNewCELEnv(t)
	// The motivating use case: reliable membership check immune to substring noise.
	// sh_commands matches the directly-invoked program; sh_words catches a program
	// anywhere in the command (e.g. behind sudo/xargs) while still ignoring quoted text.
	tests := []struct {
		expr string
		want bool
	}{
		{`"rm" in sh_commands("rm -rf /tmp")`, true},
		{`"rm" in sh_commands("echo \"rm -rf /\"")`, false},
		{`"rm" in sh_commands("ls -la")`, false},
		{`"rm" in sh_words("sudo  rm  -rf /tmp")`, true},
		{`"rm" in sh_words("xargs rm -rf")`, true},
		{`"rm" in sh_words("echo \"rm -rf /\"")`, false},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			if got := mustEval(t, prg, map[string]any{}); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShWords(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want []string
	}{
		{"simple", `sh_words("rm -rf /")`, []string{"rm", "-rf", "/"}},
		{"quotes stripped", `sh_words("echo \"rm -rf\"")`, []string{"echo", "rm -rf"}},
		{"single quotes", `sh_words("echo 'a b'")`, []string{"echo", "a b"}},
		{"empty", `sh_words("")`, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got := mustEval(t, prg, map[string]any{})
			assertStringList(t, got, tt.want)
		})
	}
}

func TestShValid(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		expr string
		want bool
	}{
		{`sh_valid("rm -rf /")`, true},
		{`sh_valid("a | b && c")`, true},
		{`sh_valid("")`, true},
		{`sh_valid("foo (")`, false},
		{`sh_valid("echo 'unterminated")`, false},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			if got := mustEval(t, prg, map[string]any{}); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func assertStringList(t *testing.T, got any, want []string) {
	t.Helper()
	gotList, ok := got.([]string)
	if !ok {
		t.Fatalf("got %v (%T), want []string", got, got)
	}
	if len(gotList) != len(want) {
		t.Fatalf("got %v, want %v", gotList, want)
	}
	for i := range want {
		if gotList[i] != want[i] {
			t.Fatalf("got %v, want %v", gotList, want)
		}
	}
}
