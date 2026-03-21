package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "agcel",
		Usage: "Coding Agent Hooks Policy Framework Using CEL",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   ".agcel.yaml",
				Usage:   "path to config file",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "hooks",
				Usage: "Hook management commands",
				Commands: []*cli.Command{
					{
						Name:      "run",
						Usage:     "Run a hook (reads stdin JSON)",
						ArgsUsage: "<hook_name>",
						Action:    runHookAction,
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
						Name:   "generate",
						Usage:  "Generate Claude Code settings.json hooks section",
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
	path := cmd.String("config")
	return LoadConfig(path)
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
	return RunHook(cfg, args.First(), os.Stdin, os.Stdout)
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
	for name, hook := range cfg.Hooks {
		if _, err := CompileCEL(env, hook.When); err != nil {
			errs = append(errs, fmt.Sprintf("hook %q when: %v", name, err))
		}
		if hook.Action.Command != "" {
			for _, e := range checkInterpolationExprs(env, hook.Action.Command) {
				errs = append(errs, fmt.Sprintf("hook %q command: %v", name, e))
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
		}
	}

	if len(errs) > 0 {
		sort.Strings(errs)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		return fmt.Errorf("check found %d error(s)", len(errs))
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
			for i, input := range tc.Inputs {
				caseName := fmt.Sprintf("%s/input[%d]", fullName, i)

				normalizedInput, err := normalizeToJSON(input)
				if err != nil {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (normalize error: %v)\n", caseName, err)
					failed++
					continue
				}

				matched, err := EvalCELBool(prg, normalizedInput)
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
				if err := ExecAction(env, &hook.Action, normalizedInput, &buf); err != nil {
					fmt.Fprintf(os.Stderr, "--- FAIL: %s (action error: %v)\n", caseName, err)
					failed++
					continue
				}

				if tc.Result.Stdout != nil {
					if ok, detail := compareJSON(tc.Result.Stdout, buf.String()); !ok {
						fmt.Fprintf(os.Stderr, "--- FAIL: %s\n    %s\n", caseName, detail)
						failed++
					} else {
						fmt.Fprintf(os.Stdout, "--- PASS: %s\n", caseName)
						passed++
					}
				} else {
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

func generateAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := loadConfigFromCmd(cmd)
	if err != nil {
		return err
	}
	return Generate(cfg, "agcel", os.Stdout)
}
