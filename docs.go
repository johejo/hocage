package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

//go:embed all:.claude/skills/hocage
var docsFS embed.FS

const docsRoot = ".claude/skills/hocage"

var docTopics = map[string]string{
	"overview":            "SKILL.md",
	"events":              "references/event-types-and-output.md",
	"cel":                 "references/cel-functions.md",
	"patterns":            "references/patterns.md",
	"transcript-patterns": "references/transcript-patterns.md",
}

func docTopicNames() []string {
	return slices.Sorted(maps.Keys(docTopics))
}

func docsCommand() *cli.Command {
	return &cli.Command{
		Name:      "docs",
		Usage:     "Show embedded documentation",
		ArgsUsage: "[topic]",
		Description: fmt.Sprintf(`Shows the embedded skill documentation (.claude/skills/hocage) from the CLI.

Available topics: %s (default: overview).

Use --output-dir to dump all docs to a directory; existing frontmatter in the
destination files is preserved unless --overwrite-frontmatter is set.`,
			strings.Join(docTopicNames(), ", ")),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "output-dir",
				Usage: "dump all docs to directory",
			},
			&cli.BoolFlag{
				Name:  "overwrite-frontmatter",
				Usage: "overwrite existing frontmatter when dumping (default: preserve)",
			},
		},
		Action: docsAction,
	}
}

func docsAction(ctx context.Context, cmd *cli.Command) error {
	outputDir := cmd.String("output-dir")
	overwriteFrontmatter := cmd.Bool("overwrite-frontmatter")

	if overwriteFrontmatter && outputDir == "" {
		return fmt.Errorf("--overwrite-frontmatter requires --output-dir")
	}

	if outputDir != "" {
		if cmd.Args().Len() > 0 {
			return fmt.Errorf("cannot specify both --output-dir and a topic")
		}
		return dumpAllDocs(outputDir, overwriteFrontmatter)
	}

	topic := "overview"
	if cmd.Args().Len() > 0 {
		topic = cmd.Args().First()
	}

	relPath, ok := docTopics[topic]
	if !ok {
		return fmt.Errorf("unknown topic %q, available topics: %s", topic, strings.Join(docTopicNames(), ", "))
	}

	data, err := docsFS.ReadFile(filepath.Join(docsRoot, relPath))
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(data)
	return err
}

func dumpAllDocs(dir string, overwriteFrontmatter bool) error {
	return fs.WalkDir(docsFS, docsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(docsRoot, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		newContent, err := docsFS.ReadFile(path)
		if err != nil {
			return err
		}

		if !overwriteFrontmatter {
			existing, readErr := os.ReadFile(destPath)
			if readErr == nil {
				newContent = preserveFrontmatter(existing, newContent)
			}
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, newContent, 0o644)
	})
}

// preserveFrontmatter keeps the frontmatter from existing content and replaces the body with new content.
// Frontmatter is defined as the content between the first "---\n" at the start of the file and the next "---\n".
func preserveFrontmatter(existing, newContent []byte) []byte {
	existingFM, _ := splitFrontmatter(existing)
	if existingFM == nil {
		return newContent
	}

	_, newBody := splitFrontmatter(newContent)
	if newBody == nil {
		// new content has no frontmatter, keep existing frontmatter + new content as body
		newBody = newContent
	}

	var buf bytes.Buffer
	buf.Write(existingFM)
	buf.Write(newBody)
	return buf.Bytes()
}

// splitFrontmatter splits content into frontmatter (including delimiters) and body.
// Returns (nil, nil) if no frontmatter is found.
func splitFrontmatter(data []byte) (frontmatter, body []byte) {
	const delimiter = "---\n"
	if !bytes.HasPrefix(data, []byte(delimiter)) {
		return nil, nil
	}
	rest := data[len(delimiter):]
	idx := bytes.Index(rest, []byte(delimiter))
	if idx < 0 {
		return nil, nil
	}
	end := len(delimiter) + idx + len(delimiter)
	return data[:end], data[end:]
}
