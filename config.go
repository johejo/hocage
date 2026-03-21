package main

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Hooks map[string]*Hook `yaml:"hooks"`
}

type Hook struct {
	EventName string            `yaml:"event_name"`
	Matcher   string            `yaml:"matcher,omitempty"`
	When      string            `yaml:"when"`
	Action    Action            `yaml:"action"`
	Tests     map[string]*Test  `yaml:"tests,omitempty"`
}

type Action struct {
	Respond any    `yaml:"respond,omitempty"`
	Command string `yaml:"command,omitempty"`
}

type Test struct {
	Inputs []any       `yaml:"inputs"`
	Result *TestResult `yaml:"result"`
}

type TestResult struct {
	Stdout any `yaml:"stdout"`
}

var validEventNames = map[string]bool{
	"PreToolUse":       true,
	"PostToolUse":      true,
	"Stop":             true,
	"UserPromptSubmit": true,
	"SubagentStop":     true,
	"PreSubagentTool":  true,
	"Notification":     true,
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
		hasRespond := hook.Action.Respond != nil
		hasCommand := hook.Action.Command != ""
		if !hasRespond && !hasCommand {
			return fmt.Errorf("hook %q: action must have respond or command", name)
		}
		if hasRespond && hasCommand {
			return fmt.Errorf("hook %q: action must have respond or command, not both", name)
		}
	}
	return nil
}
