package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// OutputFieldType represents the expected type of an output field.
type OutputFieldType int

const (
	FieldTypeString OutputFieldType = iota
	FieldTypeBool
	FieldTypeObject
	FieldTypeAny        // any JSON value, no type check
	FieldTypeStringList // list of strings
)

func (t OutputFieldType) String() string {
	switch t {
	case FieldTypeString:
		return "string"
	case FieldTypeBool:
		return "bool"
	case FieldTypeObject:
		return "object"
	case FieldTypeAny:
		return "any"
	case FieldTypeStringList:
		return "string list"
	default:
		return "unknown"
	}
}

// OutputField describes a single field in an output schema.
type OutputField struct {
	Type   OutputFieldType
	Enum   []string               // If non-nil, value must be one of these strings.
	Fields map[string]OutputField // For FieldTypeObject: nested schema. Nil means free-form object.
}

// OutputSchema defines the expected output fields for an event type.
type OutputSchema struct {
	Fields map[string]OutputField
}

// hookSpecificOutput builds the schema for the hookSpecificOutput wrapper object.
// hookEventName must equal the event name that triggered the hook.
func hookSpecificOutput(eventName string, fields map[string]OutputField) OutputField {
	all := map[string]OutputField{
		"hookEventName": {Type: FieldTypeString, Enum: []string{eventName}},
	}
	maps.Copy(all, fields)
	return OutputField{Type: FieldTypeObject, Fields: all}
}

// outputSchemas maps event names to their expected respond output schema.
var outputSchemas = map[string]*OutputSchema{
	"PreToolUse": {
		Fields: map[string]OutputField{
			"decision":       {Type: FieldTypeString, Enum: []string{"allow", "deny", "block"}},
			"reason":         {Type: FieldTypeString},
			"suppressOutput": {Type: FieldTypeBool},
			"hookSpecificOutput": hookSpecificOutput("PreToolUse", map[string]OutputField{
				"permissionDecision":       {Type: FieldTypeString, Enum: []string{"allow", "deny", "ask", "defer"}},
				"permissionDecisionReason": {Type: FieldTypeString},
				"updatedInput":             {Type: FieldTypeObject},
				"additionalContext":        {Type: FieldTypeString},
			}),
		},
	},
	"PostToolUse": {
		Fields: map[string]OutputField{
			"systemMessage": {Type: FieldTypeString},
			"hookSpecificOutput": hookSpecificOutput("PostToolUse", map[string]OutputField{
				"updatedToolOutput": {Type: FieldTypeAny},
				"additionalContext": {Type: FieldTypeString},
			}),
		},
	},
	"PermissionRequest": {
		Fields: map[string]OutputField{
			"decision":     {Type: FieldTypeString, Enum: []string{"allow", "deny", "block"}},
			"reason":       {Type: FieldTypeString},
			"updatedInput": {Type: FieldTypeObject},
			"hookSpecificOutput": hookSpecificOutput("PermissionRequest", map[string]OutputField{
				"decision": {Type: FieldTypeObject, Fields: map[string]OutputField{
					"behavior":        {Type: FieldTypeString, Enum: []string{"allow", "deny"}},
					"updatedInput":    {Type: FieldTypeObject},
					"permissionRules": {Type: FieldTypeString},
				}},
			}),
		},
	},
	"Stop": {
		Fields: map[string]OutputField{
			"decision":     {Type: FieldTypeString, Enum: []string{"block"}},
			"reason":       {Type: FieldTypeString},
			"updatedInput": {Type: FieldTypeObject},
			"hookSpecificOutput": hookSpecificOutput("Stop", map[string]OutputField{
				"additionalContext": {Type: FieldTypeString},
			}),
		},
	},
	"UserPromptSubmit": {
		Fields: map[string]OutputField{
			"updatedInput":      {Type: FieldTypeString},
			"additionalContext": {Type: FieldTypeString},
		},
	},
	"SubagentStart": {
		Fields: map[string]OutputField{
			"decision": {Type: FieldTypeString, Enum: []string{"allow", "deny", "block"}},
			"reason":   {Type: FieldTypeString},
		},
	},
	"SessionStart": {
		Fields: map[string]OutputField{
			"hookSpecificOutput": hookSpecificOutput("SessionStart", map[string]OutputField{
				"additionalContext":  {Type: FieldTypeString},
				"initialUserMessage": {Type: FieldTypeString},
				"sessionTitle":       {Type: FieldTypeString},
				"watchPaths":         {Type: FieldTypeStringList},
				"reloadSkills":       {Type: FieldTypeBool},
			}),
		},
	},
	"SubagentStop": {
		Fields: map[string]OutputField{
			"hookSpecificOutput": hookSpecificOutput("SubagentStop", map[string]OutputField{
				"additionalContext": {Type: FieldTypeString},
			}),
		},
	},
	"Elicitation": {
		Fields: map[string]OutputField{
			"hookSpecificOutput": hookSpecificOutput("Elicitation", map[string]OutputField{
				"action":  {Type: FieldTypeString, Enum: []string{"accept", "decline", "cancel"}},
				"content": {Type: FieldTypeObject},
			}),
		},
	},
	"ElicitationResult": {
		Fields: map[string]OutputField{
			"hookSpecificOutput": hookSpecificOutput("ElicitationResult", map[string]OutputField{
				"action":  {Type: FieldTypeString, Enum: []string{"accept", "decline", "cancel"}},
				"content": {Type: FieldTypeObject},
			}),
		},
	},
	"WorktreeCreate": {
		Fields: map[string]OutputField{
			"hookSpecificOutput": hookSpecificOutput("WorktreeCreate", map[string]OutputField{
				"worktreePath": {Type: FieldTypeString},
			}),
		},
	},
	// Events with no output fields
	"Notification":       {Fields: map[string]OutputField{}},
	"SessionEnd":         {Fields: map[string]OutputField{}},
	"PostToolUseFailure": {Fields: map[string]OutputField{}},
	"StopFailure":        {Fields: map[string]OutputField{}},
	// New event types
	"TaskCompleted": {
		Fields: map[string]OutputField{
			"continue":   {Type: FieldTypeBool},
			"stopReason": {Type: FieldTypeString},
		},
	},
	"ConfigChange": {
		Fields: map[string]OutputField{
			"decision": {Type: FieldTypeString, Enum: []string{"block"}},
			"reason":   {Type: FieldTypeString},
		},
	},
	"TeammateIdle": {
		Fields: map[string]OutputField{
			"continue":   {Type: FieldTypeBool},
			"stopReason": {Type: FieldTypeString},
		},
	},
	"PreCompact":         {Fields: map[string]OutputField{}},
	"PostCompact":        {Fields: map[string]OutputField{}},
	"InstructionsLoaded": {Fields: map[string]OutputField{}},
	"WorktreeRemove":     {Fields: map[string]OutputField{}},
}

