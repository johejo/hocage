package main

import (
	"fmt"
	"maps"
	"slices"
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

// FieldDef describes a single respond output field.
type FieldDef struct {
	Name string
	Type OutputFieldType
	Doc  string // one-line description for generated docs
}

// EventDef defines a hook event: its name and a one-line description for
// generated docs.
type EventDef struct {
	Name string
	Doc  string
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

// knownRespondFields are the recognized top-level respond keys. Event-specific
// content (the inside of hookSpecificOutput, decision values) is not validated,
// so hocage need not track upstream protocol changes.
var knownRespondFields = append(slices.Clone(commonOutputFields), []FieldDef{
	{Name: "decision", Type: FieldTypeString, Doc: "Event-specific decision (e.g. `block`)"},
	{Name: "reason", Type: FieldTypeString, Doc: "Explanation accompanying `decision`"},
	{Name: "hookSpecificOutput", Type: FieldTypeObject, Doc: "Event-specific output wrapper (free-form; set `hookEventName` to the event name)"},
}...)

var eventDefs = []EventDef{
	{Name: "PreToolUse", Doc: "Fires before a tool executes. Use `matcher` to filter by tool name (e.g. `Bash`, `Write`, `Read`, `Edit`, `Glob`, `Grep`)."},
	{Name: "PostToolUse", Doc: "Fires after a tool executes successfully. Use `matcher` to filter by tool name."},
	{Name: "PostToolUseFailure", Doc: "Fires after a tool execution fails."},
	{Name: "PostToolBatch", Doc: "Fires after a batch of parallel tool calls completes."},
	{Name: "PermissionRequest", Doc: "Fires when the agent requests permission for a tool use."},
	{Name: "PermissionDenied", Doc: "Fires after a permission request is denied."},
	{Name: "UserPromptSubmit", Doc: "Fires when the user submits a prompt."},
	{Name: "UserPromptExpansion", Doc: "Fires when a skill or slash command expands a user prompt."},
	{Name: "Stop", Doc: "Fires when the agent is about to stop."},
	{Name: "SubagentStart", Doc: "Fires when a subagent starts. Context only — cannot block."},
	{Name: "TaskCreated", Doc: "Fires when a task is created."},
	{Name: "TaskCompleted", Doc: "Fires when a task completes."},
	{Name: "ConfigChange", Doc: "Fires when configuration changes."},
	{Name: "TeammateIdle", Doc: "Fires when a teammate agent becomes idle. Block with the common `continue: false` + `stopReason`."},
	{Name: "SessionStart", Doc: "Fires when a session begins."},
	{Name: "SubagentStop", Doc: "Fires when a subagent stops."},
	{Name: "Setup", Doc: "Fires on `claude --init` / setup runs (trigger: `init` or `maintenance`)."},
	{Name: "PreCompact", Doc: "Fires before conversation compaction (trigger: `manual` or `auto`)."},
	{Name: "PostCompact", Doc: "Fires after conversation compaction."},
	{Name: "MessageDisplay", Doc: "Fires when an assistant message is displayed. Rewrites the on-screen text only."},
	{Name: "Elicitation", Doc: "Fires when the agent asks the user a question."},
	{Name: "ElicitationResult", Doc: "Fires when the agent receives the response to a question."},
	{Name: "WorktreeCreate", Doc: "Fires when a git worktree is created."},
	{Name: "Notification", Doc: "General notification from the agent."},
	{Name: "SessionEnd", Doc: "Session ends."},
	{Name: "StopFailure", Doc: "The agent stopped due to an error (rate limit, billing, ...)."},
	{Name: "InstructionsLoaded", Doc: "Instructions/CLAUDE.md loaded."},
	{Name: "FileChanged", Doc: "A watched file changed (see SessionStart `watchPaths`)."},
	{Name: "CwdChanged", Doc: "The working directory changed."},
	{Name: "WorktreeRemove", Doc: "Git worktree removed."},
}

// validEventNames is the set of known event_name values; `hocage hooks check`
// warns about names outside it.
var validEventNames = buildEventNames(eventDefs)

func buildEventNames(defs []EventDef) map[string]bool {
	m := make(map[string]bool, len(defs))
	for _, d := range defs {
		m[d.Name] = true
	}
	return m
}

// ValidateRespondOutput checks respond against knownRespondFields and returns
// warnings (not errors, since the output may still work).
func ValidateRespondOutput(respond map[string]any) []string {
	var warnings []string

	for _, key := range slices.Sorted(maps.Keys(respond)) {
		if !slices.ContainsFunc(knownRespondFields, func(f FieldDef) bool { return f.Name == key }) {
			warnings = append(warnings, fmt.Sprintf("unknown respond field %q", key))
		}
	}

	for _, field := range knownRespondFields {
		val, ok := respond[field.Name]
		if !ok {
			continue
		}
		// An unresolved {cel: ...} node can produce any type at runtime.
		if _, isNode, _ := exprNode(val); isNode {
			continue
		}
		switch field.Type {
		case FieldTypeString:
			if _, ok := val.(string); !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be string, got %T", field.Name, val))
			}
		case FieldTypeBool:
			if _, ok := val.(bool); !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be bool, got %T", field.Name, val))
			}
		case FieldTypeObject:
			if _, ok := val.(map[string]any); !ok {
				warnings = append(warnings, fmt.Sprintf("field %q should be object, got %T", field.Name, val))
			}
		}
	}

	return warnings
}
