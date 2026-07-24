package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTranscriptJSONL(t *testing.T) {
	r := strings.NewReader(`{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hi"}]},"uuid":"a1"}

{"type":"user","message":{"role":"user","content":"bye"},"uuid":"u2"}
`)
	result, err := ParseTranscriptJSONL(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	first, ok := result[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result[0])
	}
	if first["type"] != "user" {
		t.Errorf("expected type=user, got %v", first["type"])
	}
}

func TestParseTranscriptJSONL_Empty(t *testing.T) {
	result, err := ParseTranscriptJSONL(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result))
	}
}

func TestParseTranscriptJSONL_InvalidJSON(t *testing.T) {
	r := strings.NewReader(`{"valid":true}
not json
{"also":"valid"}
`)
	_, err := ParseTranscriptJSONL(r)
	if err == nil {
		t.Fatal("expected error for invalid JSON line")
	}
	if got := err.Error(); got != "line 2: invalid character 'o' in literal null (expecting 'u')" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestLoadTranscriptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	content := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"world"}]},"uuid":"a1"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := LoadTranscriptFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
}

func TestLoadTranscriptFile_NotFound(t *testing.T) {
	_, err := LoadTranscriptFile("/nonexistent/path.jsonl")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadTranscriptFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := LoadTranscriptFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result))
	}
}

func TestFindTailOffset_SmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.jsonl")
	content := `{"a":1}
{"a":2}
{"a":3}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Request more lines than exist — should return offset 0.
	offset, err := findTailOffset(f, int64(len(content)), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
}

func TestLoadTranscriptFile_ManyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.jsonl")

	// Create a file with 200 lines, each ~100 bytes, totaling ~20KB.
	var b strings.Builder
	for i := range 200 {
		fmt.Fprintf(&b, `{"line":%d,"padding":"%s"}`, i, strings.Repeat("x", 60))
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}

	// Request tail 10 lines.
	result, err := LoadTranscriptFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 200 {
		t.Fatalf("expected 200, got %d", len(result))
	}

	// Verify chronological order (first entry should be line 0).
	first := result[0].(map[string]any)
	if first["line"] != float64(0) {
		t.Errorf("expected first line=0, got %v", first["line"])
	}
	last := result[len(result)-1].(map[string]any)
	if last["line"] != float64(199) {
		t.Errorf("expected last line=199, got %v", last["line"])
	}
}

func TestFindTailOffset_MultiChunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multichunk.jsonl")

	// 2000 lines, each ~90 bytes, totaling ~180KB — spans multiple 64KB chunks.
	var b strings.Builder
	lineStarts := make([]int64, 2000)
	for i := range 2000 {
		lineStarts[i] = int64(b.Len())
		fmt.Fprintf(&b, `{"line":%d,"padding":"%s"}`, i, strings.Repeat("x", 60))
		b.WriteByte('\n')
	}
	content := b.String()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tests := []struct {
		name string
		n    int
		want int64
	}{
		{"tail within last chunk", 10, lineStarts[1990]},
		{"tail spans multiple chunks", 1000, lineStarts[1000]},
		{"n equals total lines", 2000, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, err := findTailOffset(f, int64(len(content)), tt.n)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if offset != tt.want {
				t.Errorf("expected offset %d, got %d", tt.want, offset)
			}
			wantPrefix := fmt.Sprintf(`{"line":%d,`, 2000-tt.n)
			if !strings.HasPrefix(content[offset:], wantPrefix) {
				t.Errorf("expected content at offset to start with %q, got %q", wantPrefix, content[offset:offset+20])
			}
		})
	}
}

func TestRunHookWithTranscript(t *testing.T) {
	// Create a transcript file.
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")
	transcriptContent := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"rm -rf /"}}]},"uuid":"a1"}
`
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig("testdata/transcript_hook.yaml")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("match with transcript_path", func(t *testing.T) {
		input := strings.NewReader(fmt.Sprintf(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hello"},"transcript_path":%q}`, transcriptPath))
		var buf strings.Builder
		if err := RunHook(cfg, "transcript_file_test", input, &buf, &buf, false); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "deny") {
			t.Errorf("expected deny, got %q", buf.String())
		}
	})

	t.Run("missing transcript_path", func(t *testing.T) {
		input := strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hello"}}`)
		var buf strings.Builder
		err := RunHook(cfg, "transcript_file_test", input, &buf, &buf, false)
		if err == nil {
			t.Fatal("expected error for missing transcript_path")
		}
		if !strings.Contains(err.Error(), "transcript_path") {
			t.Errorf("error = %q, want to mention transcript_path", err.Error())
		}
	})
}

// transcript_path points at a nonexistent file, which would error if read eagerly.
func TestRunHookTranscriptNotLoadedOnShortCircuit(t *testing.T) {
	cfg, err := LoadConfig("testdata/transcript_hook.yaml")
	if err != nil {
		t.Fatal(err)
	}

	input := strings.NewReader(fmt.Sprintf(
		`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hello"},"transcript_path":%q}`,
		"/nonexistent/transcript.jsonl",
	))
	var buf strings.Builder
	if err := RunHook(cfg, "short_circuit_no_load", input, &buf, &buf, false); err != nil {
		t.Fatalf("expected no error (transcript should not be loaded), got: %v", err)
	}
	if strings.Contains(buf.String(), "deny") {
		t.Errorf("expected no match, got %q", buf.String())
	}
}
