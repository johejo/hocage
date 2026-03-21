# TODO

Missing features extracted from a comparison analysis with cchook and the official Claude Code hooks documentation, prioritized by importance.

## 1. Additional Event Types

**Summary:** agcel currently supports 6 event types (PreToolUse, PostToolUse, Stop, UserPromptSubmit, SubagentStop, Notification). The official Claude Code documentation defines 22 event types total, leaving 16 unsupported.

### Missing events by priority

**High priority** — commonly needed for policy enforcement and session lifecycle: **DONE**

- ~~`SessionStart` — Initialization at session start (environment checks, logging, etc.)~~
- ~~`SessionEnd` — Cleanup at session end~~
- ~~`PermissionRequest` — Dynamic control of tool execution permissions~~
- ~~`SubagentStart` — Policy enforcement at subagent launch~~
- ~~`PostToolUseFailure` — React to tool execution failures~~
- ~~`StopFailure` — React to stop/completion failures~~

**Medium priority** — useful for context management and workflow automation:

- `PreCompact` — Intervention before context compaction
- `PostCompact` — Intervention after context compaction
- `TaskCompleted` — React to task completion events
- `InstructionsLoaded` — Modify or validate loaded instructions
- `ConfigChange` — React to configuration changes at runtime
- `Elicitation` — Intercept elicitation (question) events
- `ElicitationResult` — React to elicitation responses

**Low priority** — niche or less commonly needed:

- `TeammateIdle` — React when a teammate agent becomes idle
- `WorktreeCreate` — React to git worktree creation
- `WorktreeRemove` — React to git worktree removal

**Background:** `SessionStart` and `PermissionRequest` are particularly important in practice. SessionStart enables per-session environment validation, and PermissionRequest is needed for dynamic allow/deny control of tool execution. cchooks (Python SDK) also supports SessionStart, SessionEnd, and PreCompact beyond the base set agcel covers.

**Implementation:** Add to `validEventNames` in `config.go` and define the corresponding CEL context variables in `celctx.go`.

## 2. Output Schema Validation — DONE

**Summary:** There is no mechanism to verify that `respond` action output conforms to the schema expected by Claude Code.

**Background:** Currently, users are responsible for correctly formatting output. Invalid output is silently ignored or causes errors. The official docs and cchooks define typed output fields per event type. Key output fields from the official documentation include:

- `continue` — Whether to continue processing
- `stopReason` — Reason for stopping
- `suppressOutput` — Suppress tool output from display
- `systemMessage` — Inject a system message
- `decision` — Allow/deny decision for permission-related events
- `updatedInput` — Modified input to pass forward
- `additionalContext` — Extra context to inject

cchooks provides typed output methods (allow, deny, halt, etc.) that serve as a useful reference for the expected output shapes per event type.

**Implementation:** Define expected output schemas per event type. Validate during `hooks test` and `hooks run` execution. Also enable static validation in `hooks check`.

## 3. Command Action stdin Support — DONE

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

## 5. Dry-run Mode — DONE

**Summary:** A mode to preview hook execution results without actually applying them.

**Background:** During policy development and debugging, it is useful to verify hook behavior without affecting a live Claude Code session. `hooks test` covers inline tests, but a separate dry-run against actual event streams is needed.

**Implementation:** Add a `--dry-run` flag to `hooks run`. Skip action execution and display only the evaluation result. For `command` actions, show the command string; for `respond` actions, show the output JSON.

## 6. HTTP/Prompt/Agent Handler Types

**Summary:** The official Claude Code documentation defines 4 hook handler types: `command` (shell), `http` (POST endpoint), `prompt` (single-turn LLM), and `agent` (subagent with tools). agcel only supports `command`.

**Background:** The `http` handler type sends a POST request to a URL endpoint, which is useful for integrating with external services without a shell wrapper. The `prompt` and `agent` types enable LLM-powered hooks. cchooks (Python SDK) focuses only on `command` handlers as well, so agcel is not behind the Python ecosystem here, but supporting `http` at minimum would cover a common integration pattern.

**Implementation:** Consider adding `http` handler type support as a new action type. The `prompt` and `agent` types are lower priority as they are more specialized. For `http`, the action config would specify a URL and agcel would POST the event JSON to it and interpret the response.
