package main

import (
	"strings"
	"testing"
)

func TestValidateRespondOutput_ValidPreToolUse(t *testing.T) {
	warnings := ValidateRespondOutput("PreToolUse", map[string]any{
		"decision": "block",
		"reason":   "not allowed",
	})
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestValidateRespondOutput_UnknownField(t *testing.T) {
	warnings := ValidateRespondOutput("PreToolUse", map[string]any{
		"decision": "block",
		"foo":      "bar",
	})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "unknown field") {
		t.Errorf("warnings = %v, want unknown field warning", warnings)
	}
}

func TestValidateRespondOutput_InvalidEnum(t *testing.T) {
	warnings := ValidateRespondOutput("PreToolUse", map[string]any{
		"decision": "maybe",
	})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "not in allowed values") {
		t.Errorf("warnings = %v, want enum warning", warnings)
	}
}

func TestValidateRespondOutput_InvalidType(t *testing.T) {
	warnings := ValidateRespondOutput("PreToolUse", map[string]any{
		"suppressOutput": "yes",
	})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "should be bool") {
		t.Errorf("warnings = %v, want type warning", warnings)
	}
}

func TestValidateRespondOutput_ObjectField(t *testing.T) {
	warnings := ValidateRespondOutput("PermissionRequest", map[string]any{
		"decision":     "allow",
		"updatedInput": "not-an-object",
	})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "should be object") {
		t.Errorf("warnings = %v, want object type warning", warnings)
	}
}

func TestValidateRespondOutput_NoOutputEvent(t *testing.T) {
	warnings := ValidateRespondOutput("SessionStart", map[string]any{
		"anything": "should warn",
	})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "unknown field") {
		t.Errorf("warnings = %v, want unknown field warning for no-output event", warnings)
	}
}

func TestValidateRespondOutput_InterpolationSkipsEnum(t *testing.T) {
	warnings := ValidateRespondOutput("PreToolUse", map[string]any{
		"decision": "{{event.decision}}",
	})
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want no warnings for interpolated enum", warnings)
	}
}

func TestValidateRespondOutput_UnknownEvent(t *testing.T) {
	warnings := ValidateRespondOutput("FutureEvent", map[string]any{
		"anything": "goes",
	})
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want no warnings for unknown event", warnings)
	}
}

func TestValidateRespondOutput_PostToolUseUnknownField(t *testing.T) {
	warnings := ValidateRespondOutput("PostToolUse", map[string]any{
		"systemMessage": "ok",
		"extra":         123,
	})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "unknown field") {
		t.Errorf("warnings = %v", warnings)
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

func TestOutputSchemas_AllEventsCovered(t *testing.T) {
	for name := range validEventNames {
		if _, ok := outputSchemas[name]; !ok {
			t.Errorf("event %q has no output schema defined", name)
		}
	}
}
