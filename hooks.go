package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

// RunHook evaluates the named hook against input JSON and writes output if the condition matches.
func RunHook(cfg *Config, hookName string, input io.Reader, output io.Writer, dryRun bool) error {
	hook, ok := cfg.Hooks[hookName]
	if !ok {
		return fmt.Errorf("hook %q not found", hookName)
	}

	var event any
	if err := json.NewDecoder(input).Decode(&event); err != nil {
		return fmt.Errorf("decode stdin: %w", err)
	}

	evalCtx, err := BuildEvalContext()
	if err != nil {
		return fmt.Errorf("build eval context: %w", err)
	}

	if hook.Transcript != nil && hook.Transcript.Load {
		eventMap, ok := event.(map[string]any)
		if !ok {
			return fmt.Errorf("event must be a JSON object when transcript.load is enabled")
		}
		transcriptPath, _ := eventMap["transcript_path"].(string)
		if transcriptPath == "" {
			return fmt.Errorf("transcript.load is enabled but event has no transcript_path")
		}
		transcript, err := LoadTranscriptFile(transcriptPath)
		if err != nil {
			return fmt.Errorf("load transcript: %w", err)
		}
		if hook.Transcript.Order == "reverse" {
			slices.Reverse(transcript)
		}
		evalCtx.Transcript = transcript
	}

	env, err := NewCELEnv()
	if err != nil {
		return fmt.Errorf("create CEL env: %w", err)
	}

	prg, err := CompileCEL(env, hook.When)
	if err != nil {
		return fmt.Errorf("compile when: %w", err)
	}

	matched, err := EvalCELBool(prg, event, evalCtx)
	if err != nil {
		return fmt.Errorf("eval when: %w", err)
	}

	if !matched {
		if dryRun {
			fmt.Fprintln(output, "[dry-run] not matched")
		}
		return nil
	}

	if dryRun {
		return DryRunAction(env, &hook.Action, hook.EventName, event, evalCtx, output)
	}

	return ExecAction(env, &hook.Action, event, evalCtx, output)
}
