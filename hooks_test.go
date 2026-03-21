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
		if err := RunHook(cfg, "rewrite_command", input, &buf, false); err != nil {
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
		if err := RunHook(cfg, "rewrite_command", input, &buf, false); err != nil {
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
		if err := RunHook(cfg, "block_rm_rf", input, &buf, false); err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
		}
		if result["decision"] != "block" {
			t.Errorf("decision = %v", result["decision"])
		}
	})

	t.Run("no match", func(t *testing.T) {
		input := strings.NewReader(`{"tool_input":{"command":"ls -la"}}`)
		var buf strings.Builder
		if err := RunHook(cfg, "block_rm_rf", input, &buf, false); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "" {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})

	t.Run("hook not found", func(t *testing.T) {
		input := strings.NewReader(`{}`)
		var buf strings.Builder
		err := RunHook(cfg, "nonexistent", input, &buf, false)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRunHookDryRun_Match(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	input := strings.NewReader(`{"tool_input":{"command":"rm -rf /"}}`)
	var buf strings.Builder
	if err := RunHook(cfg, "block_rm_rf", input, &buf, true); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "[dry-run] respond:") {
		t.Errorf("expected dry-run respond output, got %q", output)
	}
	if !strings.Contains(output, "block") {
		t.Errorf("expected 'block' in output, got %q", output)
	}
}

func TestRunHookDryRun_NoMatch(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	input := strings.NewReader(`{"tool_input":{"command":"ls"}}`)
	var buf strings.Builder
	if err := RunHook(cfg, "block_rm_rf", input, &buf, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[dry-run] not matched") {
		t.Errorf("expected dry-run not matched, got %q", buf.String())
	}
}

func TestRunHookDryRun_Command(t *testing.T) {
	cfg, err := LoadConfig("testdata/stdin_command.yaml")
	if err != nil {
		t.Fatal(err)
	}

	input := strings.NewReader(`{"tool_name":"bash","tool_input":{"command":"echo hi"}}`)
	var buf strings.Builder
	if err := RunHook(cfg, "pipe_event", input, &buf, true); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "[dry-run] command:") {
		t.Errorf("expected dry-run command output, got %q", output)
	}
	if !strings.Contains(output, "[dry-run] stdin:") {
		t.Errorf("expected dry-run stdin output, got %q", output)
	}
}
