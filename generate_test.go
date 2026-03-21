package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateMerged(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	existingJSON := []byte(`{
  "permissions": {"allow": ["Bash(npm test)"]},
  "hooks": {"old": "data"}
}`)

	var buf strings.Builder
	if err := GenerateMerged(cfg, "agcel", existingJSON, &buf); err != nil {
		t.Fatal(err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}

	// hooks key should exist with generated content
	if _, ok := result["hooks"]; !ok {
		t.Fatal("hooks key missing from merged output")
	}

	// permissions key should be preserved
	permRaw, ok := result["permissions"]
	if !ok {
		t.Fatal("permissions key lost during merge")
	}
	var perm map[string]any
	if err := json.Unmarshal(permRaw, &perm); err != nil {
		t.Fatalf("unmarshal permissions: %v", err)
	}
	allow, ok := perm["allow"].([]any)
	if !ok || len(allow) != 1 || allow[0] != "Bash(npm test)" {
		t.Errorf("permissions not preserved: %v", perm)
	}

	// Verify hooks content is from generation, not old data
	var hooks map[string]any
	if err := json.Unmarshal(result["hooks"], &hooks); err != nil {
		t.Fatalf("unmarshal hooks: %v", err)
	}
	if _, ok := hooks["old"]; ok {
		t.Error("old hooks data should have been replaced")
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("generated PreToolUse hooks missing")
	}
}

func TestGenerateMerged_EmptyExisting(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	if err := GenerateMerged(cfg, "agcel", []byte(`{}`), &buf); err != nil {
		t.Fatal(err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}

	if _, ok := result["hooks"]; !ok {
		t.Fatal("hooks key missing from merged output")
	}
}

func TestGenerateMerged_NoExistingHooks(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	existingJSON := []byte(`{"permissions": {"allow": []}, "model": "sonnet"}`)

	var buf strings.Builder
	if err := GenerateMerged(cfg, "agcel", existingJSON, &buf); err != nil {
		t.Fatal(err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}

	// All keys should be present
	for _, key := range []string{"permissions", "model", "hooks"} {
		if _, ok := result[key]; !ok {
			t.Errorf("key %q missing from merged output", key)
		}
	}
}

func TestGenerateMerged_InvalidJSON(t *testing.T) {
	cfg, err := LoadConfig("testdata/block_rm_rf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	err = GenerateMerged(cfg, "agcel", []byte(`not json`), &buf)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse existing JSON") {
		t.Errorf("unexpected error message: %v", err)
	}
}

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

func TestGenerate_PriorityOrder(t *testing.T) {
	cfg, err := LoadConfig("testdata/priority_hooks.yaml")
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

	hooks := result["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)
	entry := preToolUse[0].(map[string]any)
	hooksList := entry["hooks"].([]any)

	if len(hooksList) != 3 {
		t.Fatalf("expected 3 hook entries, got %d", len(hooksList))
	}

	// Expected order: default_priority_hook (0), high_priority_hook (1), low_priority_hook (10)
	expected := []string{
		"agcel hooks run default_priority_hook",
		"agcel hooks run high_priority_hook",
		"agcel hooks run low_priority_hook",
	}
	for i, he := range hooksList {
		cmd := he.(map[string]any)["command"].(string)
		if cmd != expected[i] {
			t.Errorf("hooksList[%d].command = %q, want %q", i, cmd, expected[i])
		}
	}
}
