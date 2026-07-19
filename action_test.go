package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
			"reason":   map[string]any{"cel": `event.tool_input.command + " is not allowed"`},
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

func TestExecActionRespondTyped(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		Respond: map[string]any{
			"hookSpecificOutput": map[string]any{
				"updatedInput": map[string]any{"cel": `{"command": "echo safe", "timeout": 5}`},
			},
		},
	}

	var buf strings.Builder
	if err := ExecAction(env, action, map[string]any{}, nil, &buf); err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	updated := result["hookSpecificOutput"].(map[string]any)["updatedInput"].(map[string]any)
	if updated["command"] != "echo safe" {
		t.Errorf("updatedInput.command = %v", updated["command"])
	}
	if updated["timeout"] != float64(5) {
		t.Errorf("updatedInput.timeout = %v (%T), want 5", updated["timeout"], updated["timeout"])
	}
}

func TestExecActionCommand(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		action *Action
		event  map[string]any
		want   string
	}{
		{
			name:   "shell form literal",
			action: &Action{Command: "echo hello"},
			event:  map[string]any{},
			want:   "hello",
		},
		{
			name: "shell form with env indirection",
			action: &Action{
				Command: `echo "$FILE"`,
				Env:     map[string]string{"FILE": "event.tool_input.file_path"},
			},
			event: map[string]any{"tool_input": map[string]any{"file_path": "/tmp/main.go"}},
			want:  "/tmp/main.go",
		},
		{
			name: "argv form with expression node",
			action: &Action{
				Command: []any{"echo", map[string]any{"cel": "event.tool_name"}},
			},
			event: map[string]any{"tool_name": "Bash"},
			want:  "Bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			if err := ExecAction(env, tt.action, tt.event, nil, &buf); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(buf.String()) != tt.want {
				t.Errorf("output = %q, want %q", buf.String(), tt.want)
			}
		})
	}
}

// Shell metacharacters in event values must never execute, in either command form.
func TestExecActionCommandInjection(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(t.TempDir(), "pwned")
	payload := fmt.Sprintf(`x"; touch %s; echo "y`, marker)
	event := map[string]any{"tool_input": map[string]any{"file_path": payload}}

	tests := []struct {
		name   string
		action *Action
	}{
		{
			name: "shell form",
			action: &Action{
				Command: `echo "$FILE"`,
				Env:     map[string]string{"FILE": "event.tool_input.file_path"},
			},
		},
		{
			name: "argv form",
			action: &Action{
				Command: []any{"echo", map[string]any{"cel": "event.tool_input.file_path"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			if err := ExecAction(env, tt.action, event, nil, &buf); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(buf.String()) != payload {
				t.Errorf("output = %q, want the payload echoed verbatim", buf.String())
			}
			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("injection executed: marker file %s was created", marker)
			}
		})
	}
}

func TestExecActionCommandStdin(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		action *Action
		event  map[string]any
		want   string
	}{
		{
			name:   "literal stdin",
			action: &Action{Command: "cat", Stdin: "hello from stdin"},
			event:  map[string]any{},
			want:   "hello from stdin",
		},
		{
			name:   "expression stdin",
			action: &Action{Command: "cat", Stdin: map[string]any{"cel": `"tool: " + event.tool_name`}},
			event:  map[string]any{"tool_name": "Bash"},
			want:   "tool: Bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			if err := ExecAction(env, tt.action, tt.event, nil, &buf); err != nil {
				t.Fatal(err)
			}
			if buf.String() != tt.want {
				t.Errorf("output = %q, want %q", buf.String(), tt.want)
			}
		})
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
			URL:    server.URL,
			Method: "POST",
			Headers: map[string]any{
				"X-Custom": "test-value",
				"X-Tool":   map[string]any{"cel": "event.tool_name"},
			},
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

	// Verify literal and expression headers
	if receivedHeaders.Get("X-Custom") != "test-value" {
		t.Errorf("X-Custom header = %q", receivedHeaders.Get("X-Custom"))
	}
	if receivedHeaders.Get("X-Tool") != "Bash" {
		t.Errorf("X-Tool header = %q", receivedHeaders.Get("X-Tool"))
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
			Headers: map[string]any{"Authorization": "Bearer token"},
			Timeout: "5s",
		},
	}
	event := map[string]any{}

	var buf strings.Builder
	if err := DryRunAction(env, action, event, nil, &buf); err != nil {
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

func TestDryRunAction_CommandEnv(t *testing.T) {
	env, err := NewCELEnv()
	if err != nil {
		t.Fatal(err)
	}

	action := &Action{
		Command: `gofmt -w "$FILE"`,
		Env:     map[string]string{"FILE": "event.tool_input.file_path"},
	}
	event := map[string]any{"tool_input": map[string]any{"file_path": "/tmp/main.go"}}

	var buf strings.Builder
	if err := DryRunAction(env, action, event, nil, &buf); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, `[dry-run] command: gofmt -w "$FILE"`) {
		t.Errorf("output = %q, want literal command", output)
	}
	if !strings.Contains(output, "[dry-run] env: FILE=/tmp/main.go") {
		t.Errorf("output = %q, want resolved env", output)
	}
}
