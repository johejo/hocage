package main

import (
	"bytes"
	"os"
	"testing"
)

// TestGeneratedDocsUpToDate fails when a committed doc file differs from what
// the generators produce from the current source (gofmt-check style).
func TestGeneratedDocsUpToDate(t *testing.T) {
	for _, tgt := range genTargets() {
		current, err := os.ReadFile(tgt.Path)
		if err != nil {
			t.Fatalf("read %s: %v", tgt.Path, err)
		}
		want, err := tgt.Render(current)
		if err != nil {
			t.Fatalf("render %s: %v", tgt.Path, err)
		}
		if !bytes.Equal(current, want) {
			t.Errorf("%s is stale; run `go generate ./...`", tgt.Path)
		}
	}
}