// ValidateRespondOutput validates a respond output object against the schema for the given event.
// Returns a list of warning messages (not errors, since the output may still work).
func ValidateRespondOutput(eventName string, respond map[string]any) []string {
	schema, ok := outputSchemas[eventName]
	if !ok {
		return nil // Unknown event, skip validation
	}
	return validateFields(eventName, "", schema.Fields, respond)
}

// validateFields validates obj against fields, recursing into FieldTypeObject
// fields that declare a nested schema. prefix is the dotted path of obj within
// the respond object ("" at the root).
func validateFields(eventName, prefix string, fields map[string]OutputField, obj map[string]any) []string {
	var warnings []string

	// Check for unknown fields
	for key := range obj {
		if _, ok := fields[key]; !ok {
			warnings = append(warnings, fmt.Sprintf("unknown field %q for event %s", prefix+key, eventName))
		}
	}

	// Check field types and enum values
	for name, field := range fields {
		val, ok := obj[name]
		if !ok {
			continue // Field is optional
		}
		path := prefix + name
		switch field.Type {
		case FieldTypeString:
			s, ok := val.(string)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be string, got %T", path, val))
				continue
			}
			if len(field.Enum) > 0 && !interpolateRe.MatchString(s) && !slices.Contains(field.Enum, s) {
				warnings = append(warnings, fmt.Sprintf("field %q value %q not in allowed values: %s", path, s, strings.Join(field.Enum, ", ")))
			}
		case FieldTypeBool:
			if _, ok := val.(bool); !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be bool, got %T", path, val))
			}
		case FieldTypeObject:
			m, ok := val.(map[string]any)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be object, got %T", path, val))
				continue
			}
			if field.Fields != nil {
				warnings = append(warnings, validateFields(eventName, path+".", field.Fields, m)...)
			}
		case FieldTypeStringList:
			list, ok := val.([]any)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be string list, got %T", path, val))
				continue
			}
			for i, elem := range list {
				if _, ok := elem.(string); !ok {
					warnings = append(warnings, fmt.Sprintf("field %q[%d] should be string, got %T", path, i, elem))
				}
			}
		case FieldTypeAny:
			// No type check.
		}
	}

	return warnings
}
