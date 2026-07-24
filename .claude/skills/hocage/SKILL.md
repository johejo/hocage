---
name: hocage
description: >
  ALWAYS use when working with hocage: writing/editing .hocage.yaml configs,
  writing CEL expressions for Claude Code hooks, debugging hook conditions or
  tests, or using hocage CLI commands (check, test, run, list, generate, docs).
---

# hocage — Coding Agent Hooks Policy Framework Using CEL

hocage evaluates CEL expressions against Claude Code hook events. If the `when` expression is true, hocage executes the configured action. Config lives in `.hocage.yaml`.

**Workflow:** write config → `hocage hooks check` → `hocage hooks test` → `hocage hooks run <name> --dry-run` → `hocage hooks generate` → paste into Claude Code `settings.json`.

## Config Format

```yaml
hooks:
  <hook_name>:
    event_name: <EventName>       # required — see event-types reference
    matcher: <tool_name>          # optional — tool filter, used only by `generate`
    priority: <int>               # optional — lower runs first (default: 0)
    transcript:                   # optional — load session transcript
      load: <bool>
      order: <string>             # "chronological" (default) or "reverse"
    when: <cel_expression>        # required — must evaluate to bool
    action:                       # required — exactly ONE of:
      respond: <object>           #   JSON to stdout ({cel: <expr>} nodes evaluated, typed)
      command: <string|list>      #   shell command string (sh -c) or argv list (no shell)
      env: { <NAME>: <expr> }     #   optional, only with command — values are CEL exprs
      stdin: <string|node>        #   optional, only with command — literal or {cel: <expr>}
      http:                       #   webhook — event JSON as request body
        url: <string|node>        #   required — literal or {cel: <expr>}
        method: <string>          #   default: POST
        headers: { <k>: <v> }     #   values: literal or {cel: <expr>}
        timeout: <duration>       #   default: 10s
    tests:                        # optional — inline test cases
      <test_name>:
        transcript: <string>      # inline JSONL transcript
        transcript_file: <path>   # or path to JSONL file
        inputs: [ <event_object>, ... ]
        result:
          stdout: <object>        # expected output; empty result: = no match
```

A `command` action's exit code and stderr pass through hocage unchanged, so the Claude Code exit-code protocol works from commands (exit 2 = blocking error, stderr fed to Claude); stdout carries only the command's stdout.

## CEL Variables

- `event` — the hook event (stdin JSON). Dot access: `event.hook_event_name`, `event.tool_name`, `event.tool_input.command`, `event.prompt` (UserPromptSubmit). Common fields on every event: `session_id`, `transcript_path`, `cwd`, `permission_mode`.
- `transcript` — `list(dyn)` of session JSONL entries; `[]` unless `transcript.load: true`. Prefer `tool_calls(transcript)` / `user_messages(transcript)` over navigating raw entries (see transcript-patterns reference).
- `ctx` — `ctx.cwd` (working directory), `ctx.project_root` (git root, empty if not in a repo).

## Embedding Expressions in Actions

Plain strings in actions are ALWAYS literal — there is no `{{expr}}` template syntax (removed; leftover `{{...}}` in a config is a load error). Embed event data with an **expression node** `{cel: "<expr>"}`, which has access to `event`, `ctx`, and `transcript`:

- In `respond`, a node can sit at any nesting level and yields the expression's **typed** result (string, number, bool, object, list) — e.g. `updatedInput` fields can be real objects.
- `stdin`, `http.url`, and `http` header values take a literal string or one node; the result must be a string (wrap with `to_json(...)` otherwise).
- `command` text is always literal. Event data reaches a shell command only via `env:` (values are bare CEL expressions, exported as environment variables), or via node elements in the argv-list form. A node cannot expand into multiple argv words.

```yaml
# respond: compose text inside CEL (+ or format())
permissionDecisionReason:
  cel: 'event.tool_input.command + " is not allowed"'

# command: shell form — reference event data as "$VAR", never splice it
command: 'gofmt -w "$FILE"'
env:
  FILE: event.tool_input.file_path

# command: argv form — no shell at all
command: ["gofmt", "-w", { cel: event.tool_input.file_path }]

# stdin: non-string results need to_json
stdin: { cel: to_json(event) }
```

## CLI Commands

<!-- gen:cli-table:start -->
| Command | Description |
|---------|-------------|
| `hocage docs [topic]` | Show embedded documentation (flags: `--output-dir` dump all docs to directory; `--overwrite-frontmatter` overwrite existing frontmatter when dumping) |
| `hocage hooks run <hook_name>` | Run a hook (reads event JSON on stdin) (flags: `--dry-run` skip executing the action) |
| `hocage hooks check` | Validate config and CEL expressions |
| `hocage hooks test` | Run inline test cases |
| `hocage hooks list` | List all hooks defined in the config |
| `hocage hooks generate` | Generate Claude Code settings.json hooks section (flags: `--merge` merge with existing JSON file; `--output` output file (reads for merge if exists, writes with -f); `--force` write to output file (requires -o)) |
<!-- gen:cli-table:end -->

Config discovery without `--config`/`-c`: `$XDG_CONFIG_HOME/hocage/*.yaml` (fallback `~/.config/hocage/*.yaml`), then `.hocage.yaml` in CWD; files merge in order, last wins. With `-c` (repeatable, globs OK), only those paths are used.

## Gotchas

1. **Exactly one of `respond`/`command`/`http`.** `stdin` and `env` only with `command`; `http` requires `url`.
2. **`when` must return bool** — non-bool is a runtime error, not false.
3. **Test `result:` for no-match must be empty (null)** — no action means no output.
4. **`matcher` only affects `generate` output.** hocage itself filters via `when`.
5. **`event` and `transcript` entries are dynamically typed.** Field typos (`event.promt`) pass `check` and fail at runtime. Raw transcript entries vary in shape (non-message lines like `mode` are interleaved; tool calls hide inside `message.content[]`) — use `tool_calls(transcript)` / `user_messages(transcript)`, and guard any raw access with `has()`.
6. **No negative indexing in CEL.** Use `transcript.order: reverse` so `transcript[0]` (or `tool_calls(transcript)[0]`) is the most recent entry.
7. **YAML quoting for CEL.** Expressions with `!`, `:`, `{`, `}`, `#` or leading `>` need single quotes or `>-` block scalar.
8. **PreToolUse has no top-level `decision`.** Allow/deny/ask via `hookSpecificOutput` with `hookEventName`, `permissionDecision`, `permissionDecisionReason`; rewrite input by adding `updatedInput`.
9. **`glob_exists` does not support `**`** (Go `filepath.Glob`, single-level wildcards only).

## References

Read ONLY the reference you need — do not read all of them up front:

- `references/event-types-and-output.md` — read when using an event other than PreToolUse/PostToolUse/UserPromptSubmit, or to check the top-level `respond` output fields (event-specific fields are not validated; see the official Claude Code hooks docs for those).
- `references/cel-functions.md` — read when using custom functions (`sh_*`, `git_*`, `path_*`, `read_file`, `env`, …) or CEL extensions beyond basic operators.
- `references/patterns.md` — read when writing a new hook from a recipe (blocking commands, formatting, webhooks, input rewriting).
- `references/transcript-patterns.md` — read when writing hooks that use the session `transcript` (stateful policies, rate limits, auto-allow).
