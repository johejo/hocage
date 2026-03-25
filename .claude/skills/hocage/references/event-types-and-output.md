# Event Types and Output Schemas

This is the authoritative reference for all hocage event types and their output schemas.
Source of truth: `schema.go`.

## Events with Output Fields

### PreToolUse

Fires before a tool executes. Use `matcher` to filter by tool name (e.g. `Bash`, `Write`, `Read`, `Edit`, `Glob`, `Grep`).

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `decision` | string | `allow`, `deny`, `block` | `deny` = soft deny (agent may retry), `block` = hard block |
| `reason` | string | any | Explanation shown to the agent |
| `suppressOutput` | bool | `true`/`false` | Suppress the tool's output from the conversation |

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

Respond example:
```yaml
action:
  respond:
    systemMessage: "Remember to run tests after modifying code"
```

### PermissionRequest

Fires when the agent requests permission for a tool use.

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `decision` | string | `allow`, `deny`, `block` | Permission decision |
| `reason` | string | any | Explanation |
| `updatedInput` | object | any | Rewritten tool input |

### Stop

Fires when the agent is about to stop.

| Field | Type | Allowed Values | Description |
|-------|------|----------------|-------------|
| `decision` | string | `block` | Only `block` is allowed (prevents stopping) |
| `reason` | string | any | Explanation |
| `updatedInput` | object | any | Rewritten input |

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

## Events with No Output Fields

These events are observe-only. Use them with `command` or `http` actions (not `respond`), or use `respond` with an empty object.

| Event | Description |
|-------|-------------|
| `Notification` | General notification from the agent |
| `SessionStart` | Session begins |
| `SessionEnd` | Session ends |
| `SubagentStop` | Subagent stops |
| `PostToolUseFailure` | Tool execution failed |
| `StopFailure` | Stop was blocked and failed |
| `PreCompact` | Before conversation compaction |
| `PostCompact` | After conversation compaction |
| `InstructionsLoaded` | Instructions/CLAUDE.md loaded |
| `Elicitation` | Agent asks user a question |
| `ElicitationResult` | User responds to elicitation |
| `WorktreeCreate` | Git worktree created |
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
