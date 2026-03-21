package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunHook(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("match", func(t *testing.T) {
		input := strings.NewReader(`{"tool_input":{"command":"rm -rf /"}}`)
		var buf strings.Builder
		if err := RunHook(cfg, "block_rm_rf", input, &buf); err != nil {
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
		if err := RunHook(cfg, "block_rm_rf", input, &buf); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "" {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})

	t.Run("hook not found", func(t *testing.T) {
		input := strings.NewReader(`{}`)
		var buf strings.Builder
		err := RunHook(cfg, "nonexistent", input, &buf)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
