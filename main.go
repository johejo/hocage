package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/urfave/cli/v3"
)

func newApp() *cli.Command {
	return &cli.Command{
		Name:    "hocage",
		Usage:   "Coding Agent Hooks Policy Framework Using CEL",
		Version: Version(),
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "path to config file (can be specified multiple times, supports glob patterns; default: $XDG_CONFIG_HOME/hocage/*.yaml + .hocage.yaml)",
			},
		},
		Commands: []*cli.Command{
			docsCommand(),
			gendocsCommand(),
			{
				Name:  "hooks",
				Usage: "Hook management commands",
				Commands: []*cli.Command{
					{
						Name:      "run",
						Usage:     "Run a hook (reads event JSON on stdin)",
						ArgsUsage: "<hook_name>",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "dry-run",
								Usage: "skip executing the action",
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
						Description: `Generates the hooks section for Claude Code's settings.json from the config.

Example output:

    {
      "hooks": {
        "PreToolUse": [
          {
            "matcher": "Bash",
            "hooks": [
              {
                "type": "command",
                "command": "hocage hooks run block_rm_rf"
              }
            ]
          }
        ]
      }
    }`,
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
}

func main() {
	if err := newApp().Run(context.Background(), os.Args); err != nil {
		// Exit with the command's code and keep stderr free of wrapper noise.
		if cmdExit, ok := errors.AsType[*commandExitError](err); ok {
			os.Exit(cmdExit.code)
		}
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
	return RunHook(cfg, args.First(), os.Stdin, os.Stdout, os.Stderr, dryRun)
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
		if !validEventNames[hook.EventName] {
			warnings = append(warnings, fmt.Sprintf("hook %q: unknown event_name %q", name, hook.EventName))
		}
		if _, err := CompileCEL(env, hook.When); err != nil {
			errs = append(errs, fmt.Sprintf("hook %q when: %v", name, err))
		}
		loadTranscript := hook.Transcript != nil && hook.Transcript.Load
		if !loadTranscript && strings.Contains(hook.When, "transcript") {
			warnings = append(warnings, fmt.Sprintf("hook %q: when expression references 'transcript' but transcript.load is not enabled", name))
		}
		for _, e := range checkActionExprs(env, &hook.Action) {
			errs = append(errs, fmt.Sprintf("hook %q %s", name, e))
		}
		for _, w := range checkEnvNames(&hook.Action) {
			warnings = append(warnings, fmt.Sprintf("hook %q %s", name, w))
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
			if _, isNode, _ := exprNode(normalized); !isNode {
				if m, ok := normalized.(map[string]any); ok {
					for _, w := range ValidateRespondOutput(m) {
						warnings = append(warnings, fmt.Sprintf("hook %q respond schema: %v", name, w))
					}
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

// checkActionExprs statically compiles every expression slot in an action.
// String slots additionally reject expressions whose static type is a list or map.
func checkActionExprs(env *cel.Env, action *Action) []string {
	var errs []string
	checkExpr := func(slot, expr string, stringSlot bool) {
		ast, issues := env.Compile(expr)
		if issues != nil && issues.Err() != nil {
			errs = append(errs, fmt.Sprintf("%s: expression %q: %v", slot, expr, issues.Err()))
			return
		}
		if stringSlot {
			switch ast.OutputType().Kind() {
			case types.ListKind, types.MapKind, types.NullTypeKind:
				errs = append(errs, fmt.Sprintf("%s: expression %q: result must be a string, got %s (use to_json(...))", slot, expr, ast.OutputType()))
			}
		}
	}
	checkStringSlot := func(slot string, v any) {
		if expr, ok, _ := exprNode(v); ok {
			checkExpr(slot, expr, true)
		}
	}

	if action.Respond != nil {
		if normalized, err := normalizeToJSON(action.Respond); err == nil {
			checkRespondExprs(checkExpr, "respond", normalized)
		}
	}
	if argv, ok := action.Command.([]any); ok {
		for i, elem := range argv {
			checkStringSlot(fmt.Sprintf("command[%d]", i), elem)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(action.Env)) {
		checkExpr("env "+name, action.Env[name], true)
	}
	if action.Stdin != nil {
		checkStringSlot("stdin", action.Stdin)
	}
	if action.HTTP != nil {
		checkStringSlot("http url", action.HTTP.URL)
		for _, key := range slices.Sorted(maps.Keys(action.HTTP.Headers)) {
			checkStringSlot(fmt.Sprintf("http header %q", key), action.HTTP.Headers[key])
		}
	}
	return errs
}

// checkRespondExprs walks a respond value and compiles every {cel: ...} node.
// Respond slots are typed, so any result type is acceptable.
func checkRespondExprs(checkExpr func(slot, expr string, stringSlot bool), slot string, v any) {
	if expr, ok, _ := exprNode(v); ok {
		checkExpr(slot, expr, false)
		return
	}
	switch val := v.(type) {
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(val)) {
			checkRespondExprs(checkExpr, slot+"."+k, val[k])
		}
	case []any:
		for i, v2 := range val {
			checkRespondExprs(checkExpr, fmt.Sprintf("%s[%d]", slot, i), v2)
		}
	}
}

// shellDangerousEnvNames are env names that change how sh interprets the
// command itself; setting them from event data deserves a warning.
var shellDangerousEnvNames = map[string]bool{
	"PATH": true, "IFS": true, "ENV": true, "BASH_ENV": true, "SHELLOPTS": true,
}

func checkEnvNames(action *Action) []string {
	var warnings []string
	for _, name := range slices.Sorted(maps.Keys(action.Env)) {
		if shellDangerousEnvNames[name] || strings.HasPrefix(name, "LD_") || strings.HasPrefix(name, "DYLD_") {
			warnings = append(warnings, fmt.Sprintf("env %s: setting %s can change how the shell resolves or runs commands", name, name))
		}
	}
	return warnings
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

	needProjectRoot := false
	for _, hook := range cfg.Hooks {
		if HookReferencesProjectRoot(hook) {
			needProjectRoot = true
			break
		}
	}
	evalCtx, err := BuildEvalContext(needProjectRoot)
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
				if err := ExecAction(env, &hook.Action, normalizedInput, testEvalCtx, &buf, &buf); err != nil {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (action error: %v)\n", caseName, err)
					failed++
					continue
				}

				if tc.Result.Stdout != nil {
					if ok, detail := compareJSON(tc.Result.Stdout, buf.String()); !ok {
						fmt.Fprintf(os.Stderr, "--- FAIL: %s\n    %s\n", caseName, detail)
						failed++
					} else {
						printSchemaWarnings(os.Stderr, caseName, buf.String())
						fmt.Fprintf(os.Stdout, "--- PASS: %s\n", caseName)
						passed++
					}
				} else {
					printSchemaWarnings(os.Stderr, caseName, buf.String())
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

func printSchemaWarnings(w io.Writer, caseName, output string) {
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &m); err != nil {
		return // Not JSON, skip
	}
	for _, warning := range ValidateRespondOutput(m) {
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
		switch {
		case hook.Action.Command != nil:
			actionType = "command"
		case hook.Action.HTTP != nil:
			actionType = "http"
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
