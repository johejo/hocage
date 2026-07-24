package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
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

// TestRenderTargetMissingFile: a missing target file is a new target at the
// repo root (go.mod present), a wrong working directory elsewhere.
func TestRenderTargetMissingFile(t *testing.T) {
	rendered := []byte("generated\n")
	for _, tt := range []struct {
		name     string
		content  string // written to the target file; "" = no file
		goMod    bool
		wantErr  string // substring of the expected error; "" = no error
		wantCurr string
	}{
		{name: "existing file", content: "old\n", goMod: true, wantCurr: "old\n"},
		{name: "new target at repo root", goMod: true, wantCurr: ""},
		{name: "missing file outside repo root", wantErr: "must run from the repo root"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			if tt.goMod {
				if err := os.WriteFile("go.mod", []byte("module x\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			tgt := genTarget{
				Path:   "doc.md",
				Render: func([]byte) ([]byte, error) { return rendered, nil },
			}
			if tt.content != "" {
				if err := os.WriteFile(tgt.Path, []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			current, want, err := renderTarget(tgt)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(current) != tt.wantCurr {
				t.Errorf("current = %q, want %q", current, tt.wantCurr)
			}
			if !bytes.Equal(want, rendered) {
				t.Errorf("want = %q, want %q", want, rendered)
			}
		})
	}
}

// TestCLIDocsSkipHidden guards that hidden flags and commands stay out of the
// generated docs; a leak would be regenerated into the committed files, so the
// golden diff above cannot catch it.
func TestCLIDocsSkipHidden(t *testing.T) {
	app := &cli.Command{
		Name:  "hocage",
		Flags: []cli.Flag{&cli.StringFlag{Name: "secret-global", Hidden: true}},
		Commands: []*cli.Command{
			{
				Name:  "visible",
				Usage: "a visible command",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "shown", Usage: "a visible flag"},
					&cli.BoolFlag{Name: "secret", Hidden: true},
				},
			},
			{Name: "ghost", Usage: "a hidden command", Hidden: true},
		},
	}
	for _, tt := range []struct {
		name string
		out  string
	}{
		{name: "generateCLIDocs", out: string(generateCLIDocs(app))},
		{name: "generateCLITable", out: string(generateCLITable(app))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.out, "--shown") {
				t.Errorf("missing visible flag --shown in output:\n%s", tt.out)
			}
			for _, hidden := range []string{"secret", "ghost"} {
				if strings.Contains(tt.out, hidden) {
					t.Errorf("hidden name %q leaked into output:\n%s", hidden, tt.out)
				}
			}
		})
	}
}
