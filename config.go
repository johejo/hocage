package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Hooks map[string]*Hook `yaml:"hooks"`
}

type Hook struct {
	EventName  string            `yaml:"event_name"`
	Matcher    string            `yaml:"matcher,omitempty"`
	Priority   int               `yaml:"priority,omitempty"`
	Transcript *TranscriptConfig `yaml:"transcript,omitempty"`
	When       string            `yaml:"when"`
	Action     Action            `yaml:"action"`
	Tests      map[string]*Test  `yaml:"tests,omitempty"`
}

type TranscriptConfig struct {
	Load  bool   `yaml:"load"`
	Order string `yaml:"order,omitempty"` // "chronological" (default) or "reverse"
}

type Action struct {
	Respond any               `yaml:"respond,omitempty"`
	Command any               `yaml:"command,omitempty"` // string (run via sh -c) or argv list
	Env     map[string]string `yaml:"env,omitempty"`     // NAME -> CEL expression, exported to the command
	Stdin   any               `yaml:"stdin,omitempty"`   // literal string or {cel: ...} node
	HTTP    *HTTPAction       `yaml:"http,omitempty"`
}

type HTTPAction struct {
	URL     any            `yaml:"url"` // literal string or {cel: ...} node
	Method  string         `yaml:"method,omitempty"`
	Headers map[string]any `yaml:"headers,omitempty"` // values: literal string or {cel: ...} node
	Timeout string         `yaml:"timeout,omitempty"`
}

type Test struct {
	Inputs         []any       `yaml:"inputs"`
	Result         *TestResult `yaml:"result"`
	Transcript     string      `yaml:"transcript,omitempty"`
	TranscriptFile string      `yaml:"transcript_file,omitempty"`
}

type TestResult struct {
	Stdout any `yaml:"stdout"`
}

// DefaultConfigPatterns returns the default config file patterns when --config
// is not explicitly specified. It looks for $XDG_CONFIG_HOME/hocage/*.yaml
// (falling back to ~/.config if unset) then CWD's .hocage.yaml, skipping any
// that don't exist.
func DefaultConfigPatterns() ([]string, error) {
	var patterns []string

	xdgHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		xdgHome = filepath.Join(home, ".config")
	}
	xdgPattern := filepath.Join(xdgHome, "hocage", "*.yaml")
	if matches, _ := filepath.Glob(xdgPattern); len(matches) > 0 {
		patterns = append(patterns, xdgPattern)
	}

	if _, err := os.Stat(".hocage.yaml"); err == nil {
		patterns = append(patterns, ".hocage.yaml")
	}

	return patterns, nil
}

// LoadConfig loads a single config file (mainly for tests).
func LoadConfig(path string) (*Config, error) {
	return LoadConfigs([]string{path})
}

