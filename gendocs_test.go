package main

import (
	"bytes"
	"testing"
)

// TestGeneratedDocsUpToDate fails when a committed doc file differs from what
// the generators produce from the current source (gofmt-check style).
func TestGeneratedDocsUpToDate(t *testing.T) {
	for _, tgt := range genTargets() {
		current, want, err := renderTarget(tgt)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(current, want) {
			t.Errorf("%s is stale; run `go generate ./...`", tgt.Path)
		}
	}
}
