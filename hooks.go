package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// RunHook evaluates the named hook against input JSON and writes output if the condition matches.
func RunHook(cfg *Config, hookName string, input io.Reader, output io.Writer) error {
	hook, ok := cfg.Hooks[hookName]
	if !ok {
		return fmt.Errorf("hook %q not found", hookName)
	}

	var event any
	if err := json.NewDecoder(input).Decode(&event); err != nil {
		return fmt.Errorf("decode stdin: %w", err)
	}

	env, err := NewCELEnv()
	if err != nil {
		return fmt.Errorf("create CEL env: %w", err)
	}

	prg, err := CompileCEL(env, hook.When)
	if err != nil {
		return fmt.Errorf("compile when: %w", err)
	}

	matched, err := EvalCELBool(prg, event)
	if err != nil {
		return fmt.Errorf("eval when: %w", err)
	}

	if !matched {
		return nil
	}

	return ExecAction(env, &hook.Action, event, output)
}
