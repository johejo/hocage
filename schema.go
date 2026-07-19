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

// commonOutputFields are accepted at the top level of every event's respond
// output, per the Claude Code hooks protocol.
var commonOutputFields = []FieldDef{
	{Name: "continue", Type: FieldTypeBool, Doc: "`false` stops all further processing (default `true`)"},
	{Name: "stopReason", Type: FieldTypeString, Doc: "Shown to the user when `continue` is `false`"},
	{Name: "suppressOutput", Type: FieldTypeBool, Doc: "Hide the hook's stdout from the transcript"},
	{Name: "systemMessage", Type: FieldTypeString, Doc: "Warning message shown to the user"},
	{Name: "terminalSequence", Type: FieldTypeString, Doc: "OSC escape sequence forwarded to the terminal (allowlisted)"},
}

var eventDefs = []EventDef{
	{
		Name: "PreToolUse",
		Doc:  "Fires before a tool executes. Use `matcher` to filter by tool name (e.g. `Bash`, `Write`, `Read`, `Edit`, `Glob`, `Grep`).",
		Fields: []FieldDef{
			hookSpecificOutputDef("PreToolUse",
				FieldDef{Name: "permissionDecision", Type: FieldTypeString, Enum: []string{"allow", "deny", "ask", "defer"}, Doc: "`allow` bypasses the permission prompt, `deny` blocks the call, `ask` forces the prompt, `defer` falls through to normal permission evaluation"},
				FieldDef{Name: "permissionDecisionReason", Type: FieldTypeString, Doc: "Explanation shown to the agent (deny) or user (allow/ask)"},
				FieldDef{Name: "updatedInput", Type: FieldTypeObject, Doc: "Rewritten tool input (free-form, replaces `tool_input`)"},
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
		Example: "Respond example (block a tool call):\n" + fence + `yaml
action:
  respond:
    hookSpecificOutput:
      hookEventName: PreToolUse
      permissionDecision: deny
      permissionDecisionReason: "This command is not allowed"
` + fence + "\n\nTo rewrite tool input, use `updatedInput`:\n" + fence + `yaml
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
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed (feeds `reason` back to the agent; the tool already ran)"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the agent"},
			hookSpecificOutputDef("PostToolUse",
				FieldDef{Name: "updatedToolOutput", Type: FieldTypeAny, Doc: "Rewritten tool output (replaces `tool_response`)"},
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "PostToolUseFailure",
		Doc:  "Fires after a tool execution fails.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the agent"},
			hookSpecificOutputDef("PostToolUseFailure",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "PostToolBatch",
		Doc:  "Fires after a batch of parallel tool calls completes.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the agent"},
			hookSpecificOutputDef("PostToolBatch",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "PermissionRequest",
		Doc:  "Fires when the agent requests permission for a tool use.",
		Fields: []FieldDef{
			hookSpecificOutputDef("PermissionRequest",
				FieldDef{Name: "decision", Type: FieldTypeObject, Doc: "Structured permission decision", Fields: []FieldDef{
					{Name: "behavior", Type: FieldTypeString, Enum: []string{"allow", "deny"}, Doc: "Permission behavior"},
					{Name: "updatedInput", Type: FieldTypeObject, Doc: "Rewritten tool input (free-form)"},
					{Name: "permissionRules", Type: FieldTypeStringList, Doc: "Permission rules to auto-approve"},
				}},
			),
		},
	},
	{
		Name: "PermissionDenied",
		Doc:  "Fires after a permission request is denied.",
		Fields: []FieldDef{
			hookSpecificOutputDef("PermissionDenied",
				FieldDef{Name: "retry", Type: FieldTypeBool, Doc: "Tell the agent it may retry the tool call"},
			),
		},
	},
	{
		Name: "UserPromptSubmit",
		Doc:  "Fires when the user submits a prompt.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed (erases the prompt, shows `reason` to the user)"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the user"},
			hookSpecificOutputDef("UserPromptSubmit",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context injected alongside the prompt"},
			),
		},
		Example: "Respond example:\n" + fence + `yaml
action:
  respond:
    hookSpecificOutput:
      hookEventName: UserPromptSubmit
      additionalContext: "Always run tests before deploying"
` + fence,
	},
	{
		Name: "UserPromptExpansion",
		Doc:  "Fires when a skill or slash command expands a user prompt.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the user"},
			hookSpecificOutputDef("UserPromptExpansion",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context injected alongside the expanded prompt"},
			),
		},
	},
	{
		Name: "Stop",
		Doc:  "Fires when the agent is about to stop.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed (prevents stopping, feeds `reason` to the agent)"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the agent"},
			hookSpecificOutputDef("Stop",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Non-blocking feedback for the agent"},
			),
		},
	},
	{
		Name: "SubagentStart",
		Doc:  "Fires when a subagent starts. Context only — cannot block.",
		Fields: []FieldDef{
			hookSpecificOutputDef("SubagentStart",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the subagent"},
			),
		},
	},
	{
		Name: "TaskCreated",
		Doc:  "Fires when a task is created.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the agent"},
			hookSpecificOutputDef("TaskCreated",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "TaskCompleted",
		Doc:  "Fires when a task completes.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the agent"},
			hookSpecificOutputDef("TaskCompleted",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "ConfigChange",
		Doc:  "Fires when configuration changes.",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation"},
			hookSpecificOutputDef("ConfigChange",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "TeammateIdle",
		Doc:  "Fires when a teammate agent becomes idle. Block with the common `continue: false` + `stopReason`.",
		Fields: []FieldDef{
			hookSpecificOutputDef("TeammateIdle",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the teammate"},
			),
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
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed (prevents the subagent from stopping)"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation shown to the subagent"},
			hookSpecificOutputDef("SubagentStop",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Non-blocking feedback for the subagent"},
			),
		},
	},
	{
		Name: "Setup",
		Doc:  "Fires on `claude --init` / setup runs (trigger: `init` or `maintenance`).",
		Fields: []FieldDef{
			hookSpecificOutputDef("Setup",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "PreCompact",
		Doc:  "Fires before conversation compaction (trigger: `manual` or `auto`).",
		Fields: []FieldDef{
			{Name: "decision", Type: FieldTypeString, Enum: []string{"block"}, Doc: "Only `block` is allowed (prevents compaction)"},
			{Name: "reason", Type: FieldTypeString, Doc: "Explanation"},
		},
	},
	{
		Name: "PostCompact",
		Doc:  "Fires after conversation compaction.",
		Fields: []FieldDef{
			hookSpecificOutputDef("PostCompact",
				FieldDef{Name: "additionalContext", Type: FieldTypeString, Doc: "Extra context for the agent"},
			),
		},
	},
	{
		Name: "MessageDisplay",
		Doc:  "Fires when an assistant message is displayed. Rewrites the on-screen text only.",
		Fields: []FieldDef{
			hookSpecificOutputDef("MessageDisplay",
				FieldDef{Name: "displayContent", Type: FieldTypeString, Doc: "Replaces the displayed text (transcript unchanged)"},
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
	// Events with no event-specific output fields (observe-only)
	{Name: "Notification", Doc: "General notification from the agent"},
	{Name: "SessionEnd", Doc: "Session ends"},
	{Name: "StopFailure", Doc: "The agent stopped due to an error (rate limit, billing, ...)"},
	{Name: "InstructionsLoaded", Doc: "Instructions/CLAUDE.md loaded"},
	{Name: "FileChanged", Doc: "A watched file changed (see SessionStart `watchPaths`)"},
	{Name: "CwdChanged", Doc: "The working directory changed"},
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
// The common output fields (continue, stopReason, ...) are accepted for every event.
// Returns a list of warning messages (not errors, since the output may still work).
func ValidateRespondOutput(eventName string, respond map[string]any) []string {
	def, ok := eventDefsByName[eventName]
	if !ok {
		return nil // Unknown event, skip validation
	}
	fields := append(slices.Clone(commonOutputFields), def.Fields...)
	return validateFields(eventName, "", fields, respond)
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
