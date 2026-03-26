---
name: hocage
description: >
  ALWAYS use when: writing or editing hocage YAML config files (.hocage.yaml),
  writing CEL expressions for Claude Code hooks, debugging hook conditions or
  test failures, using hocage CLI commands (check, test, run, list, generate),
  understanding hook event types or output schemas, creating hook policies for
  Claude Code, or any task involving the hocage tool in this repository.
  This skill provides the complete reference for hocage's config format, CEL
  functions, event types, output schemas, and CLI usage.
---

# hocage — Coding Agent Hooks Policy Framework Using CEL

## Overview

hocage evaluates CEL (Common Expression Language) expressions against Claude Code hook events. If the `when` expression evaluates to true, hocage executes the configured action. All config lives in `.hocage.yaml`.

## Workflow

1. **Write config** — Define hooks in `.hocage.yaml`
2. **Check** — `hocage hooks check` validates CEL syntax and runs heuristics
3. **Test** — `hocage hooks test` runs inline test cases
4. **Dry run** — `hocage hooks run <name> --dry-run` with piped JSON to verify behavior
5. **Generate** — `hocage hooks generate` produces the `hooks` section for Claude Code `settings.json`
6. **Apply** — Paste generated JSON into your Claude Code settings

## Config Format

```yaml
hooks:
  <hook_name>:
    event_name: <EventName>       # required — see Event Types reference
    matcher: <tool_name>          # optional — tool name filter for PreToolUse/PostToolUse
    priority: <int>               # optional — lower runs first in generated settings (default: 0)
    transcript:                    # optional — load session transcript
      load: <bool>                # enable transcript loading
      order: <string>             # "chronological" (default) or "reverse"
    when: <cel_expression>        # required — must evaluate to bool
    action:                       # required — exactly ONE of respond/command/http
      respond: <object>           # JSON object serialized to stdout
      # OR
      command: <string>           # shell command to execute
      stdin: <string>             # optional — pipe to command (only with command)
      # OR
      http:                       # HTTP request with event JSON body
        url: <string>             # required
        method: <string>          # optional (default: POST)
        headers:                  # optional
          <key>: <value>
        timeout: <duration>       # optional (default: 10s, e.g. "5s", "30s")
    tests:                        # optional — inline test cases
      <test_name>:
        transcript: <string>      # optional — inline JSONL transcript for testing
        transcript_file: <path>   # optional — path to JSONL transcript file
        inputs:                   # list of event JSON objects
          - <event_object>
        result:
          stdout: <object>        # expected output (omit when `when` is false)
```

**Validation rules:**
- `event_name` and `when` are required
- Exactly one of `respond`, `command`, or `http` must be set
- `stdin` requires `command`
- `http` requires `url`

## Action Types

| Action | When to use | Output |
|--------|------------|--------|
| `respond` | Return structured JSON to Claude Code (decisions, messages) | JSON to stdout |
| `command` | Run external tools (formatters, notifiers, scripts) | Command stdout/stderr |
| `http` | Send webhooks or API calls | HTTP response (event JSON as body) |

## CEL Variables

### `event` — The hook event (stdin JSON)

Access fields with dot notation: `event.hook_type`, `event.tool_name`, `event.tool_input.command`.

Tool events include: `hook_type`, `tool_name`, `tool_input` (object with tool-specific fields).
UserPromptSubmit includes: `hook_type`, `prompt`.

### `transcript` — Session transcript entries

A `list(dyn)` of JSONL entries from the Claude Code session. Empty list `[]` when `transcript.load` is not enabled. Each entry is a dynamic map — use `has()` to check field existence before access.

### `ctx` — Execution context

| Field | Type | Description |
|-------|------|-------------|
| `ctx.cwd` | string | Current working directory |
| `ctx.project_root` | string | Git repository root (empty if not in a repo) |

## Expression Interpolation

Use `{{expr}}` in `command`, `stdin`, `respond` string values, and `http` url/headers:

```yaml
command: "gofmt -w {{event.tool_input.file_path}}"
stdin: "{{to_json(event)}}"
respond:
  reason: "{{event.tool_input.command}} is not allowed"
http:
  url: "https://hooks.example.com/{{event.tool_name}}"
  headers:
    X-Hook-Event: "{{event.hook_type}}"
```

CEL expressions inside `{{}}` have access to `event` and `ctx`, same as `when`.

## CLI Commands

| Command | Description |
|---------|-------------|
| `hocage hooks check` | Validate CEL syntax, types, and heuristic checks |
| `hocage hooks test` | Run inline test cases from config |
| `hocage hooks run <name>` | Execute a single hook (reads event JSON from stdin) |
| `hocage hooks run <name> --dry-run` | Evaluate `when` without executing the action |
| `hocage hooks list` | List all hooks defined in config |
| `hocage hooks generate` | Generate Claude Code `settings.json` hooks section |

Default config path: `.hocage.yaml`. Override with `--config` / `-c` flag.

Multiple config files can be specified with repeated `-c` flags or glob patterns. When the same hook name appears in multiple files, the last one wins.

## Gotchas

1. **`respond` vs `command` vs `http` are mutually exclusive.** Config validation fails if more than one is set.
2. **`stdin` only works with `command`.** It is invalid with `respond` or `http`.
3. **`when` must return bool.** A non-bool return is a runtime error, not a false.
4. **Test `result:` for no-match must be empty (null).** When `when` is false, no action executes and no output is produced. Set `result:` with no value.
5. **`matcher` is for `generate` only.** hocage itself uses `when` for all filtering. The `matcher` field only affects the generated Claude Code settings JSON.
6. **`glob_exists` does not support `**`.** It uses Go's `filepath.Glob` which only supports single-level wildcards.
7. **YAML quoting for CEL.** Expressions with `!`, `:`, `{`, `}`, `#` or leading `>` need quoting. Use single quotes or `>-` block scalar.
8. **`hookSpecificOutput` for `updatedInput` in PreToolUse.** To rewrite tool input, nest under `hookSpecificOutput` with `hookEventName`, `permissionDecision`, `permissionDecisionReason`, and `updatedInput`.
9. **Priority ordering.** Lower `priority` values run first. Default is 0. Hooks with equal priority have undefined order.
10. **`event` is dynamically typed.** Field access typos (e.g. `event.promt` instead of `event.prompt`) are not caught by `hocage hooks check` — they only fail at runtime. Double-check field names against the event input structure.
11. **`transcript` is `list(dyn)`.** Field access typos on transcript entries (e.g. `t.commad`) are runtime errors. Always use `has()` to check field existence before accessing transcript entry fields.
12. **CEL has no negative indexing.** Use `order: reverse` so `transcript[0]` is the most recent entry instead of trying `transcript[-1]`.
13. **`has()` is required for transcript fields.** Transcript entries have varying structure (user messages, tool calls, errors). Always guard with `has(t.tool)`, `has(t.input)`, etc.
14. **`check` validates transcript/transcript_file in tests.** `hocage hooks check` verifies that inline `transcript` and `transcript_file` values are valid JSONL.

## References

For detailed information, read these reference files:

- **Event types and output schemas:** `references/event-types-and-output.md` — all event types, output fields, allowed values, and examples
- **CEL functions:** `references/cel-functions.md` — all custom hocage functions + standard CEL extensions
- **Patterns:** `references/patterns.md` — common recipes with complete YAML examples
