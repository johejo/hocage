package main

import (
	"bytes"
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
