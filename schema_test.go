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

func TestOutputSchemas_AllEventsCovered(t *testing.T) {
	for name := range validEventNames {
		if _, ok := outputSchemas[name]; !ok {
			t.Errorf("event %q has no output schema defined", name)
		}
	}
}
