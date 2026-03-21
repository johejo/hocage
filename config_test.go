package main

import (
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

func TestLoadConfigValidation(t *testing.T) {
	tests := []struct {
		path    string
		wantErr string
	}{
		{"testdata/invalid_no_action.yaml", "must have respond or command"},
		{"testdata/invalid_both_actions.yaml", "not both"},
		{"testdata/invalid_event.yaml", "invalid event_name"},
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
