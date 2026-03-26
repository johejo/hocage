package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTranscriptJSONL(t *testing.T) {
	r := strings.NewReader(`{"role":"user","message":"hello"}
{"role":"assistant","message":"hi"}

{"role":"user","message":"bye"}
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
	if first["role"] != "user" {
		t.Errorf("expected role=user, got %v", first["role"])
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
	content := `{"type":"user","text":"hello"}
{"type":"assistant","text":"world"}
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

func TestFindTailOffset_LargerThanChunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.jsonl")

	// Create a file with 200 lines, each ~100 bytes, totaling ~20KB.
	// This is smaller than chunkSize (64KB) so it tests the single-chunk path.
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

func TestRunHookWithTranscript(t *testing.T) {
	// Create a transcript file.
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")
	transcriptContent := `{"type":"tool_use","tool":"Bash","input":{"command":"rm -rf /"}}
`
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig("testdata/transcript_hook.yaml")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("match with transcript_path", func(t *testing.T) {
		input := strings.NewReader(fmt.Sprintf(`{"tool_name":"Bash","input":{"command":"echo hello"},"transcript_path":%q}`, transcriptPath))
		var buf strings.Builder
		if err := RunHook(cfg, "transcript_file_test", input, &buf, false); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "block") {
			t.Errorf("expected block, got %q", buf.String())
		}
	})

	t.Run("missing transcript_path", func(t *testing.T) {
		input := strings.NewReader(`{"tool_name":"Bash","input":{"command":"echo hello"}}`)
		var buf strings.Builder
		err := RunHook(cfg, "transcript_file_test", input, &buf, false)
		if err == nil {
			t.Fatal("expected error for missing transcript_path")
		}
		if !strings.Contains(err.Error(), "transcript_path") {
			t.Errorf("error = %q, want to mention transcript_path", err.Error())
		}
	})
}
