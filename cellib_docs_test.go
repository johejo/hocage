package main

import (
	"os"
	"strings"
	"testing"
)

// TestCustomCELFunctionsDocumented fails when a function registered by
// HocageLibrary is missing from the hand-written CEL reference.
func TestCustomCELFunctionsDocumented(t *testing.T) {
	names, err := customCELFunctions()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no custom CEL functions found; env diff is broken")
	}
	doc, err := os.ReadFile(".claude/skills/hocage/references/cel-functions.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !strings.Contains(string(doc), "`"+name+"`") {
			t.Errorf("function %q is not documented in cel-functions.md", name)
		}
	}
}
