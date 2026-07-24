package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocsTopics(t *testing.T) {
	for topic, relPath := range docTopics {
		data, err := docsFS.ReadFile(filepath.Join(docsRoot, relPath))
		if err != nil {
			t.Errorf("topic %q: read error: %v", topic, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("topic %q: empty content", topic)
		}
	}
}

func TestDocsTopicContent(t *testing.T) {
	tests := []struct {
		topic         string
		wantSubstring string
	}{
		{"overview", "hocage"},
		{"cel", "CEL"},
		{"events", "Event"},
		{"patterns", "Pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			data, err := docsFS.ReadFile(filepath.Join(docsRoot, docTopics[tt.topic]))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tt.wantSubstring) {
				t.Errorf("%s topic should contain %q", tt.topic, tt.wantSubstring)
			}
		})
	}
}

func TestPreserveFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		new      string
		want     string
	}{
		{
			name:     "existing has frontmatter, new has frontmatter",
			existing: "---\nname: old\n---\nold body\n",
			new:      "---\nname: new\n---\nnew body\n",
			want:     "---\nname: old\n---\nnew body\n",
		},
		{
			name:     "existing has frontmatter, new has no frontmatter",
			existing: "---\nname: old\n---\nold body\n",
			new:      "new body only\n",
			want:     "---\nname: old\n---\nnew body only\n",
		},
		{
			name:     "existing has no frontmatter",
			existing: "no frontmatter\n",
			new:      "---\nname: new\n---\nnew body\n",
			want:     "---\nname: new\n---\nnew body\n",
		},
		{
			name:     "both have no frontmatter",
			existing: "old content\n",
			new:      "new content\n",
			want:     "new content\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preserveFrontmatter([]byte(tt.existing), []byte(tt.new))
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestDumpAllDocs(t *testing.T) {
	dir := t.TempDir()

	if err := dumpAllDocs(dir, false); err != nil {
		t.Fatal(err)
	}

	// Verify expected files exist
	for _, relPath := range docTopics {
		path := filepath.Join(dir, relPath)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected file %s: %v", relPath, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("expected non-empty file %s", relPath)
		}
	}
}

func TestDumpAllDocsPreservesFrontmatter(t *testing.T) {
	dir := t.TempDir()

	// First dump
	if err := dumpAllDocs(dir, false); err != nil {
		t.Fatal(err)
	}

	// Modify frontmatter of SKILL.md
	skillPath := filepath.Join(dir, "SKILL.md")
	original, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}

	customFM := "---\nname: custom\ndescription: my custom description\n---\n"
	_, body := splitFrontmatter(original)
	if body == nil {
		t.Fatal("SKILL.md should have frontmatter")
	}
	modified := customFM + string(body)
	if err := os.WriteFile(skillPath, []byte(modified), 0o644); err != nil {
		t.Fatal(err)
	}

	// Second dump (should preserve frontmatter)
	if err := dumpAllDocs(dir, false); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}

	fm, _ := splitFrontmatter(after)
	if string(fm) != customFM {
		t.Errorf("frontmatter not preserved: got %q, want %q", string(fm), customFM)
	}
}

func TestDumpAllDocsOverwriteFrontmatter(t *testing.T) {
	dir := t.TempDir()

	// First dump
	if err := dumpAllDocs(dir, false); err != nil {
		t.Fatal(err)
	}

	// Modify frontmatter
	skillPath := filepath.Join(dir, "SKILL.md")
	original, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}

	customFM := "---\nname: custom\n---\n"
	_, body := splitFrontmatter(original)
	if body == nil {
		t.Fatal("SKILL.md should have frontmatter")
	}
	modified := customFM + string(body)
	if err := os.WriteFile(skillPath, []byte(modified), 0o644); err != nil {
		t.Fatal(err)
	}

	// Dump with overwrite
	if err := dumpAllDocs(dir, true); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}

	// Should match the embedded original
	if string(after) != string(original) {
		t.Error("overwrite-frontmatter should restore original content")
	}
}
