package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	if err := Generate(cfg, "agcel", &buf); err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}

	hooks, ok := result["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks not found in output")
	}

	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		t.Fatal("PreToolUse not found")
	}

	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 PreToolUse entry, got %d", len(preToolUse))
	}

	entry := preToolUse[0].(map[string]any)
	if entry["matcher"] != "Bash" {
		t.Errorf("matcher = %v", entry["matcher"])
	}

	hooksList := entry["hooks"].([]any)
	if len(hooksList) != 1 {
		t.Fatalf("expected 1 hook entry, got %d", len(hooksList))
	}

	hookEntry := hooksList[0].(map[string]any)
	if hookEntry["type"] != "command" {
		t.Errorf("type = %v", hookEntry["type"])
	}
	if hookEntry["command"] != "agcel hooks run block_rm_rf" {
		t.Errorf("command = %v", hookEntry["command"])
	}
}
