package main

import (
	"fmt"
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

// FieldDef describes a single output field: validation constraints plus a
// one-line description consumed by the docs generator.
type FieldDef struct {
	Name   string
	Type   OutputFieldType
	Enum   []string   // If non-nil, value must be one of these strings.
	Doc    string     // One-line description for generated docs.
	Fields []FieldDef // For FieldTypeObject: nested schema. Nil means free-form object.
}

// EventDef defines a hook event. eventDefs is the single source of truth for
// event names, respond output validation, and the generated event docs.
type EventDef struct {
	Name    string
	Doc     string
	Fields  []FieldDef // Empty means the event is observe-only.
	Example string     // Optional markdown appended after the field tables in generated docs.
}

const fence = "```"

// hookSpecificOutputDef builds the FieldDef for the hookSpecificOutput wrapper
// object. hookEventName must equal the event name that triggered the hook.
func hookSpecificOutputDef(eventName string, fields ...FieldDef) FieldDef {
	all := append([]FieldDef{
		{Name: "hookEventName", Type: FieldTypeString, Enum: []string{eventName}, Doc: "Must match the event name"},
	}, fields...)
	return FieldDef{Name: "hookSpecificOutput", Type: FieldTypeObject, Doc: "Event-specific output wrapper", Fields: all}
}

var eventDefs = []EventDef{
	{
		Name: "PreToolUse",
		Doc:  "Fires before a tool executes. Use `matcher` to filter by tool name (e.g. `Bash`, `Write`, `Read`, `Edit`, `Glob`, `Grep`).",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"allow", "deny", "block"}, Doc: "`deny` = soft deny (agent may retry), `block` = hard block"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the agent"},
			{Name: "suppressOutput", Type: FieldTypeBool, Doc: "Suppress the tool's output from the conversation"},
			hookSpecificOutputDef("PreToolUse",
				FieldDef{Name: "permissionDecision", Type: FieldTypeString, Enum: []string{"allow", "deny", "ask", "defer"}, Doc: "Permission decision"},
				FieldDef{Name: "permissionDecisionReason", Type: FieldTypeString, Doc: "Explanation"},
				FieldDef{Name: "updatedInput", Type: FieldTypeObject, Doc: "Rewritten tool input (free-form)"},
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
		Example: "Respond example:\n" + fence + `yaml
action:
  respond:
    decision: block
    reason: "This command is not allowed"
` + fence + "\n\nTo rewrite tool input, use `hookSpecificOutput` with `updatedInput`:\n" + fence + `yaml
action:
  respond:
    hookSpecificOutput:
      hookEventName: PreToolUse
      permissionDecision: allow
      permissionDecisionReason: "rewritten for safety"
      updatedInput:
        command: "echo blocked"
` + fence,
	},
	{
		Name: "PostToolUse",
		Doc:  "Fires after a tool executes successfully. Use `matcher` to filter by tool name.",
		Fields: []FieldDef{
			{Name: "systemMessage", Type: FieldTypeString, Doc: "Message injected into the conversation as system context"},
			hookSpecificOutputDef("PostToolUse",
				FieldDef{Name: "updatedToolOutput", Type: FieldTypeAny, Doc: "Rewritten tool output"},
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "PermissionRequest",
		Doc:  "Fires when the agent requests permission for a tool use.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"allow", "deny", "block"}, Doc: "Permission decision"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation"},
			{Name: "updatedInput", Type: FieldTypeObject, Doc: "Rewritten tool input"},
			hookSpecificOutputDef("PermissionRequest",
				FieldDef{Name: "decision", Type: FieldTypeObject, Doc: "Structured permission decision", Fields: []FieldDef{
					{Name: "behavior", Type: FieldTypeString, Enum: []string{"allow", "deny"}, Doc: "Permission behavior"},
					{Name: "updatedInput", Type: FieldTypeObject, Doc: "Rewritten tool input (free-form)"},
					{Name: "permissionRules", Type: FieldTypeString, Doc: "Permission rules"},
				}},
			),
		},
	},
	{
		Name: "Stop",
		Doc:  "Fires when the agent is about to stop.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed (prevents stopping)"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation"},
			{Name: "updatedInput", Type: FieldTypeObject, Doc: "Rewritten input"},
			hookSpecificOutputDef("Stop",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "UserPromptSubmit",
		Doc:  "Fires when the user submits a prompt.",
		Fields: []FieldDef{
			{Name: "updatedInput", Type: FieldTypeString, Doc: "Rewritten user prompt"},
			{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context appended to the prompt"},
		},
		Example: "Respond example:\n" + fence + `yaml
action:
  respond:
    additionalContext: "Always run tests before deploying"
` + fence,
	},
	{
		Name: "SubagentStart",
		Doc:  "Fires when a subagent is about to start.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"allow", "deny", "block"}, Doc: "Whether to allow the subagent"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation"},
		},
	},
	{
		Name: "TaskCompleted",
		Doc:  "Fires when a task completes.",
		Fields: []FieldDef{
			{Name: "continue", Type: FieldTypeBool, Doc: "Whether to continue processing"},
			{Name: "stopReason", Type: FieldTypeString, Doc: "Reason for stopping"},
		},
	},
	{
		Name: "ConfigChange",
		Doc:  "Fires when configuration changes.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation"},
		},
	},
	{
		Name: "TeammateIdle",
		Doc:  "Fires when a teammate agent becomes idle.",
		Fields: []FieldDef{
			{Name: "continue", Type: FieldTypeBool, Doc: "Whether to continue"},
			{Name: "stopReason", Type: FieldTypeString, Doc: "Reason for stopping"},
		},
	},
	{
		Name: "SessionStart",
		Doc:  "Fires when a session begins.",
		Fields: []FieldDef{
			hookSpecificOutputDef("SessionStart",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Context injected at session start"},
				FieldDef{Name: "initialUserMessage", Type: FieldTypeString, Doc: "Initial user message"},
				FieldDef{Name: "sessionTitle", Type: FieldTypeString, Doc: "Session title"},
				FieldDef{Name: "watchPaths", Type: FieldTypeStringList, Doc: "Paths to watch"},
				FieldDef{Name: "reloadSkills", Type: FieldTypeBool, Doc: "Reload skills"},
			),
		},
	},
	{
		Name: "SubagentStop",
		Doc:  "Fires when a subagent stops.",
		Fields: []FieldDef{
			hookSpecificOutputDef("SubagentStop",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "Elicitation",
		Doc:  "Fires when the agent asks the user a question.",
		Fields: []FieldDef{
			hookSpecificOutputDef("Elicitation",
				FieldDef{Name: "action", Type: FieldTypeString, Enum: []string{"accept", "decline", "cancel"}, Doc: "Elicitation action"},
				FieldDef{Name: "content", Type: FieldTypeObject, Doc: "Form field values (free-form)"},
			),
		},
	},
	{
		Name: "ElicitationResult",
		Doc:  "Fires when the agent receives the response to a question.",
		Fields: []FieldDef{
			hookSpecificOutputDef("ElicitationResult",
				FieldDef{Name: "action", Type: FieldTypeString, Enum: []string{"accept", "decline", "cancel"}, Doc: "Elicitation action"},
				FieldDef{Name: "content", Type: FieldTypeObject, Doc: "Form field values (free-form)"},
			),
		},
	},
	{
		Name: "WorktreeCreate",
		Doc:  "Fires when a git worktree is created.",
		Fields: []FieldDef{
			hookSpecificOutputDef("WorktreeCreate",
				FieldDef{Name: "worktreePath", Type: FieldTypeString, Doc: "Path to the worktree"},
			),
		},
	},
	// Events with no output fields (observe-only)
	{Name: "Notification", Doc: "General notification from the agent"},
	{Name: "SessionEnd", Doc: "Session ends"},
	{Name: "PostToolUseFailure", Doc: "Tool execution failed"},
	{Name: "StopFailure", Doc: "Stop was blocked and failed"},
	{Name: "PreCompact", Doc: "Before conversation compaction"},
	{Name: "PostCompact", Doc: "After conversation compaction"},
	{Name: "InstructionsLoaded", Doc: "Instructions/CLAUDE.md loaded"},
	{Name: "WorktreeRemove", Doc: "Git worktree removed"},
}

// eventDefsByName indexes eventDefs by event name.
var eventDefsByName = buildEventIndex(eventDefs)

// validEventNames is the set of recognized event_name values, derived from eventDefs.
var validEventNames = buildEventNames(eventDefs)

func buildEventIndex(defs []EventDef) map[string]EventDef {
	m := make(map[string]EventDef, len(defs))
	for _, d := range defs {
		m[d.Name] = d
	}
	return m
}

func buildEventNames(defs []EventDef) map[string]bool {
	m := make(map[string]bool, len(defs))
	for _, d := range defs {
		m[d.Name] = true
	}
	return m
}

// ValidateRespondOutput validates a respond output object against the schema for the given event.
// Returns a list of warning messages (not errors, since the output may still work).
func ValidateRespondOutput(eventName string, respond map[string]any) []string {
	def, ok := eventDefsByName[eventName]
	if !ok {
		return nil // Unknown event, skip validation
	}
	return validateFields(eventName, "", def.Fields, respond)
}

// validateFields validates obj against fields, recursing into FieldTypeObject
// fields that declare a nested schema. prefix is the dotted path of obj within
// the respond object ("" at the root).
func validateFields(eventName, prefix string, fields []FieldDef, obj map[string]any) []string {
	var warnings []string

	// Check for unknown fields
	for key := range obj {
		if !slices.ContainsFunc(fields, func(f FieldDef) bool { return f.Name == key }) {
			warnings = append(warnings, fmt.Sprintf("unknown field %q for event %s", prefix+key, eventName))
		}
	}

	// Check field types and enum values
	for _, field := range fields {
		val, ok := obj[field.Name]
		if !ok {
			continue // Field is optional
		}
		path := prefix + field.Name
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
