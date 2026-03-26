package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}
	hook, ok := cfg.Hooks["block_rm_rf"]
	if !ok {
		t.Fatal("hook block_rm_rf not found")
	}
	if hook.EventName != "PreToolUse" {
		t.Errorf("event_name = %q, want PreToolUse", hook.EventName)
	}
	if hook.Matcher != "Bash" {
		t.Errorf("matcher = %q, want Bash", hook.Matcher)
	}
	if hook.Action.Respond == nil {
		t.Error("respond should not be nil")
	}
	if len(hook.Tests) != 2 {
		t.Errorf("tests count = %d, want 2", len(hook.Tests))
	}
}

func TestLoadConfigs_Merge(t *testing.T) {
	cfg, err := LoadConfigs([]string{
		"testdata/merge_base.yaml",
		"testdata/merge_override.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Hooks) != 3 {
		t.Fatalf("hooks count = %d, want 3", len(cfg.Hooks))
	}
	if _, ok := cfg.Hooks["base_hook"]; !ok {
		t.Error("base_hook not found")
	}
	if _, ok := cfg.Hooks["override_hook"]; !ok {
		t.Error("override_hook not found")
	}
}

func TestLoadConfigs_OverrideSameHook(t *testing.T) {
	cfg, err := LoadConfigs([]string{
		"testdata/merge_base.yaml",
		"testdata/merge_override.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	hook, ok := cfg.Hooks["shared_hook"]
	if !ok {
		t.Fatal("shared_hook not found")
	}
	resp, ok := hook.Action.Respond.(map[string]any)
	if !ok {
		t.Fatal("respond is not a map")
	}
	if reason, _ := resp["reason"].(string); reason != "from override" {
		t.Errorf("reason = %q, want %q", reason, "from override")
	}
}

func TestLoadConfigs_GlobPattern(t *testing.T) {
	cfg, err := LoadConfigs([]string{"testdata/merge_glob/*.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Hooks) != 2 {
		t.Fatalf("hooks count = %d, want 2", len(cfg.Hooks))
	}
	if _, ok := cfg.Hooks["glob_hook_a"]; !ok {
		t.Error("glob_hook_a not found")
	}
	if _, ok := cfg.Hooks["glob_hook_b"]; !ok {
		t.Error("glob_hook_b not found")
	}
}

func TestLoadConfigs_NoMatchFallsBackToLiteral(t *testing.T) {
	_, err := LoadConfigs([]string{"testdata/nonexistent.yaml"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "reading config") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "reading config")
	}
}

func TestLoadConfigs_GlobNoMatch(t *testing.T) {
	_, err := LoadConfigs([]string{"testdata/no_such_dir/*.yaml"})
	if err == nil {
		t.Fatal("expected error for glob with no matches")
	}
	if !strings.Contains(err.Error(), "no config files matched pattern") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "no config files matched pattern")
	}
}

func TestLoadConfigs_MixedLiteralAndGlob(t *testing.T) {
	cfg, err := LoadConfigs([]string{
		"testdata/merge_base.yaml",
		"testdata/merge_glob/*.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	// base_hook + shared_hook from merge_base, glob_hook_a + glob_hook_b from glob
	if len(cfg.Hooks) != 4 {
		t.Fatalf("hooks count = %d, want 4", len(cfg.Hooks))
	}
	if _, ok := cfg.Hooks["base_hook"]; !ok {
		t.Error("base_hook not found")
	}
	if _, ok := cfg.Hooks["glob_hook_b"]; !ok {
		t.Error("glob_hook_b not found")
	}
}

func TestLoadConfig_NewEventTypes(t *testing.T) {
	tests := []struct {
		path      string
		hookName  string
		eventName string
	}{
		{"testdata/session_start.yaml", "log_session_start", "SessionStart"},
		{"testdata/permission_request.yaml", "allow_read_tools", "PermissionRequest"},
		{"testdata/subagent_start.yaml", "restrict_subagent_model", "SubagentStart"},
		{"testdata/post_tool_use_failure.yaml", "log_tool_failure", "PostToolUseFailure"},
		{"testdata/task_completed.yaml", "on_task_completed", "TaskCompleted"},
		{"testdata/config_change.yaml", "block_config_change", "ConfigChange"},
		{"testdata/pre_compact.yaml", "log_compact", "PreCompact"},
	}
	for _, tt := range tests {
		t.Run(tt.eventName, func(t *testing.T) {
			cfg, err := LoadConfig(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			hook, ok := cfg.Hooks[tt.hookName]
			if !ok {
				t.Fatalf("hook %q not found", tt.hookName)
			}
			if hook.EventName != tt.eventName {
				t.Errorf("event_name = %q, want %q", hook.EventName, tt.eventName)
			}
		})
	}
}

func TestValidEventNames_AllNewTypes(t *testing.T) {
	newEvents := []string{
		"SessionStart", "SessionEnd", "PermissionRequest",
		"SubagentStart", "PostToolUseFailure", "StopFailure",
		"PreCompact", "PostCompact", "TaskCompleted",
		"InstructionsLoaded", "ConfigChange", "Elicitation",
		"ElicitationResult", "TeammateIdle", "WorktreeCreate",
		"WorktreeRemove",
	}
	for _, name := range newEvents {
		if !validEventNames[name] {
			t.Errorf("event %q should be in validEventNames", name)
		}
	}
}

func TestLoadConfig_HTTPAction(t *testing.T) {
	cfg, err := LoadConfig("testdata/http_hook.yaml")
	if err != nil {
		t.Fatal(err)
	}
	hook, ok := cfg.Hooks["notify_webhook"]
	if !ok {
		t.Fatal("hook notify_webhook not found")
	}
	if hook.Action.HTTP == nil {
		t.Fatal("http action should not be nil")
	}
	if hook.Action.HTTP.URL != "http://localhost:8080/hooks" {
		t.Errorf("url = %q", hook.Action.HTTP.URL)
	}
	if hook.Action.HTTP.Method != "POST" {
		t.Errorf("method = %q", hook.Action.HTTP.Method)
	}
	if hook.Action.HTTP.Timeout != "5s" {
		t.Errorf("timeout = %q", hook.Action.HTTP.Timeout)
	}
	if hook.Action.HTTP.Headers["Authorization"] != "Bearer test-token" {
		t.Errorf("authorization header = %q", hook.Action.HTTP.Headers["Authorization"])
	}
}

func TestDefaultConfigPatterns_XDGEnv(t *testing.T) {
	dir := t.TempDir()
	xdgDir := filepath.Join(dir, "hocage")
	if err := os.MkdirAll(xdgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdgDir, "test.yaml"), []byte("hooks: {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Change to a directory without .hocage.yaml
	orig, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	patterns, err := DefaultConfigPatterns()
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 1 {
		t.Fatalf("patterns = %v, want 1 element", patterns)
	}
	want := filepath.Join(dir, "hocage", "*.yaml")
	if patterns[0] != want {
		t.Errorf("pattern = %q, want %q", patterns[0], want)
	}
}

func TestDefaultConfigPatterns_NoFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	orig, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	patterns, err := DefaultConfigPatterns()
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 0 {
		t.Fatalf("patterns = %v, want empty", patterns)
	}
}

func TestDefaultConfigPatterns_CWDOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cwdDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwdDir, ".hocage.yaml"), []byte("hooks:\n  h:\n    event_name: Stop\n    when: \"true\"\n    action:\n      respond: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	patterns, err := DefaultConfigPatterns()
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 1 {
		t.Fatalf("patterns = %v, want 1 element", patterns)
	}
	if patterns[0] != ".hocage.yaml" {
		t.Errorf("pattern = %q, want .hocage.yaml", patterns[0])
	}
}

func TestDefaultConfigPatterns_Both(t *testing.T) {
	dir := t.TempDir()
	xdgDir := filepath.Join(dir, "hocage")
	if err := os.MkdirAll(xdgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdgDir, "global.yaml"), []byte("hooks: {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	cwdDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwdDir, ".hocage.yaml"), []byte("hooks: {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	patterns, err := DefaultConfigPatterns()
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 2 {
		t.Fatalf("patterns = %v, want 2 elements", patterns)
	}
	wantXDG := filepath.Join(dir, "hocage", "*.yaml")
	if patterns[0] != wantXDG {
		t.Errorf("patterns[0] = %q, want %q", patterns[0], wantXDG)
	}
	if patterns[1] != ".hocage.yaml" {
		t.Errorf("patterns[1] = %q, want %q", patterns[1], ".hocage.yaml")
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tests := []struct {
		path    string
		wantErr string
	}{
		{"testdata/invalid_no_action.yaml", "exactly one of respond, command, or http"},
		{"testdata/invalid_both_actions.yaml", "exactly one of respond, command, or http"},
		{"testdata/invalid_event.yaml", "invalid event_name"},
		{"testdata/invalid_stdin_respond.yaml", "stdin requires command action"},
		{"testdata/invalid_http_no_url.yaml", "http action requires url"},
		{"testdata/invalid_http_and_command.yaml", "exactly one of respond, command, or http"},
		{"testdata/invalid_transcript_both.yaml", "transcript and transcript_file are mutually exclusive"},
		{"testdata/invalid_transcript_no_load.yaml", "transcript requires transcript.load: true"},
		{"testdata/invalid_transcript_order.yaml", "invalid transcript order"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			_, err := LoadConfig(tt.path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
