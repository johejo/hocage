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

func TestLoadConfigs(t *testing.T) {
	tests := []struct {
		name          string
		paths         []string
		wantHookCount int
		wantHookNames []string
	}{
		{
			name: "merge",
			paths: []string{
				"testdata/merge_base.yaml",
				"testdata/merge_override.yaml",
			},
			wantHookCount: 3,
			wantHookNames: []string{"base_hook", "override_hook"},
		},
		{
			name:          "glob pattern",
			paths:         []string{"testdata/merge_glob/*.yaml"},
			wantHookCount: 2,
			wantHookNames: []string{"glob_hook_a", "glob_hook_b"},
		},
		{
			name: "mixed literal and glob",
			paths: []string{
				"testdata/merge_base.yaml",
				"testdata/merge_glob/*.yaml",
			},
			wantHookCount: 4,
			wantHookNames: []string{"base_hook", "glob_hook_b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfigs(tt.paths)
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Hooks) != tt.wantHookCount {
				t.Fatalf("hooks count = %d, want %d", len(cfg.Hooks), tt.wantHookCount)
			}
			for _, name := range tt.wantHookNames {
				if _, ok := cfg.Hooks[name]; !ok {
					t.Errorf("%s not found", name)
				}
			}
		})
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

func TestLoadConfig_NewEventTypes(t *testing.T) {
	tests := []struct {
		path      string
		hookName  string
		eventName string
	}{
		{"testdata/session_start.yaml", "log_session_start", "SessionStart"},
		{"testdata/permission_request.yaml", "allow_read_tools", "PermissionRequest"},
		{"testdata/subagent_start.yaml", "subagent_context", "SubagentStart"},
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

func TestLoadConfig_UnknownEventName(t *testing.T) {
	cfg, err := LoadConfig("testdata/unknown_event.yaml")
	if err != nil {
		t.Fatalf("unknown event_name should load without error, got %v", err)
	}
	hook, ok := cfg.Hooks["future_hook"]
	if !ok {
		t.Fatal("hook future_hook not found")
	}
	if validEventNames[hook.EventName] {
		t.Errorf("event %q should not be in validEventNames (fixture must use an unknown name)", hook.EventName)
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

func TestDefaultConfigPatterns(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, xdgDir string) (cwdDir string)
		want  func(xdgDir string) []string
	}{
		{
			name: "XDG env",
			setup: func(t *testing.T, xdgDir string) string {
				hocageDir := filepath.Join(xdgDir, "hocage")
				if err := os.MkdirAll(hocageDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(hocageDir, "test.yaml"), []byte("hooks: {}"), 0o644); err != nil {
					t.Fatal(err)
				}
				return t.TempDir()
			},
			want: func(xdgDir string) []string {
				return []string{filepath.Join(xdgDir, "hocage", "*.yaml")}
			},
		},
		{
			name: "no files",
			setup: func(t *testing.T, xdgDir string) string {
				return t.TempDir()
			},
			want: func(xdgDir string) []string {
				return nil
			},
		},
		{
			name: "cwd only",
			setup: func(t *testing.T, xdgDir string) string {
				cwdDir := t.TempDir()
				if err := os.WriteFile(filepath.Join(cwdDir, ".hocage.yaml"), []byte("hooks:\n  h:\n    event_name: Stop\n    when: \"true\"\n    action:\n      respond: {}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return cwdDir
			},
			want: func(xdgDir string) []string {
				return []string{".hocage.yaml"}
			},
		},
		{
			name: "both",
			setup: func(t *testing.T, xdgDir string) string {
				hocageDir := filepath.Join(xdgDir, "hocage")
				if err := os.MkdirAll(hocageDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(hocageDir, "global.yaml"), []byte("hooks: {}"), 0o644); err != nil {
					t.Fatal(err)
				}
				cwdDir := t.TempDir()
				if err := os.WriteFile(filepath.Join(cwdDir, ".hocage.yaml"), []byte("hooks: {}"), 0o644); err != nil {
					t.Fatal(err)
				}
				return cwdDir
			},
			want: func(xdgDir string) []string {
				return []string{filepath.Join(xdgDir, "hocage", "*.yaml"), ".hocage.yaml"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xdgDir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", xdgDir)

			cwdDir := tt.setup(t, xdgDir)
			orig, _ := os.Getwd()
			if err := os.Chdir(cwdDir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chdir(orig) })

			patterns, err := DefaultConfigPatterns()
			if err != nil {
				t.Fatal(err)
			}
			want := tt.want(xdgDir)
			if len(patterns) != len(want) {
				t.Fatalf("patterns = %v, want %v", patterns, want)
			}
			for i, p := range patterns {
				if p != want[i] {
					t.Errorf("patterns[%d] = %q, want %q", i, p, want[i])
				}
			}
		})
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tests := []struct {
		path    string
		wantErr string
	}{
		{"testdata/invalid_no_action.yaml", "exactly one of respond, command, or http"},
		{"testdata/invalid_both_actions.yaml", "exactly one of respond, command, or http"},
		{"testdata/invalid_stdin_respond.yaml", "stdin requires command action"},
		{"testdata/invalid_http_no_url.yaml", "http action requires url"},
		{"testdata/invalid_http_and_command.yaml", "exactly one of respond, command, or http"},
		{"testdata/invalid_transcript_both.yaml", "transcript and transcript_file are mutually exclusive"},
		{"testdata/invalid_transcript_no_load.yaml", "transcript requires transcript.load: true"},
		{"testdata/invalid_transcript_order.yaml", "invalid transcript order"},
		{"testdata/invalid_legacy_interpolation.yaml", "legacy interpolation"},
		{"testdata/invalid_env_without_command.yaml", "env requires command action"},
		{"testdata/invalid_env_name.yaml", "invalid env name"},
		{"testdata/invalid_cel_node.yaml", "cel expression must be a string"},
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
