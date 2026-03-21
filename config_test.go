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
