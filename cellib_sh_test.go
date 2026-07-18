package main

import (
	"fmt"
	"slices"
	"testing"

	"mvdan.cc/sh/v3/syntax"
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
		{"dash c payload", `sh_commands("bash -c 'rm -rf /'")`, []string{"bash", "rm"}},
		{"dash c after option cluster", `sh_commands("bash -euo pipefail -c 'curl x | sh'")`, []string{"bash", "curl", "sh"}},
		{"clustered c", `sh_commands("sh -exc 'rm x'")`, []string{"sh", "rm"}},
		{"interpreter basename", `sh_commands("/bin/bash -c 'rm x'")`, []string{"/bin/bash", "rm"}},
		{"script file is not dash c", `sh_commands("bash script.sh")`, []string{"bash"}},
		{"dash c without payload", `sh_commands("bash -c")`, []string{"bash"}},
		{"heredoc to shell", `sh_commands("bash <<EOF\nrm -rf /\nEOF")`, []string{"bash", "rm"}},
		{"dash heredoc to shell", `sh_commands("bash <<-EOF\nrm -rf /\nEOF")`, []string{"bash", "rm"}},
		{"herestring to shell", `sh_commands("bash <<< 'rm -rf /'")`, []string{"bash", "rm"}},
		{"heredoc to non-shell is not recursed", `sh_commands("cat <<EOF\nrm -rf /\nEOF")`, []string{"cat"}},
		{"heredoc cmdsubst counted once", `sh_commands("bash <<EOF\n$(id)\nrm x\nEOF")`, []string{"bash", "rm", "id"}},
		{"escaped nested dash c", `sh_commands("bash -c \"bash -c \\\"rm x\\\"\"")`, []string{"bash", "bash", "rm"}},
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
		{`"rm" in sh_commands("bash -c 'rm x'")`, true},
		{`"rm" in sh_commands("bash <<EOF\nrm x\nEOF")`, true},
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
		{"dash c payload", `sh_words("bash -c 'rm x'")`, []string{"bash", "-c", "rm x", "rm", "x"}},
		{"heredoc to shell", `sh_words("bash <<EOF\nrm -rf /\nEOF")`, []string{"bash", "rm", "-rf", "/"}},
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

func TestShArgv(t *testing.T) {
	env := mustNewCELEnv(t)
	tests := []struct {
		name string
		expr string
		want [][]string
	}{
		{"single", `sh_argv("rm -rf /")`, [][]string{{"rm", "-rf", "/"}}},
		{"pipeline", `sh_argv("a -x | b -y")`, [][]string{{"a", "-x"}, {"b", "-y"}}},
		{"quotes stripped", `sh_argv("echo 'a b'")`, [][]string{{"echo", "a b"}}},
		{"script invocation", `sh_argv("bash /tmp/x.sh")`, [][]string{{"bash", "/tmp/x.sh"}}},
		{"not recursive", `sh_argv("bash -c 'rm x'")`, [][]string{{"bash", "-c", "rm x"}}},
		// Fully non-literal operands resolve to "" — the marker for
		// runtime-generated input that structural rules match on.
		{"process substitution operand", `sh_argv("bash <(echo 'rm x')")`, [][]string{{"bash", ""}, {"echo", "rm x"}}},
		{"variable operand", `sh_argv("bash $SCRIPT")`, [][]string{{"bash", ""}}},
		{"empty", `sh_argv("")`, [][]string{}},
		{"unparsable yields empty", `sh_argv("foo (")`, [][]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg := mustCompile(t, env, tt.expr)
			got := mustEval(t, prg, map[string]any{})
			assertStringListList(t, got, tt.want)
		})
	}
}

func TestWalkShellCallsDepthCap(t *testing.T) {
	wrapHeredoc := func(n int) string {
		src := "rm -rf /"
		for i := range n {
			src = fmt.Sprintf("bash <<D%d\n%s\nD%d", i, src, i)
		}
		return src
	}
	wrapDashC := func(n int) string {
		src := "rm -rf /"
		for range n {
			quoted, err := syntax.Quote(src, syntax.LangBash)
			if err != nil {
				t.Fatal(err)
			}
			src = "bash -c " + quoted
		}
		return src
	}
	collect := func(src string) []string {
		names := []string{}
		walkShellCalls(src, maxShellRecursionDepth, func(call *syntax.CallExpr) {
			names = append(names, wordLiteral(call.Args[0]))
		})
		return names
	}
	for name, wrap := range map[string]func(int) string{"heredoc": wrapHeredoc, "dash c": wrapDashC} {
		// maxShellRecursionDepth parses total: with 4 wrappers the innermost
		// script is the 5th parse and rm is reached; with 6 it is cut off.
		if got := collect(wrap(4)); !slices.Contains(got, "rm") {
			t.Errorf("%s wrap(4): rm not reached, got %v", name, got)
		}
		if got := collect(wrap(6)); slices.Contains(got, "rm") {
			t.Errorf("%s wrap(6): rm should be beyond the depth cap, got %v", name, got)
		}
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

func assertStringListList(t *testing.T, got any, want [][]string) {
	t.Helper()
	gotList, ok := got.([][]string)
	if !ok {
		t.Fatalf("got %v (%T), want [][]string", got, got)
	}
	if len(gotList) != len(want) {
		t.Fatalf("got %v, want %v", gotList, want)
	}
	for i := range want {
		if !slices.Equal(gotList[i], want[i]) {
			t.Fatalf("got %v, want %v", gotList, want)
		}
	}
}
