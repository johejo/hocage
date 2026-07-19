# hocage

Coding Agent Hooks Policy Framework Using CEL

Define declarative policies for [Claude Code hooks](https://code.claude.com/docs/en/hooks) using [CEL (Common Expression Language)](https://cel.dev/).

The name is a portmanteau of "hooks" and "cage", where "cage" doubles as an abbreviation for **C**oding **AGE**nt — caging your coding agent's hooks with declarative policies.

## Config

```yaml
hooks:
  <hook_name>:
    event_name: <event_name>   # e.g. PreToolUse, Stop, UserPromptSubmit
    matcher: <matcher>         # optional: tool name for PreToolUse (e.g. Bash, Write)
    priority: <int>            # optional: lower values run first in generated settings (default: 0)
    transcript:                # optional: load session transcript for stateful policy evaluation
      load: <bool>            # enable transcript loading
      order: <string>         # "chronological" (default) or "reverse"
    when: <cel_expression>     # CEL expression that evaluates to bool
    action:
      respond: <object>       # object serialized as JSON to stdout ({cel: <expr>} nodes are evaluated)
      # or
      command: <string|list>  # shell command string (run via sh -c) or argv list (no shell)
      env:                    # optional: env vars exported to command; values are CEL expressions
        <NAME>: <cel_expression>
      stdin: <string|node>    # optional: pipe input to command (literal string or {cel: <expr>})
      # or
      http:                    # send HTTP request with event JSON as body
        url: <string|node>    # required (literal string or {cel: <expr>})
        method: <string>      # optional: HTTP method (default: POST)
        headers:              # optional: HTTP headers (values: literal string or {cel: <expr>})
          <key>: <value>
        timeout: <duration>   # optional: request timeout (default: 10s, e.g. "5s", "30s")
    tests:
      <test_case_name>:
        transcript: <string>      # optional: inline JSONL transcript for testing
        transcript_file: <path>   # optional: path to JSONL transcript file
        inputs:               # list of stdin JSON inputs
          - <event_object>
        result:
          stdout: <object>    # expected stdout (compared as JSON)
          # stdout is omitted when `when` evaluates to false (action not executed)
```

`respond`, `command`, and `http` are mutually exclusive. Exactly one must be present.

### Config File Discovery

When `--config` / `-c` is **not** specified, hocage searches for config files in this order:

1. `$XDG_CONFIG_HOME/hocage/*.yaml` (falls back to `~/.config/hocage/*.yaml` if `$XDG_CONFIG_HOME` is unset)
2. `.hocage.yaml` in the current working directory

Files are merged in order — when the same hook name appears in multiple files, the **last one wins** (CWD overrides XDG). Missing directories or files are silently skipped.

When `--config` / `-c` **is** specified, only the explicitly provided paths are used (no XDG discovery).

### CEL Variable Bindings

| variable | type | description |
|----------|------|-------------|
| `event` | dynamic | The stdin JSON from Claude Code (event payload) |
| `ctx.cwd` | string | Current working directory |
| `ctx.project_root` | string | Git repository root (empty if not in a repo) |
| `transcript` | list | Session transcript entries (empty list if `transcript.load` is not enabled) |

For example, a PreToolUse event for the Bash tool receives:

```json
{
  "hook_event_name": "PreToolUse",
  "session_id": "...",
  "transcript_path": "/path/to/session.jsonl",
  "cwd": "/current/working/directory",
  "permission_mode": "default",
  "tool_name": "Bash",
  "tool_input": {
    "command": "rm -rf /"
  }
}
```

These fields are available as `event.hook_event_name`, `event.tool_name`, `event.tool_input.command`, etc.

### Transcript

Enable `transcript.load` to access the session transcript in CEL expressions. The `transcript` variable is a list of JSONL entries from the Claude Code session file at `transcript_path`.

```yaml
transcript:
  load: true
  order: reverse  # optional: "chronological" (default) or "reverse"
```

When `order` is `reverse`, transcript entries are reversed so that `transcript[0]` is the most recent entry. This is useful because CEL does not support negative indexing.

Transcript entries are heterogeneous: each line has a top-level `type` (`user`, `assistant`, `system`, plus non-message lines like `mode` and `file-history-snapshot`), and tool calls live inside assistant entries as `message.content[]` blocks. Use the `tool_calls(transcript)` and `user_messages(transcript)` functions to flatten this shape instead of navigating it by hand:

```cel
tool_calls(transcript).exists(c,
  c.name == "Bash" && c.input.command.contains("rm -rf"))
```

### Action

| field | description |
|-------|-------------|
| `respond` | Serializes an object as JSON to stdout. |
| `command` | Executes an external command. Optionally accepts `stdin` to pipe input to the command. |
| `http` | Sends an HTTP request with the event JSON as the body. Supports `url`, `method` (default: POST), `headers`, and `timeout` (default: 10s). |

hocage evaluates the CEL `when` expression and, if true, executes the action. The user is responsible for producing the correct output for the hook protocol (JSON format, exit code, etc.).

See the [Claude Code hooks documentation](https://code.claude.com/docs/en/hooks) for the expected output format per event.

### Embedding CEL Expressions in Actions

Plain strings in actions are always literal — there is no in-string template syntax. To embed event data, use an **expression node**: a mapping with the single key `cel` whose value is a CEL expression. The node is replaced by the expression's result.

In `respond`, an expression node can appear at any nesting level and produces a **typed** JSON value (string, number, bool, object, list):

```yaml
action:
  respond:
    hookSpecificOutput:
      hookEventName: PreToolUse
      permissionDecision: deny
      permissionDecisionReason:
        cel: 'event.tool_input.command + " is not allowed"'
```

The `command` field itself is always literal. Event data reaches a shell command only through `env:` — each value is a CEL expression exported as an environment variable — so shell metacharacters in event data are never parsed as shell syntax:

```yaml
action:
  command: 'gofmt -w "$FILE"'
  env:
    FILE: event.tool_input.file_path
```

Alternatively, `command` can be an argv list executed without a shell. List elements are literal strings or expression nodes:

```yaml
action:
  command: ["gofmt", "-w", { cel: event.tool_input.file_path }]
```

`stdin`, `http.url`, and `http` header values each accept a literal string or one expression node. These slots require a string result — wrap non-strings with `to_json(...)`:

```yaml
action:
  command: "cat"
  stdin: { cel: to_json(event) }
```

```yaml
action:
  http:
    url: { cel: '"https://hooks.example.com/" + event.tool_name' }
    headers:
      X-Hook-Event: { cel: event.hook_event_name }
```

The legacy `{{expr}}` in-string interpolation has been removed; configs still using it fail to load with a migration hint.

### Built-in CEL Functions

In addition to the [standard CEL functions](https://github.com/google/cel-spec/blob/master/doc/langdef.md#list-of-standard-definitions), hocage provides:

<!-- gen:cel-functions:start -->
`default`, `dir_exists`, `env`, `file_exists`, `from_entries`, `from_json`, `git_branch`, `git_ignored`, `git_modified`, `git_staged`, `git_tracked`, `glob_exists`, `has_key`, `indent`, `is_symlink`, `keys`, `max`, `min`, `path_base`, `path_clean`, `path_dir`, `path_ext`, `path_join`, `quote`, `read_file`, `read_file_ok`, `semver_compare`, `sh_argv`, `sh_commands`, `sh_valid`, `sh_words`, `sha256sum`, `squote`, `to_entries`, `to_json`, `tool_calls`, `trim_prefix`, `trim_suffix`, `user_messages`, `values`
<!-- gen:cel-functions:end -->

Full signatures, semantics, and caveats: `hocage docs cel`.

<!-- gen:cel-extensions:start -->
Standard CEL extensions are also enabled: `ext.Strings()`, `ext.Lists() (v3)`, `ext.Sets()`, `ext.Math()`, `ext.Encoders()`, `ext.Regex()`, `ext.Bindings()`, `ext.TwoVarComprehensions()`, `cel.OptionalTypes()`.
<!-- gen:cel-extensions:end -->

## Cookbook

### Block dangerous commands

```yaml
hooks:
  block_rm_rf:
    event_name: PreToolUse
    matcher: Bash
    # Parse the command with sh_words so quoted text like `echo "rm -rf /"` is
    # not misread as a real rm, and `sudo rm -rf` is still caught.
    when: >-
      "rm" in sh_words(event.tool_input.command)
      && sh_words(event.tool_input.command).exists(w, w.matches("^-[a-zA-Z]*[rR]") || w == "--recursive")
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "rm -rf is not allowed"
    tests:
      should_block:
        inputs:
          - tool_input: { command: "rm -rf /" }
          - tool_input: { command: "sudo rm -rf /tmp" }
          - tool_input: { command: "rm --recursive --force /" }
        result:
          stdout:
            hookSpecificOutput:
              hookEventName: PreToolUse
              permissionDecision: deny
              permissionDecisionReason: "rm -rf is not allowed"
      should_allow:
        inputs:
          - tool_input: { command: "ls -la" }
          - tool_input: { command: "rm file.txt" }
          - tool_input: { command: 'echo "rm -rf /"' }
```

### Block writes outside the project

```yaml
hooks:
  block_write_outside_project:
    event_name: PreToolUse
    matcher: Write
    when: '!event.tool_input.file_path.startsWith(ctx.project_root)'
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "Writing outside the project directory is not allowed"
```

### Auto-format Go files after write

```yaml
hooks:
  format_go:
    event_name: PostToolUse
    matcher: Write
    when: event.tool_input.file_path.endsWith(".go")
    action:
      command: 'gofmt -w "$FILE"'
      env:
        FILE: event.tool_input.file_path
```

### Rewrite tool input with `updatedInput`

```yaml
hooks:
  rewrite_command:
    event_name: PreToolUse
    matcher: Bash
    when: event.tool_input.command.contains("rm -rf")
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: allow
          permissionDecisionReason: "command rewritten for safety"
          updatedInput:
            command:
              cel: '"echo " + squote(event.tool_input.command) + " was blocked"'
```

### Inject context on user prompt

```yaml
hooks:
  remind_testing:
    event_name: UserPromptSubmit
    when: event.prompt.contains("deploy")
    action:
      respond:
        hookSpecificOutput:
          hookEventName: UserPromptSubmit
          additionalContext: "Remember to run tests before deploying"
```

### Send event to a webhook via HTTP

```yaml
hooks:
  webhook_notify:
    event_name: Stop
    when: "true"
    action:
      http:
        url: "https://hooks.example.com/claude-code"
        method: POST
        headers:
          Authorization: "Bearer my-token"
        timeout: "5s"
```

### Require Read before Edit

Prevent the agent from editing a file it has never read in the session, catching edits based on assumptions rather than actual file content:

```yaml
hooks:
  must_read_before_edit:
    event_name: PreToolUse
    transcript:
      load: true
    when: |
      event.tool_name == "Edit"
      && has(event.tool_input.file_path)
      && !tool_calls(transcript).exists(c,
        c.name == "Read"
        && has(c.input.file_path)
        && c.input.file_path == event.tool_input.file_path)
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "You must Read a file before editing it"
```

These are a starter set. Many more recipes — script inspection, rate limits,
retry-loop and thrashing detection, auto-allow via session history — live in
the embedded docs: `hocage docs patterns` and `hocage docs transcript-patterns`.

## CLI

<!-- gen:cli:start -->
Global flags:

- `--config`, `-c` — path to config file (can be specified multiple times, supports glob patterns; default: $XDG_CONFIG_HOME/hocage/*.yaml + .hocage.yaml)

### `hocage docs [topic]`

Show embedded documentation.

Shows the embedded skill documentation (.claude/skills/hocage) from the CLI.

Available topics: cel, events, overview, patterns, transcript-patterns (default: overview).

Use --output-dir to dump all docs to a directory; existing frontmatter in the
destination files is preserved unless --overwrite-frontmatter is set.

Flags:

- `--output-dir` — dump all docs to directory
- `--overwrite-frontmatter` — overwrite existing frontmatter when dumping (default: preserve)

### `hocage hooks run <hook_name>`

Run a hook (reads event JSON on stdin).

Flags:

- `--dry-run` — preview hook execution without running actions

### `hocage hooks check`

Validate config and CEL expressions.

### `hocage hooks test`

Run inline test cases.

### `hocage hooks list`

List all hooks defined in the config.

### `hocage hooks generate`

Generate Claude Code settings.json hooks section.

Generates the hooks section for Claude Code's settings.json from the config.

Example output:

    {
      "hooks": {
        "PreToolUse": [
          {
            "matcher": "Bash",
            "hooks": [
              {
                "type": "command",
                "command": "hocage hooks run block_rm_rf"
              }
            ]
          }
        ]
      }
    }

Flags:

- `--merge`, `-m` — merge with existing JSON file
- `--output`, `-o` — output file (reads for merge if exists, writes with -f)
- `--force`, `-f` — write to output file (requires -o)
<!-- gen:cli:end -->

## Design Notes

- **Claude Code focused:** The current scope targets Claude Code hooks. Codex support (shared events: SessionStart, UserPromptSubmit, Stop) may be added later.
- **`matcher` field:** Used for `hocage hooks generate` to produce the correct Claude Code settings. hocage itself uses the CEL `when` expression for all filtering logic.
- **No output protocol abstraction (yet):** hocage does not abstract the hook output protocol. Users write the output object directly in `respond`. A higher-level abstraction (e.g. `action: deny`) may be added later once the protocol stabilizes.
- **`updatedInput`:** Claude Code supports rewriting tool input via `updatedInput` in PreToolUse. This is supported through the `respond` action with `{cel: <expr>}` nodes in `updatedInput` fields — the node's typed result is embedded, so rewritten values need not be strings. See the "Rewrite tool input" example above.

## See also

- https://code.claude.com/docs/en/hooks
- https://github.com/google/cel-go
