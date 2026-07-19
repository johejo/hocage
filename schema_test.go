package main

import (
	"strings"
	"testing"
)

func TestValidateRespondOutput(t *testing.T) {
	tests := []struct {
		name    string
		event   string
		respond map[string]any
		want    string // substring of the single expected warning; "" = no warnings
	}{
		{"valid PreToolUse", "PreToolUse",
			map[string]any{"decision": "block", "reason": "not allowed"}, ""},
		{"unknown field", "PreToolUse",
			map[string]any{"decision": "block", "foo": "bar"}, "unknown field"},
		{"invalid enum", "PreToolUse",
			map[string]any{"decision": "maybe"}, "not in allowed values"},
		{"invalid type", "PreToolUse",
			map[string]any{"suppressOutput": "yes"}, "should be bool"},
		{"object field", "PermissionRequest",
			map[string]any{"decision": "allow", "updatedInput": "not-an-object"}, "should be object"},
		{"no-output event", "SessionStart",
			map[string]any{"anything": "should warn"}, "unknown field"},
		{"interpolation skips enum", "PreToolUse",
			map[string]any{"decision": "{{event.decision}}"}, ""},
		{"unknown event", "FutureEvent",
			map[string]any{"anything": "goes"}, ""},
		{"PostToolUse unknown field", "PostToolUse",
			map[string]any{"systemMessage": "ok", "extra": 123}, "unknown field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateRespondOutput(tt.event, tt.respond)
			switch {
			case tt.want == "" && len(warnings) > 0:
				t.Errorf("warnings = %v, want none", warnings)
			case tt.want != "" && (len(warnings) != 1 || !strings.Contains(warnings[0], tt.want)):
				t.Errorf("warnings = %v, want one warning containing %q", warnings, tt.want)
			}
		})
	}
}

func TestValidateRespondOutput_HookSpecificOutput(t *testing.T) {
	hso := func(fields map[string]any) map[string]any {
		return map[string]any{"hookSpecificOutput": fields}
	}
	tests := []struct {
		name    string
		event   string
		respond map[string]any
		want    string // substring of the single expected warning; "" = no warnings
	}{
		{"valid full PreToolUse", "PreToolUse", hso(map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "allow",
			"permissionDecisionReason": "command rewritten for safety",
			"updatedInput":             map[string]any{"command": "echo blocked"},
		}), ""},
		{"unknown nested field (typo)", "PreToolUse",
			hso(map[string]any{"permisionDecision": "allow"}),
			`unknown field "hookSpecificOutput.permisionDecision"`},
		{"invalid nested enum", "PreToolUse",
			hso(map[string]any{"permissionDecision": "block"}),
			`field "hookSpecificOutput.permissionDecision" value "block" not in allowed values`},
		{"hookEventName mismatch", "PreToolUse",
			hso(map[string]any{"hookEventName": "PostToolUse"}),
			`field "hookSpecificOutput.hookEventName" value "PostToolUse" not in allowed values`},
		{"free-form updatedInput not validated", "PreToolUse",
			hso(map[string]any{"updatedInput": map[string]any{"anything": map[string]any{"deep": true}}}), ""},
		{"interpolation skips nested enum", "PreToolUse",
			hso(map[string]any{"permissionDecision": "{{event.decision}}"}), ""},
		{"hookSpecificOutput not object", "PreToolUse",
			map[string]any{"hookSpecificOutput": "not-an-object"},
			`field "hookSpecificOutput" should be object`},
		{"valid watchPaths", "SessionStart",
			hso(map[string]any{"watchPaths": []any{"src", "docs"}}), ""},
		{"watchPaths non-string element", "SessionStart",
			hso(map[string]any{"watchPaths": []any{"src", 42}}),
			`field "hookSpecificOutput.watchPaths"[1] should be string`},
		{"watchPaths not a list", "SessionStart",
			hso(map[string]any{"watchPaths": "src"}),
			`field "hookSpecificOutput.watchPaths" should be string list`},
		{"updatedToolOutput as string", "PostToolUse",
			hso(map[string]any{"updatedToolOutput": "plain text"}), ""},
		{"updatedToolOutput as object", "PostToolUse",
			hso(map[string]any{"updatedToolOutput": map[string]any{"stdout": "ok"}}), ""},
		{"valid nested decision", "PermissionRequest",
			hso(map[string]any{"decision": map[string]any{"behavior": "allow", "updatedInput": map[string]any{"command": "ls"}}}), ""},
		{"doubly-nested enum violation", "PermissionRequest",
			hso(map[string]any{"decision": map[string]any{"behavior": "block"}}),
			`field "hookSpecificOutput.decision.behavior" value "block" not in allowed values`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateRespondOutput(tt.event, tt.respond)
			switch {
			case tt.want == "" && len(warnings) > 0:
				t.Errorf("warnings = %v, want none", warnings)
			case tt.want != "" && (len(warnings) != 1 || !strings.Contains(warnings[0], tt.want)):
				t.Errorf("warnings = %v, want one warning containing %q", warnings, tt.want)
			}
		})
	}
}
