package main

import (
	"fmt"
	"strings"
)

// OutputFieldType represents the expected type of an output field.
type OutputFieldType int

const (
	FieldTypeString OutputFieldType = iota
	FieldTypeBool
	FieldTypeObject
)

func (t OutputFieldType) String() string {
	switch t {
	case FieldTypeString:
		return "string"
	case FieldTypeBool:
		return "bool"
	case FieldTypeObject:
		return "object"
	default:
		return "unknown"
	}
}

// OutputField describes a single field in an output schema.
type OutputField struct {
	Type OutputFieldType
	Enum []string // If non-nil, value must be one of these strings.
}

// OutputSchema defines the expected output fields for an event type.
type OutputSchema struct {
	Fields map[string]OutputField
}

// outputSchemas maps event names to their expected respond output schema.
var outputSchemas = map[string]*OutputSchema{
	"PreToolUse": {
		Fields: map[string]OutputField{
			"decision":       {Type: FieldTypeString, Enum: []string{"allow", "deny", "block"}},
			"reason":         {Type: FieldTypeString},
			"suppressOutput": {Type: FieldTypeBool},
		},
	},
	"PostToolUse": {
		Fields: map[string]OutputField{
			"systemMessage": {Type: FieldTypeString},
		},
	},
	"PermissionRequest": {
		Fields: map[string]OutputField{
			"decision":     {Type: FieldTypeString, Enum: []string{"allow", "deny", "block"}},
			"reason":       {Type: FieldTypeString},
			"updatedInput": {Type: FieldTypeObject},
		},
	},
	"Stop": {
		Fields: map[string]OutputField{
			"decision":     {Type: FieldTypeString, Enum: []string{"block"}},
			"reason":       {Type: FieldTypeString},
			"updatedInput": {Type: FieldTypeObject},
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
	// Events with no output fields
	"Notification":      {Fields: map[string]OutputField{}},
	"SessionStart":      {Fields: map[string]OutputField{}},
	"SessionEnd":        {Fields: map[string]OutputField{}},
	"SubagentStop":      {Fields: map[string]OutputField{}},
	"PostToolUseFailure": {Fields: map[string]OutputField{}},
	"StopFailure":       {Fields: map[string]OutputField{}},
}

// ValidateRespondOutput validates a respond output object against the schema for the given event.
// Returns a list of warning messages (not errors, since the output may still work).
func ValidateRespondOutput(eventName string, respond map[string]any) []string {
	schema, ok := outputSchemas[eventName]
	if !ok {
		return nil // Unknown event, skip validation
	}

	var warnings []string

	// Check for unknown fields
	for key := range respond {
		if _, ok := schema.Fields[key]; !ok {
			warnings = append(warnings, fmt.Sprintf("unknown field %q for event %s", key, eventName))
		}
	}

	// Check field types and enum values
	for name, field := range schema.Fields {
		val, ok := respond[name]
		if !ok {
			continue // Field is optional
		}
		switch field.Type {
		case FieldTypeString:
			s, ok := val.(string)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be string, got %T", name, val))
				continue
			}
			if len(field.Enum) > 0 && !interpolateRe.MatchString(s) {
				found := false
				for _, v := range field.Enum {
					if s == v {
						found = true
						break
					}
				}
				if !found {
					warnings = append(warnings, fmt.Sprintf("field %q value %q not in allowed values: %s", name, s, strings.Join(field.Enum, ", ")))
				}
			}
		case FieldTypeBool:
			if _, ok := val.(bool); !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be bool, got %T", name, val))
			}
		case FieldTypeObject:
			if _, ok := val.(map[string]any); !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be object, got %T", name, val))
			}
		}
	}

	return warnings
}

