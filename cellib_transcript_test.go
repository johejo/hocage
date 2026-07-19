package main

import (
	"slices"
	"testing"
)

// evalTranscriptBool compiles expr and evaluates it against the given
// transcript entries, expecting a bool result.
func evalTranscriptBool(t *testing.T, expr string, transcript []any) bool {
	t.Helper()
	env := mustNewCELEnv(t)
	prg := mustCompile(t, env, expr)
	out, _, err := prg.Eval(NewActivation(map[string]any{}, &EvalContext{Transcript: transcript}))
	if err != nil {
		t.Fatal(err)
	}
	b, ok := out.Value().(bool)
	if !ok {
		t.Fatalf("expression %q returned %T, want bool", expr, out.Value())
	}
	return b
}

func mustLoadRealSession(t *testing.T) []any {
	t.Helper()
	entries, err := LoadTranscriptFile("testdata/real_session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

// TestToolCallsRealSession runs tool_calls/user_messages against a fixture in
// the real Claude Code transcript shape (v2.1.207): tool_use blocks inside
// assistant message.content, tool_result + toolUseResult on user entries, and
// interleaved non-message lines (mode, file-history-snapshot, queue-operation).
func TestToolCallsRealSession(t *testing.T) {
	entries := mustLoadRealSession(t)
	reversed := slices.Clone(entries)
	slices.Reverse(reversed)

	tests := map[string]struct {
		expr    string
		reverse bool
		want    bool
	}{
		"two tool calls": {
			expr: `size(tool_calls(transcript)) == 2`, want: true,
		},
		"first call is the go test Bash call": {
			expr: `tool_calls(transcript)[0].name == "Bash" && tool_calls(transcript)[0].input.command == "go test ./..."`, want: true,
		},
		"result joined from toolUseResult": {
			expr: `tool_calls(transcript)[0].result.stdout.startsWith("ok") && tool_calls(transcript)[0].result.is_error == false`, want: true,
		},
		"failed call has is_error and tool_result content": {
			expr: `tool_calls(transcript)[1].result.is_error && tool_calls(transcript)[1].result.content.contains("Permission denied")`, want: true,
		},
		"dangerous command detectable": {
			expr: `tool_calls(transcript).exists(c, c.name == "Bash" && c.input.command.contains("rm -rf"))`, want: true,
		},
		"user_messages: string and text-block content, meta excluded": {
			expr: `user_messages(transcript) == ["please run the tests", "now delete the scratch dir"]`, want: true,
		},
		"reverse order: most recent call first": {
			expr: `tool_calls(transcript)[0].input.command.contains("rm -rf")`, reverse: true, want: true,
		},
		"reverse order: result still joined across entries": {
			expr: `tool_calls(transcript)[0].result.is_error`, reverse: true, want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tr := entries
			if tt.reverse {
				tr = reversed
			}
			if got := evalTranscriptBool(t, tt.expr, tr); got != tt.want {
				t.Errorf("%s = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestToolCallsEdgeCases(t *testing.T) {
	tests := map[string]struct {
		transcript []any
		expr       string
		want       bool
	}{
		"empty transcript": {
			transcript: []any{},
			expr:       `size(tool_calls(transcript)) == 0 && size(user_messages(transcript)) == 0`,
			want:       true,
		},
		"non-map and shapeless entries are skipped": {
			transcript: []any{
				"garbage",
				float64(42),
				map[string]any{"type": "mode", "mode": "normal"},
				map[string]any{"type": "assistant"},
				map[string]any{"type": "assistant", "message": map[string]any{"content": "plain text"}},
			},
			expr: `size(tool_calls(transcript)) == 0`,
			want: true,
		},
		"tool_use without result has no result key": {
			transcript: []any{
				map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
					map[string]any{"type": "tool_use", "id": "toolu_1", "name": "Read", "input": map[string]any{"file_path": "a.txt"}},
				}}},
			},
			expr: `size(tool_calls(transcript)) == 1 && !has(tool_calls(transcript)[0].result)`,
			want: true,
		},
		"tool_use without input gets empty map": {
			transcript: []any{
				map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
					map[string]any{"type": "tool_use", "id": "toolu_1", "name": "Read"},
				}}},
			},
			expr: `size(tool_calls(transcript)[0].input) == 0`,
			want: true,
		},
		"tool_result without toolUseResult still joins": {
			transcript: []any{
				map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
					map[string]any{"type": "tool_use", "id": "toolu_1", "name": "Bash", "input": map[string]any{"command": "ls"}},
				}}},
				map[string]any{"type": "user", "message": map[string]any{"content": []any{
					map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": "a.txt"},
				}}},
			},
			expr: `tool_calls(transcript)[0].result.content == "a.txt" && tool_calls(transcript)[0].result.is_error == false`,
			want: true,
		},
		"user message with empty content excluded": {
			transcript: []any{
				map[string]any{"type": "user", "message": map[string]any{"content": ""}},
				map[string]any{"type": "user", "message": map[string]any{"content": "hello"}},
			},
			expr: `user_messages(transcript) == ["hello"]`,
			want: true,
		},
		"multiple text blocks joined with newline": {
			transcript: []any{
				map[string]any{"type": "user", "message": map[string]any{"content": []any{
					map[string]any{"type": "text", "text": "line1"},
					map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": "ignored"},
					map[string]any{"type": "text", "text": "line2"},
				}}},
			},
			expr: `user_messages(transcript) == ["line1\nline2"]`,
			want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := evalTranscriptBool(t, tt.expr, tt.transcript); got != tt.want {
				t.Errorf("%s = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}
