# Event Types and Output Schemas

This is the authoritative reference for all hocage event types and their output schemas.
Source of truth: `schema.go`.

## hookSpecificOutput

Many events accept a `hookSpecificOutput` object for event-specific output (see each
event section below for its nested fields). Inside it, `hookEventName` must exactly
match the event name that triggered the hook (e.g. `PreToolUse`). `hocage hooks check`
validates nested fields, types, and enum values, using dotted paths in error messages
(e.g. `hookSpecificOutput.permissionDecision`).

## Events with Output Fields

### PreToolUse

Fires before a tool executes. Use `matcher` to filter by tool name (e.g. `Bash`, `Write`, `Read`, `Edit`, `Glob`, `Grep`).

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `decision` | string | `allow`, `deny`, `block` | `deny` = soft deny (agent may retry), `block` = hard block |
| `reason` | string | any | Explanation shown to the agent |
| `suppressOutput` | bool | `true`/`false` | Suppress the tool's output from the conversation |
| `hookSpecificOutput` | object | see below | Event-specific output wrapper |

`hookSpecificOutput` nested fields:

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `hookEventName` | string | `PreToolUse` | Must match the event name |
| `permissionDecision` | string | `allow`, `deny`, `ask`, `defer` | Permission decision |
| `permissionDecisionReason` | string | any | Explanation |
| `updatedInput` | object | any | Rewritten tool input (free-form) |
| `additionalContext` | string | any | Extra context for the agent |

Respond example:
```yaml
action:
  respond:
    decision: block
    reason: "This command is not allowed"
```

To rewrite tool input, use `hookSpecificOutput` with `updatedInput`:
```yaml
action:
  respond:
    hookSpecificOutput:
      hookEventName: PreToolUse
      permissionDecision: allow
      permissionDecisionReason: "rewritten for safety"
      updatedInput:
        command: "echo blocked"
```

### PostToolUse

Fires after a tool executes successfully. Use `matcher` to filter by tool name.

| Field | Type | Description |
|-------|------|-------------|
| `systemMessage` | string | Message injected into the conversation as system context |
| `hookSpecificOutput` | object | Event-specific output wrapper (see below) |

`hookSpecificOutput` nested fields:

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `hookEventName` | string | `PostToolUse` | Must match the event name |
| `updatedToolOutput` | string or object | any | Rewritten tool output |
| `additionalContext` | string | any | Extra context for the agent |

### PermissionRequest

Fires when the agent requests permission for a tool use.

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `decision` | string | `allow`, `deny`, `block` | Permission decision |
| `reason` | string | any | Explanation |
| `updatedInput` | object | any | Rewritten tool input |
| `hookSpecificOutput` | object | see below | Event-specific output wrapper |

`hookSpecificOutput` nested fields:

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `hookEventName` | string | `PermissionRequest` | Must match the event name |
| `decision` | object | see below | Structured permission decision |
| `decision.behavior` | string | `allow`, `deny` | Permission behavior |
| `decision.updatedInput` | object | any | Rewritten tool input (free-form) |
| `decision.permissionRules` | string | any | Permission rules |

### Stop

Fires when the agent is about to stop.

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `decision` | string | `block` | Only `block` is allowed (prevents stopping) |
| `reason` | string | any | Explanation |
| `updatedInput` | object | any | Rewritten input |
| `hookSpecificOutput` | object | see below | Event-specific output wrapper |

`hookSpecificOutput` nested fields:

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `hookEventName` | string | `Stop` | Must match the event name |
| `additionalContext` | string | any | Extra context for the agent |

### UserPromptSubmit

Fires when the user submits a prompt.

| Field | Type | Description |
|-------|------|-------------|
| `updatedInput` | string | Rewritten user prompt |
| `additionalContext` | string | Extra context appended to the prompt |

Respond example:
```yaml
action:
  respond:
    additionalContext: "Always run tests before deploying"
```

### SubagentStart

Fires when a subagent is about to start.

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `decision` | string | `allow`, `deny`, `block` | Whether to allow the subagent |
| `reason` | string | any | Explanation |

### TaskCompleted

Fires when a task completes.

| Field | Type | Description |
|-------|------|-------------|
| `continue` | bool | Whether to continue processing |
| `stopReason` | string | Reason for stopping |

### ConfigChange

Fires when configuration changes.

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `decision` | string | `block` | Only `block` is allowed |
| `reason` | string | any | Explanation |

### TeammateIdle

Fires when a teammate agent becomes idle.

| Field | Type | Description |
|-------|------|-------------|
| `continue` | bool | Whether to continue |
| `stopReason` | string | Reason for stopping |

### SessionStart

Fires when a session begins. Output only via `hookSpecificOutput`:

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `hookEventName` | string | `SessionStart` | Must match the event name |
| `additionalContext` | string | any | Context injected at session start |
| `initialUserMessage` | string | any | Initial user message |
| `sessionTitle` | string | any | Session title |
| `watchPaths` | string list | any | Paths to watch |
| `reloadSkills` | bool | `true`/`false` | Reload skills |

### SubagentStop

Fires when a subagent stops. Output only via `hookSpecificOutput`:

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `hookEventName` | string | `SubagentStop` | Must match the event name |
| `additionalContext` | string | any | Extra context for the agent |

### Elicitation / ElicitationResult

Fire when the agent asks the user a question / receives the response. Output only via `hookSpecificOutput`:

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `hookEventName` | string | `Elicitation` or `ElicitationResult` | Must match the event name |
| `action` | string | `accept`, `decline`, `cancel` | Elicitation action |
| `content` | object | any | Form field values (free-form) |

### WorktreeCreate

Fires when a git worktree is created. Output only via `hookSpecificOutput`:

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `hookEventName` | string | `WorktreeCreate` | Must match the event name |
| `worktreePath` | string | any | Path to the worktree |

## Events with No Output Fields

These events are observe-only. Use them with `command` or `http` actions (not `respond`), or use `respond` with an empty object.

| Event | Description |
|-------|-------------|
| `Notification` | General notification from the agent |
| `SessionEnd` | Session ends |
| `PostToolUseFailure` | Tool execution failed |
| `StopFailure` | Stop was blocked and failed |
| `PreCompact` | Before conversation compaction |
| `PostCompact` | After conversation compaction |
| `InstructionsLoaded` | Instructions/CLAUDE.md loaded |
| `WorktreeRemove` | Git worktree removed |

## Event Input Structure

All events receive a JSON object on stdin with at minimum:
```json
{ "hook_type": "<EventName>" }
```

Tool events (PreToolUse, PostToolUse, etc.) also include:
```json
{
  "hook_type": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": { "command": "ls -la" }
}
```

UserPromptSubmit includes:
```json
{
  "hook_type": "UserPromptSubmit",
  "prompt": "the user's prompt text"
}
```

Access these via `event.hook_type`, `event.tool_name`, `event.tool_input.command`, etc. in CEL expressions.
