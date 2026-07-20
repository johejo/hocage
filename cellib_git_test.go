package main

import (
	"os"
	"os/exec"
	"testing"
)

func initGitRepo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	runGit(t, "init", "-q", "-b", "main")
	if err := os.WriteFile("tracked.txt", []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".gitignore", []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, "add", ".")
	runGit(t, "commit", "-q", "-m", "init")
}

func runGit(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestGitFunctions(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		setup func(t *testing.T)
		expr  string
		want  bool
	}{
		{"tracked file", nil, `git_tracked("tracked.txt")`, true},
		{"untracked file", nil, `git_tracked("nonexistent.txt")`, false},
		{"branch", nil, `git_branch() == "main"`, true},
		{
			"ignored file",
			func(t *testing.T) {
				if err := os.WriteFile("ignored.txt", []byte("x\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			`git_ignored("ignored.txt")`, true,
		},
		{"tracked not ignored", nil, `git_ignored("tracked.txt")`, false},
		{"clean file", nil, `git_modified("tracked.txt")`, false},
		{
			"modified file",
			func(t *testing.T) {
				if err := os.WriteFile("tracked.txt", []byte("changed\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			`git_modified("tracked.txt")`, true,
		},
		{"nothing staged", nil, `git_staged("tracked.txt")`, false},
		{
			"staged change",
			func(t *testing.T) {
				if err := os.WriteFile("tracked.txt", []byte("changed\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				runGit(t, "add", "tracked.txt")
			},
			`git_staged("tracked.txt")`, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initGitRepo(t)
			if tt.setup != nil {
				tt.setup(t)
			}
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
