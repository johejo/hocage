package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	Respond any         `yaml:"respond,omitempty"`
	Command string      `yaml:"command,omitempty"`
	Stdin   string      `yaml:"stdin,omitempty"`
	HTTP    *HTTPAction `yaml:"http,omitempty"`
}

type HTTPAction struct {
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Timeout string            `yaml:"timeout,omitempty"`
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

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
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
			for name, hook := range cfg.Hooks {
				merged.Hooks[name] = hook
			}
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
		if !validEventNames[hook.EventName] {
			return fmt.Errorf("hook %q: invalid event_name %q", name, hook.EventName)
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
		hasCommand := hook.Action.Command != ""
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
		if hook.Action.Stdin != "" && !hasCommand {
			return fmt.Errorf("hook %q: stdin requires command action", name)
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
			if hook.Action.HTTP.URL == "" {
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
