package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestExecActionHTTP(t *testing.T) {
	var receivedBody string
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		receivedHeaders = r.Header
		fmt.Fprint(w, `{"decision":"allow"}`)
	}))
	defer server.Close()

	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		HTTP: &HTTPAction{
			URL:     server.URL,
			Method:  "POST",
			Headers: map[string]string{"X-Custom": "test-value"},
			Timeout: "5s",
		},
	}
	event := map[string]any{"tool_name": "Bash"}

	var buf strings.Builder
	if err := ExecAction(env, action, event, nil, &buf); err != nil {
		t.Fatal(err)
	}

	if buf.String() != `{"decision":"allow"}` {
		t.Errorf("output = %q", buf.String())
	}

	// Verify request body contains the event
	if !strings.Contains(receivedBody, `"tool_name"`) {
		t.Errorf("request body = %q, want tool_name", receivedBody)
	}

	// Verify custom header
	if receivedHeaders.Get("X-Custom") != "test-value" {
		t.Errorf("X-Custom header = %q", receivedHeaders.Get("X-Custom"))
	}

	// Verify content type
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", receivedHeaders.Get("Content-Type"))
	}
}

func TestExecActionHTTP_DefaultMethod(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		fmt.Fprint(w, "{}")
	}))
	defer server.Close()

	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		HTTP: &HTTPAction{URL: server.URL},
	}
	event := map[string]any{}

	var buf strings.Builder
	if err := ExecAction(env, action, event, nil, &buf); err != nil {
		t.Fatal(err)
	}

	if receivedMethod != "POST" {
		t.Errorf("method = %q, want POST", receivedMethod)
	}
}

func TestExecActionHTTP_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer server.Close()

	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		HTTP: &HTTPAction{URL: server.URL},
	}
	event := map[string]any{}

	var buf strings.Builder
	err = ExecAction(env, action, event, nil, &buf)
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("error = %q, want status 500", err.Error())
	}
}

func TestDryRunAction_HTTP(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		HTTP: &HTTPAction{
			URL:     "http://localhost:8080/hooks",
			Method:  "POST",
			Headers: map[string]string{"Authorization": "Bearer token"},
			Timeout: "5s",
		},
	}
	event := map[string]any{}

	var buf strings.Builder
	if err := DryRunAction(env, action, "PostToolUse", event, nil, &buf); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "[dry-run] http: POST http://localhost:8080/hooks") {
		t.Errorf("output = %q, want http dry-run info", output)
	}
	if !strings.Contains(output, "timeout: 5s") {
		t.Errorf("output = %q, want timeout info", output)
	}
	if !strings.Contains(output, "Authorization") {
		t.Errorf("output = %q, want header info", output)
	}
}