// LoadConfigs loads and merges multiple config files. Each pattern can be a
// file path or a glob pattern. Glob-matched files are sorted alphabetically.
// When the same hook name appears in multiple files, the last one wins.
func LoadConfigs(patterns []string) (*Config, error) {
	merged := &Config{Hooks: make(map[string]*Hook)}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			if strings.ContainsAny(pattern, "*?[") {
				return nil, fmt.Errorf("no config files matched pattern %q", pattern)
			}
			// Treat as literal file path for backwards compatibility
			matches = []string{pattern}
		}
		sort.Strings(matches)

		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("reading config %q: %w", path, err)
			}
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("parsing config %q: %w", path, err)
			}
			maps.Copy(merged.Hooks, cfg.Hooks)
		}
	}

	if err := validateConfig(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func validateConfig(cfg *Config) error {
	if len(cfg.Hooks) == 0 {
		return fmt.Errorf("config must define at least one hook")
	}
	for name, hook := range cfg.Hooks {
		if hook.EventName == "" {
			return fmt.Errorf("hook %q: event_name is required", name)
		}
		if hook.When == "" {
			return fmt.Errorf("hook %q: when is required", name)
		}
		if hook.Transcript != nil {
			switch hook.Transcript.Order {
			case "", "chronological", "reverse":
				// valid
			default:
				return fmt.Errorf("hook %q: invalid transcript order %q (must be \"chronological\" or \"reverse\")", name, hook.Transcript.Order)
			}
		}
		hasRespond := hook.Action.Respond != nil
		hasCommand := hook.Action.Command != nil
		hasHTTP := hook.Action.HTTP != nil
		count := 0
		if hasRespond {
			count++
		}
		if hasCommand {
			count++
		}
		if hasHTTP {
			count++
		}
		if count != 1 {
			return fmt.Errorf("hook %q: action must have exactly one of respond, command, or http", name)
		}
		if hook.Action.Stdin != nil && !hasCommand {
			return fmt.Errorf("hook %q: stdin requires command action", name)
		}
		if hook.Action.Env != nil && !hasCommand {
			return fmt.Errorf("hook %q: env requires command action", name)
		}
		if err := validateAction(name, &hook.Action); err != nil {
			return err
		}
		loadTranscript := hook.Transcript != nil && hook.Transcript.Load
		for testName, tc := range hook.Tests {
			if tc.Transcript != "" && tc.TranscriptFile != "" {
				return fmt.Errorf("hook %q test %q: transcript and transcript_file are mutually exclusive", name, testName)
			}
			if !loadTranscript && (tc.Transcript != "" || tc.TranscriptFile != "") {
				return fmt.Errorf("hook %q test %q: transcript requires transcript.load: true on the hook", name, testName)
			}
		}
		if hasHTTP {
			if hook.Action.HTTP.URL == nil || hook.Action.HTTP.URL == "" {
				return fmt.Errorf("hook %q: http action requires url", name)
			}
			if hook.Action.HTTP.Timeout != "" {
				if _, err := time.ParseDuration(hook.Action.HTTP.Timeout); err != nil {
					return fmt.Errorf("hook %q: invalid http timeout %q: %w", name, hook.Action.HTTP.Timeout, err)
				}
			}
		}
	}
	return nil
}

var envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// validateAction checks the structural shape of an action's value slots.
func validateAction(name string, action *Action) error {
	if action.Respond != nil {
		if err := validateValueSlot(action.Respond); err != nil {
			return fmt.Errorf("hook %q respond: %w", name, err)
		}
	}
	switch cmd := action.Command.(type) {
	case nil:
	case string:
		if cmd == "" {
			return fmt.Errorf("hook %q: command must not be empty", name)
		}
		if err := checkLiteralString(cmd); err != nil {
			return fmt.Errorf("hook %q command: %w", name, err)
		}
	case []any:
		if len(cmd) == 0 {
			return fmt.Errorf("hook %q: command argv list must not be empty", name)
		}
		for i, elem := range cmd {
			if err := validateStringSlot(elem); err != nil {
				return fmt.Errorf("hook %q command[%d]: %w", name, i, err)
			}
		}
		if _, ok, _ := exprNode(cmd[0]); ok {
			return fmt.Errorf("hook %q: command[0] (the program) must be a literal string", name)
		}
	default:
		return fmt.Errorf("hook %q: command must be a string or a list, got %T", name, cmd)
	}
	for envName, expr := range action.Env {
		if !envNameRe.MatchString(envName) {
			return fmt.Errorf("hook %q: invalid env name %q (must match %s)", name, envName, envNameRe)
		}
		if expr == "" {
			return fmt.Errorf("hook %q env %s: expression must not be empty", name, envName)
		}
	}
	if action.Stdin != nil {
		if err := validateStringSlot(action.Stdin); err != nil {
			return fmt.Errorf("hook %q stdin: %w", name, err)
		}
	}
	if action.HTTP != nil {
		// A nil url is reported by validateConfig.
		if action.HTTP.URL != nil {
			if err := validateStringSlot(action.HTTP.URL); err != nil {
				return fmt.Errorf("hook %q http url: %w", name, err)
			}
		}
		for key, val := range action.HTTP.Headers {
			if err := validateStringSlot(val); err != nil {
				return fmt.Errorf("hook %q http header %q: %w", name, key, err)
			}
		}
	}
	return nil
}

// validateValueSlot recursively checks a respond value.
func validateValueSlot(v any) error {
	if _, ok, err := exprNode(v); err != nil {
		return err
	} else if ok {
		return nil
	}
	switch val := v.(type) {
	case string:
		return checkLiteralString(val)
	case map[string]any:
		for k, v2 := range val {
			if err := validateValueSlot(v2); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}
	case []any:
		for i, v2 := range val {
			if err := validateValueSlot(v2); err != nil {
				return fmt.Errorf("[%d]: %w", i, err)
			}
		}
	}
	return nil
}

// validateStringSlot checks a slot that must be a literal string or one
// expression node (stdin, http url, header values, argv elements).
func validateStringSlot(v any) error {
	if _, ok, err := exprNode(v); err != nil {
		return err
	} else if ok {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("must be a string or {cel: ...} node, got %T", v)
	}
	return checkLiteralString(s)
}

// checkLiteralString rejects literal strings that still use the removed v1
// {{expr}} syntax so stale configs fail loudly.
func checkLiteralString(s string) error {
	if m := legacyInterpolateRe.FindString(s); m != "" {
		return fmt.Errorf("legacy interpolation %q is no longer supported; strings are now literal — use {cel: ...} nodes or env:", m)
	}
	return nil
}
