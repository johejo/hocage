package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExecActionRespond(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		Respond: map[string]any{
			"decision": "block",
			"reason":   "{{event.tool_input.command}} is not allowed",
		},
	}
	event := map[string]any{
		"tool_input": map[string]any{"command": "rm -rf /"},
	}

	var buf strings.Builder
	if err := ExecAction(env, action, event, nil, &buf); err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if result["decision"] != "block" {
		t.Errorf("decision = %q", result["decision"])
	}
	if result["reason"] != "rm -rf / is not allowed" {
		t.Errorf("reason = %q", result["reason"])
	}
}

func TestExecActionCommand(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		Command: "echo hello",
	}
	event := map[string]any{}

	var buf strings.Builder
	if err := ExecAction(env, action, event, nil, &buf); err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(buf.String()) != "hello" {
		t.Errorf("output = %q, want hello", buf.String())
	}
}

func TestExecActionCommandStdin(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		Command: "cat",
		Stdin:   "hello from stdin",
	}
	event := map[string]any{}

	var buf strings.Builder
	if err := ExecAction(env, action, event, nil, &buf); err != nil {
		t.Fatal(err)
	}

	if buf.String() != "hello from stdin" {
		t.Errorf("output = %q, want %q", buf.String(), "hello from stdin")
	}
}

func TestExecActionCommandStdinInterpolation(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		Command: "cat",
		Stdin:   "tool: {{event.tool_name}}",
	}
	event := map[string]any{
		"tool_name": "Bash",
	}

	var buf strings.Builder
	if err := ExecAction(env, action, event, nil, &buf); err != nil {
		t.Fatal(err)
	}

	if buf.String() != "tool: Bash" {
		t.Errorf("output = %q, want %q", buf.String(), "tool: Bash")
	}
}

func TestExecActionCommandInterpolation(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		Command: "echo {{event.tool_input.file_path}}",
	}
	event := map[string]any{
		"tool_input": map[string]any{"file_path": "/tmp/main.go"},
	}

	var buf strings.Builder
	if err := ExecAction(env, action, event, nil, &buf); err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(buf.String()) != "/tmp/main.go" {
		t.Errorf("output = %q", buf.String())
	}
}
