# TODO

Missing features extracted from a comparison analysis with cchook and the official Claude Code hooks documentation, prioritized by importance.

## 1. Additional Event Types

**Summary:** agcel currently supports 12 event types. The official Claude Code documentation defines 22 event types total, leaving 10 unsupported.

### Missing events by priority

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

**Implementation:** Add to `validEventNames` in `config.go` and define the corresponding CEL context variables in `celctx.go`.

## 2. Multiple Hook Result Merge Strategy

**Summary:** The merge strategy for results when multiple hooks match the same event is undefined.

**Background:** Currently, multiple hooks execute independently, but there is no control over which result is returned to Claude Code when multiple `respond` actions fire. For example, merging is needed when one hook returns `reason` and another returns `updatedInput`.

**Implementation:** Allow setting a priority on hooks and make the merge strategy (first-match, merge-all, last-wins, etc.) configurable.

## 3. HTTP/Prompt/Agent Handler Types

**Summary:** The official Claude Code documentation defines 4 hook handler types: `command` (shell), `http` (POST endpoint), `prompt` (single-turn LLM), and `agent` (subagent with tools). agcel only supports `command`.

**Background:** The `http` handler type sends a POST request to a URL endpoint, which is useful for integrating with external services without a shell wrapper. The `prompt` and `agent` types enable LLM-powered hooks. cchooks (Python SDK) focuses only on `command` handlers as well, so agcel is not behind the Python ecosystem here, but supporting `http` at minimum would cover a common integration pattern.

**Implementation:** Consider adding `http` handler type support as a new action type. The `prompt` and `agent` types are lower priority as they are more specialized. For `http`, the action config would specify a URL and agcel would POST the event JSON to it and interpret the response.
