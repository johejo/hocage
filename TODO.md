# TODO

Missing features extracted from a comparison analysis with cchook, prioritized by importance.

## 1. Additional Event Types

**Summary:** agcel currently supports 7 event types (PreToolUse, PostToolUse, Stop, UserPromptSubmit, SubagentStop, PreSubagentTool, Notification), but the following Claude Code events are not yet supported:

- `SessionStart` — Initialization at session start (environment checks, logging, etc.)
- `SessionEnd` — Cleanup at session end
- `PermissionRequest` — Dynamic control of tool execution permissions
- `SubagentStart` — Policy enforcement at subagent launch
- `PreCompact` — Intervention before context compaction

**Background:** `SessionStart` and `PermissionRequest` are particularly important in practice. SessionStart enables per-session environment validation, and PermissionRequest is needed for dynamic allow/deny control of tool execution.

**Implementation:** Add to `validEventNames` in `config.go` and define the corresponding CEL context variables in `celctx.go`.

## 2. Output Schema Validation

**Summary:** There is no mechanism to verify that `respond` action output conforms to the schema expected by Claude Code.

**Background:** Currently, users are responsible for correctly formatting output. Invalid output is silently ignored or causes errors. cchook provides JSON Schema-based output validation.

**Implementation:** Define expected output schemas per event type. Validate during `hooks test` and `hooks run` execution. Also enable static validation in `hooks check`.

## 3. Command Action stdin Support

**Summary:** There is no way to pass data to external commands via stdin in `command` actions.

**Background:** When passing complex structured data (e.g., full event input) to external commands, command-line arguments and environment variables have limitations. Piping via stdin is safer and more flexible.

**Implementation:** Add an option to pipe data (CEL expression result or full event JSON) to stdin when executing a `command` action in `action.go`. Example config:

```yaml
action:
  command: "my-validator"
  stdin: "{{to_json(event)}}"
```

## 4. Multiple Hook Result Merge Strategy

**Summary:** The merge strategy for results when multiple hooks match the same event is undefined.

**Background:** Currently, multiple hooks execute independently, but there is no control over which result is returned to Claude Code when multiple `respond` actions fire. For example, merging is needed when one hook returns `reason` and another returns `updatedInput`.

**Implementation:** Allow setting a priority on hooks and make the merge strategy (first-match, merge-all, last-wins, etc.) configurable.

## 5. Dry-run Mode

**Summary:** A mode to preview hook execution results without actually applying them.

**Background:** During policy development and debugging, it is useful to verify hook behavior without affecting a live Claude Code session. `hooks test` covers inline tests, but a separate dry-run against actual event streams is needed.

**Implementation:** Add a `--dry-run` flag to `hooks run`. Skip action execution and display only the evaluation result. For `command` actions, show the command string; for `respond` actions, show the output JSON.
