package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// maxTranscriptLines is the maximum number of non-empty JSONL lines to parse.
// This prevents memory explosion from extremely large transcript files.
const maxTranscriptLines = 100_000

// maxTranscriptLineSize is the maximum size of a single JSONL line in bytes.
// Claude Code transcript lines can be very large (e.g. tool outputs with full file contents).
const maxTranscriptLineSize = 10 * 1024 * 1024 // 10MB

// ParseTranscriptJSONL parses JSONL from a reader into a slice of arbitrary JSON values.
// If there are more than maxTranscriptLines non-empty lines, only the last
// maxTranscriptLines are returned (tail behavior), since recent history is
// typically more relevant for policy evaluation.
func ParseTranscriptJSONL(r io.Reader) ([]any, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTranscriptLineSize)

	var result []any
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		result = append(result, v)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read transcript: %w", err)
	}

	// Tail: keep only the last maxTranscriptLines entries.
	// This is the fallback for io.Reader where we can't seek.
	// For files, findTailOffset already limits the read range.
	if len(result) > maxTranscriptLines {
		result = result[len(result)-maxTranscriptLines:]
	}
	return result, nil
}

// LoadTranscriptFile reads a JSONL file, efficiently reading from the tail
// for large files. Returns entries in chronological order.
func LoadTranscriptFile(path string) ([]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open transcript file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat transcript file: %w", err)
	}
	if stat.Size() == 0 {
		return []any{}, nil
	}

	// Seek to the approximate start of the last maxTranscriptLines lines.
	offset, err := findTailOffset(f, stat.Size(), maxTranscriptLines)
	if err != nil {
		return nil, fmt.Errorf("find tail offset: %w", err)
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek transcript file: %w", err)
	}

	return ParseTranscriptJSONL(f)
}

// findTailOffset scans backwards from the end of file to find the byte offset
// where the last n lines (by newline count) begin. Empty lines are counted as
// lines, so for files with many empty lines the result may contain slightly
// fewer than n non-empty entries.
func findTailOffset(f *os.File, size int64, n int) (int64, error) {
	const chunkSize = 64 * 1024
	buf := make([]byte, chunkSize)
	nlCount := 0
	pos := size

	for pos > 0 {
		readSize := min(int64(chunkSize), pos)
		pos -= readSize

		b := buf[:readSize]
		if _, err := f.ReadAt(b, pos); err != nil && err != io.EOF {
			return 0, fmt.Errorf("read transcript file at offset %d: %w", pos, err)
		}

		for i := len(b) - 1; i >= 0; i-- {
			if b[i] == '\n' {
				nlCount++
				if nlCount > n {
					return pos + int64(i) + 1, nil
				}
			}
		}
	}
	return 0, nil
}
