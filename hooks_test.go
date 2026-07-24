package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunHookUpdatedInput(t *testing.T) {
	cfg, err := LoadConfig("testdata/updated_input.yaml")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("match", func(t *testing.T) {
		input := strings.NewReader(`{"tool_input":{"command":"rm -rf /tmp"}}`)
		var buf strings.Builder
		if err := RunHook(cfg, "rewrite_command", input, &buf, &buf, false); err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
		}
		hso, ok := result["hookSpecificOutput"].(map[string]any)
		if !ok {
			t.Fatalf("hookSpecificOutput not found or not a map: %v", result)
		}
		if hso["permissionDecision"] != "allow" {
			t.Errorf("permissionDecision = %v, want allow", hso["permissionDecision"])
		}
		updatedInput, ok := hso["updatedInput"].(map[string]any)
		if !ok {
			t.Fatalf("updatedInput not found or not a map: %v", hso)
		}
		wantCmd := "echo 'rm -rf /tmp' was blocked"
		if updatedInput["command"] != wantCmd {
			t.Errorf("updatedInput.command = %v, want %v", updatedInput["command"], wantCmd)
		}
	})

	t.Run("no match", func(t *testing.T) {
		input := strings.NewReader(`{"tool_input":{"command":"ls -la"}}`)
		var buf strings.Builder
		if err := RunHook(cfg, "rewrite_command", input, &buf, &buf, false); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "" {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})
}

func TestRunHook(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("match", func(t *testing.T) {
		input := strings.NewReader(`{"tool_input":{"command":"rm -rf /"}}`)
		var buf strings.Builder
		if err := RunHook(cfg, "block_rm_rf", input, &buf, &buf, false); err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
		}
		hso, ok := result["hookSpecificOutput"].(map[string]any)
		if !ok {
			t.Fatalf("hookSpecificOutput not found or not a map: %v", result)
		}
		if hso["permissionDecision"] != "deny" {
			t.Errorf("permissionDecision = %v, want deny", hso["permissionDecision"])
		}
	})

	t.Run("no match", func(t *testing.T) {
		input := strings.NewReader(`{"tool_input":{"command":"ls -la"}}`)
		var buf strings.Builder
		if err := RunHook(cfg, "block_rm_rf", input, &buf, &buf, false); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "" {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})

	t.Run("hook not found", func(t *testing.T) {
		input := strings.NewReader(`{}`)
		var buf strings.Builder
		err := RunHook(cfg, "nonexistent", input, &buf, &buf, false)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRunHookDryRun(t *testing.T) {
	tests := []struct {
		name         string
		configPath   string
		hookName     string
		input        string
		wantContains []string
	}{
		{
			name:         "match",
			configPath:   "testdata/block_rm_rf.yaml",
			hookName:     "block_rm_rf",
			input:        `{"tool_input":{"command":"rm -rf /"}}`,
			wantContains: []string{"[dry-run] respond:", "deny"},
		},
		{
			name:         "no match",
			configPath:   "testdata/block_rm_rf.yaml",
			hookName:     "block_rm_rf",
			input:        `{"tool_input":{"command":"ls"}}`,
			wantContains: []string{"[dry-run] not matched"},
		},
		{
			name:         "command",
			configPath:   "testdata/stdin_command.yaml",
			hookName:     "pipe_event",
			input:        `{"tool_name":"bash","tool_input":{"command":"echo hi"}}`,
			wantContains: []string{"[dry-run] command:", "[dry-run] stdin:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(tt.configPath)
			if err != nil {
				t.Fatal(err)
			}

			input := strings.NewReader(tt.input)
			var buf strings.Builder
			if err := RunHook(cfg, tt.hookName, input, &buf, &buf, true); err != nil {
				t.Fatal(err)
			}
			output := buf.String()
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output = %q, want to contain %q", output, want)
				}
			}
		})
	}
}
