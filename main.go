package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/google/cel-go/cel"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:    "hocage",
		Usage:   "Coding Agent Hooks Policy Framework Using CEL",
		Version: Version(),
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage: "path to config file (can be specified multiple times, supports glob patterns; default: $XDG_CONFIG_HOME/hocage/*.yaml + .hocage.yaml)",
			},
		},
		Commands: []*cli.Command{
			docsCommand(),
			{
				Name:  "hooks",
				Usage: "Hook management commands",
				Commands: []*cli.Command{
					{
						Name:      "run",
						Usage:     "Run a hook (reads stdin JSON)",
						ArgsUsage: "<hook_name>",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "dry-run",
								Usage: "preview hook execution without running actions",
							},
						},
						Action: runHookAction,
					},
					{
						Name:   "check",
						Usage:  "Validate config and CEL expressions",
						Action: checkAction,
					},
					{
						Name:   "test",
						Usage:  "Run inline test cases",
						Action: testAction,
					},
					{
						Name:   "list",
						Usage:  "List all hooks defined in the config",
						Action: listAction,
					},
					{
						Name:  "generate",
						Usage: "Generate Claude Code settings.json hooks section",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "merge",
								Aliases: []string{"m"},
								Usage:   "merge with existing JSON file",
							},
							&cli.StringFlag{
								Name:    "output",
								Aliases: []string{"o"},
								Usage:   "output file (reads for merge if exists, writes with -f)",
							},
							&cli.BoolFlag{
								Name:    "force",
								Aliases: []string{"f"},
								Usage:   "write to output file (requires -o)",
							},
						},
						Action: generateAction,
					},
				},
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func loadConfigFromCmd(cmd *cli.Command) (*Config, error) {
	if cmd.IsSet("config") {
		return LoadConfigs(cmd.StringSlice("config"))
	}
	patterns, err := DefaultConfigPatterns()
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no config files found (looked for $XDG_CONFIG_HOME/hocage/*.yaml and .hocage.yaml)")
	}
	return LoadConfigs(patterns)
}

func runHookAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := loadConfigFromCmd(cmd)
	if err != nil {
		return err
	}
	args := cmd.Args()
	if args.Len() < 1 {
		return fmt.Errorf("hook name required")
	}
	dryRun := cmd.Bool("dry-run")
	return RunHook(cfg, args.First(), os.Stdin, os.Stdout, dryRun)
}

func checkAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := loadConfigFromCmd(cmd)
	if err != nil {
		return err
	}

	env, err := NewCELEnv()
	if err != nil {
		return err
	}

	var errs []string
	var warnings []string
	for name, hook := range cfg.Hooks {
		if _, err := CompileCEL(env, hook.When); err != nil {
			errs = append(errs, fmt.Sprintf("hook %q when: %v", name, err))
		}
		loadTranscript := hook.Transcript != nil && hook.Transcript.Load
		if !loadTranscript && strings.Contains(hook.When, "transcript") {
			warnings = append(warnings, fmt.Sprintf("hook %q: when expression references 'transcript' but transcript.load is not enabled", name))
		}
		if hook.Action.Command != "" {
			for _, e := range checkInterpolationExprs(env, hook.Action.Command) {
				errs = append(errs, fmt.Sprintf("hook %q command: %v", name, e))
			}
			if hook.Action.Stdin != "" {
				for _, e := range checkInterpolationExprs(env, hook.Action.Stdin) {
					errs = append(errs, fmt.Sprintf("hook %q stdin: %v", name, e))
				}
			}
		}
		for testName, tc := range hook.Tests {
			if tc.Transcript != "" {
				if _, err := ParseTranscriptJSONL(strings.NewReader(tc.Transcript)); err != nil {
					errs = append(errs, fmt.Sprintf("hook %q test %q transcript: %v", name, testName, err))
				}
			}
			if tc.TranscriptFile != "" {
				if _, err := LoadTranscriptFile(tc.TranscriptFile); err != nil {
					errs = append(errs, fmt.Sprintf("hook %q test %q transcript_file: %v", name, testName, err))
				}
			}
		}
		if hook.Action.Respond != nil {
			normalized, nerr := normalizeToJSON(hook.Action.Respond)
			if nerr != nil {
				errs = append(errs, fmt.Sprintf("hook %q respond: %v", name, nerr))
				continue
			}
			for _, e := range collectStringErrors(env, normalized) {
				errs = append(errs, fmt.Sprintf("hook %q respond: %v", name, e))
			}
			if m, ok := normalized.(map[string]any); ok {
				for _, w := range ValidateRespondOutput(hook.EventName, m) {
					errs = append(errs, fmt.Sprintf("hook %q respond schema: %v", name, w))
				}
			}
		}
	}

	if len(errs) > 0 {
		sort.Strings(errs)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		return fmt.Errorf("check found %d error(s)", len(errs))
	}
	sort.Strings(warnings)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARN: %s\n", w)
	}
	fmt.Println("ok")
	return nil
}

func checkInterpolationExprs(env *cel.Env, s string) []string {
	var errs []string
	matches := interpolateRe.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		expr := extractExpr("{{" + m[1] + "}}")
		if _, err := CompileCEL(env, expr); err != nil {
			errs = append(errs, fmt.Sprintf("expression {{%s}}: %v", expr, err))
		}
	}
	return errs
}

// collectStringErrors uses walkValue to find all interpolation errors in a respond object.
func collectStringErrors(env *cel.Env, v any) []string {
	var errs []string
	walkValue(v, func(s string) (string, error) {
		errs = append(errs, checkInterpolationExprs(env, s)...)
		return s, nil
	})
	return errs
}

func testAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := loadConfigFromCmd(cmd)
	if err != nil {
		return err
	}

	env, err := NewCELEnv()
	if err != nil {
		return err
	}

	evalCtx, err := BuildEvalContext()
	if err != nil {
		return fmt.Errorf("build eval context: %w", err)
	}

	passed, failed := 0, 0

	for hookName, hook := range cfg.Hooks {
		if len(hook.Tests) == 0 {
			continue
		}
		prg, err := CompileCEL(env, hook.When)
		if err != nil {
			fmt.Fprintf(os.Stderr, "--- FAIL: %s (compile error: %v)\n", hookName, err)
			failed++
			continue
		}

		for testName, tc := range hook.Tests {
			fullName := hookName + "/" + testName

			// Build per-test EvalContext with transcript if specified
			testEvalCtx := &EvalContext{CWD: evalCtx.CWD, ProjectRoot: evalCtx.ProjectRoot}
			if tc.Transcript != "" {
				transcript, err := ParseTranscriptJSONL(strings.NewReader(tc.Transcript))
				if err != nil {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (parse transcript: %v)\n", fullName, err)
					failed++
					continue
				}
				testEvalCtx.Transcript = transcript
			} else if tc.TranscriptFile != "" {
				transcript, err := LoadTranscriptFile(tc.TranscriptFile)
				if err != nil {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (load transcript file: %v)\n", fullName, err)
					failed++
					continue
				}
				testEvalCtx.Transcript = transcript
			}
			if hook.Transcript != nil && hook.Transcript.Order == "reverse" && testEvalCtx.Transcript != nil {
				slices.Reverse(testEvalCtx.Transcript)
			}

			for i, input := range tc.Inputs {
				caseName := fmt.Sprintf("%s/input[%d]", fullName, i)

				normalizedInput, err := normalizeToJSON(input)
				if err != nil {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (normalize error: %v)\n", caseName, err)
					failed++
					continue
				}

				matched, err := EvalCELBool(prg, normalizedInput, testEvalCtx)
				if err != nil {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (eval error: %v)\n", caseName, err)
					failed++
					continue
				}

				if tc.Result == nil {
					if matched {
						fmt.Fprintf(os.Stderr, "--- FAIL: %s (expected no match, got match)\n", caseName)
						failed++
					} else {
						fmt.Fprintf(os.Stdout, "--- PASS: %s\n", caseName)
						passed++
					}
					continue
				}

				if !matched {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (expected match, got no match)\n", caseName)
					failed++
					continue
				}

				var buf strings.Builder
				if err := ExecAction(env, &hook.Action, normalizedInput, testEvalCtx, &buf); err != nil {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (action error: %v)\n", caseName, err)
					failed++
					continue
				}

				if tc.Result.Stdout != nil {
					if ok, detail := compareJSON(tc.Result.Stdout, buf.String()); !ok {
						fmt.Fprintf(os.Stderr, "--- FAIL: %s\n    %s\n", caseName, detail)
						failed++
					} else {
						printSchemaWarnings(os.Stderr, caseName, hook.EventName, buf.String())
						fmt.Fprintf(os.Stdout, "--- PASS: %s\n", caseName)
						passed++
					}
				} else {
					printSchemaWarnings(os.Stderr, caseName, hook.EventName, buf.String())
					fmt.Fprintf(os.Stdout, "--- PASS: %s\n", caseName)
					passed++
				}
			}
		}
	}

	fmt.Fprintf(os.Stdout, "\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return fmt.Errorf("FAIL")
	}
	return nil
}

func printSchemaWarnings(w io.Writer, caseName, eventName, output string) {
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &m); err != nil {
		return // Not JSON, skip
	}
	for _, warning := range ValidateRespondOutput(eventName, m) {
		fmt.Fprintf(w, "    WARN: %s: %s\n", caseName, warning)
	}
}

func compareJSON(expected any, actualStr string) (bool, string) {
	expectedNorm, err := normalizeToJSON(expected)
	if err != nil {
		return false, fmt.Sprintf("normalize expected: %v", err)
	}

	var actualNorm any
	if err := json.Unmarshal([]byte(strings.TrimSpace(actualStr)), &actualNorm); err != nil {
		return false, fmt.Sprintf("unmarshal actual: %v", err)
	}

	expectedBytes, _ := json.Marshal(expectedNorm)
	actualBytes, _ := json.Marshal(actualNorm)

	if string(expectedBytes) == string(actualBytes) {
		return true, ""
	}
	return false, fmt.Sprintf("expected: %s\n    got:      %s", string(expectedBytes), string(actualBytes))
}

func listAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := loadConfigFromCmd(cmd)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(cfg.Hooks))
	for name := range cfg.Hooks {
		names = append(names, name)
	}
	sort.Strings(names)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tEVENT\tMATCHER\tACTION")
	for _, name := range names {
		hook := cfg.Hooks[name]
		matcher := hook.Matcher
		actionType := "respond"
		if hook.Action.Command != "" {
			actionType = "command"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, hook.EventName, matcher, actionType)
	}
	return w.Flush()
}

func generateAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := loadConfigFromCmd(cmd)
	if err != nil {
		return err
	}

	mergeFile := cmd.String("merge")
	outputFile := cmd.String("output")
	force := cmd.Bool("force")

	if force && outputFile == "" {
		return fmt.Errorf("--force requires --output")
	}

	// Determine the source file for merging
	var existingJSON []byte
	if mergeFile != "" {
		existingJSON, err = os.ReadFile(mergeFile)
		if err != nil {
			return fmt.Errorf("read merge file: %w", err)
		}
	} else if outputFile != "" {
		existingJSON, err = os.ReadFile(outputFile)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read output file: %w", err)
		}
	}

	cmdName := cmd.Root().Name
	// No merge needed: original behavior
	if existingJSON == nil {
		return Generate(cfg, cmdName, os.Stdout)
	}

	if force {
		var buf strings.Builder
		if err := GenerateMerged(cfg, cmdName, existingJSON, &buf); err != nil {
			return err
		}
		if err := os.WriteFile(outputFile, []byte(buf.String()), 0600); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Written to %s\n", outputFile)
		return nil
	}

	return GenerateMerged(cfg, cmdName, existingJSON, os.Stdout)
}
