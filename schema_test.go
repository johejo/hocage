package main

import (
	"strings"
	"testing"
)

func TestValidateRespondOutput(t *testing.T) {
	tests := []struct {
		name    string
		respond map[string]any
		want    string // substring of the single expected warning; "" = no warnings
	}{
		{"decision and reason accepted",
			map[string]any{"decision": "block", "reason": "lint failed"}, ""},
		{"decision values not validated",
			map[string]any{"decision": "totally-new-value"}, ""},
		{"unknown top-level field",
			map[string]any{"foo": "bar"}, "unknown respond field"},
		{"common field typo",
			map[string]any{"continu": true}, "unknown respond field"},
		{"common field invalid type",
			map[string]any{"suppressOutput": "yes"}, "should be bool"},
		{"decision invalid type",
			map[string]any{"decision": 42}, "should be string"},
		{"all common fields accepted",
			map[string]any{"continue": false, "stopReason": "done", "suppressOutput": true, "systemMessage": "hi", "terminalSequence": "\x1b]0;t\x07"}, ""},
		{"hookSpecificOutput content is free-form",
			map[string]any{"hookSpecificOutput": map[string]any{
				"hookEventName":      "PreToolUse",
				"permissionDecision": "some-future-value",
				"anything":           map[string]any{"deep": true},
			}}, ""},
		{"hookSpecificOutput not object",
			map[string]any{"hookSpecificOutput": "not-an-object"}, `field "hookSpecificOutput" should be object`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateRespondOutput(tt.respond)
			switch {
			case tt.want == "" && len(warnings) > 0:
				t.Errorf("warnings = %v, want none", warnings)
			case tt.want != "" && (len(warnings) != 1 || !strings.Contains(warnings[0], tt.want)):
				t.Errorf("warnings = %v, want one warning containing %q", warnings, tt.want)
			}
		})
	}
}
