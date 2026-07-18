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
      respond: <object>       # object serialized as JSON to stdout
      # or
      command: <string>       # external command to execute
      stdin: <string>         # optional: pipe input to command (supports {{expr}} interpolation)
      # or
      http:                    # send HTTP request with event JSON as body
        url: <string>         # required (supports {{expr}} interpolation)
        method: <string>      # optional: HTTP method (default: POST)
        headers:              # optional: HTTP headers (values support {{expr}} interpolation)
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
  "hook_type": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {
    "command": "rm -rf /"
  }
}
```

These fields are available as `event.hook_type`, `event.tool_name`, `event.tool_input.command`, etc.

### Transcript

Enable `transcript.load` to access the session transcript in CEL expressions. The `transcript` variable is a list of JSONL entries from the Claude Code session.

```yaml
transcript:
  load: true
  order: reverse  # optional: "chronological" (default) or "reverse"
```

When `order` is `reverse`, transcript entries are reversed so that `transcript[0]` is the most recent entry. This is useful because CEL does not support negative indexing.

### Action

| field | description |
|-------|-------------|
| `respond` | Serializes an object as JSON to stdout. |
| `command` | Executes an external command. Optionally accepts `stdin` to pipe input to the command. |
| `http` | Sends an HTTP request with the event JSON as the body. Supports `url`, `method` (default: POST), `headers`, and `timeout` (default: 10s). |

hocage evaluates the CEL `when` expression and, if true, executes the action. The user is responsible for producing the correct output for the hook protocol (JSON format, exit code, etc.).

See the [Claude Code hooks documentation](https://code.claude.com/docs/en/hooks) for the expected output format per event.

### CEL Expressions in `command` and `respond`

The `command` and `stdin` fields support CEL expression interpolation using `{{expr}}` syntax:

```yaml
action:
  command: "gofmt -w {{event.tool_input.file_path}}"
```

String values in `respond` also support `{{expr}}` interpolation:

```yaml
action:
  respond:
    decision: block
    reason: "{{event.tool_input.command}} is not allowed"
```

The `http` action supports `{{expr}}` interpolation in `url` and header values:

```yaml
action:
  http:
    url: "https://hooks.example.com/{{event.tool_name}}"
    headers:
      X-Hook-Event: "{{event.hook_type}}"
```

### Built-in CEL Functions

In addition to the [standard CEL functions](https://github.com/google/cel-spec/blob/master/doc/langdef.md#list-of-standard-definitions), hocage provides:

| category | function | description |
|----------|----------|-------------|
| File system | `file_exists(path)` | Returns true if file exists |
| | `dir_exists(path)` | Returns true if directory exists |
| | `read_file(path)` | File contents as UTF-8 text (`""` on any failure) |
| Git | `git_tracked(path)` | Returns true if file is tracked by git |
| Shell | `sh_commands(cmd)` | Program names invoked by a shell command, including inline `sh -c`/heredoc bodies |
| | `sh_words(cmd)` | All argument words of a shell command (quote-stripped) |
| | `sh_argv(cmd)` | Quote-stripped argv per simple command (structural, non-recursive) |
| | `sh_valid(cmd)` | Returns true if the command parses as valid shell |
| Glob | `glob_exists(pattern)` | Returns true if any file matches the glob pattern |
| Lists | `min(list)`, `max(list)` | Returns min/max element |
| Maps | `keys(map)`, `values(map)` | Returns keys/values as a list |
| | `to_entries(map)` | Converts map to `[{key, value}, ...]` |
| | `from_entries(list)` | Converts `[{key, value}, ...]` back to map |
| | `has_key(map, key)` | Returns true if map contains key |
| Strings | `trim_prefix(str, prefix)`, `trim_suffix(str, suffix)` | Trim prefix/suffix |
| | `path_base(path)`, `path_dir(path)`, `path_ext(path)` | Path manipulation |
| | `path_clean(path)`, `path_join(...)` | Path normalization/joining |
| | `quote(str)`, `squote(str)` | Shell quoting (double/single) |
| | `indent(str, prefix)` | Indent each line with prefix |
| Encoding | `to_json(value)`, `from_json(str)` | JSON serialization/parsing |
| Crypto | `sha256sum(data)` | SHA-256 hash |
| Semver | `semver_compare(v1, v2)` | Compare semantic versions |
| Environment | `env(name)` | Get environment variable |
| Utility | `default(value, fallback)` | Null coalescing |

Standard CEL extensions are also enabled: `ext.Strings()`, `ext.Lists()`, `ext.Sets()`, `ext.Math()`, `ext.Encoders()`, `ext.Regex()`, `ext.Bindings()`, `ext.TwoVarComprehensions()`.

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
        decision: block
        reason: "rm -rf is not allowed"
    tests:
      should_block:
        inputs:
          - tool_input: { command: "rm -rf /" }
          - tool_input: { command: "sudo rm -rf /tmp" }
          - tool_input: { command: "rm --recursive --force /" }
        result:
          stdout:
            decision: block
            reason: "rm -rf is not allowed"
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
        decision: block
        reason: "Writing outside the project directory is not allowed"
```

### Auto-format Go files after write

```yaml
hooks:
  format_go:
    event_name: PostToolUse
    matcher: Write
    when: event.tool_input.file_path.endsWith(".go")
    action:
      command: "gofmt -w {{event.tool_input.file_path}}"
```

### Send notification on session stop

```yaml
hooks:
  notify_stop:
    event_name: Stop
    when: "true"
    action:
      command: "ntfy publish --title 'Claude Code' 'Session completed'"
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
            command: "echo '{{event.tool_input.command}}' was blocked"
```

### Inject system message on user prompt

```yaml
hooks:
  remind_testing:
    event_name: UserPromptSubmit
    when: event.prompt.contains("deploy")
    action:
      respond:
        decision: allow
        systemMessage: "Remember to run tests before deploying"
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

### Block after dangerous command in transcript

Use transcript to detect prior dangerous actions in the session and block further tool use:

```yaml
hooks:
  block_after_dangerous:
    event_name: PreToolUse
    transcript:
      load: true
    when: |
      transcript.exists(t,
        has(t.tool) && t.tool == "Bash"
        && has(t.input) && has(t.input.command)
        && t.input.command.contains("rm -rf"))
    action:
      respond:
        decision: block
        reason: "Blocked: dangerous command detected in session transcript"
    tests:
      match_dangerous:
        transcript: |
          {"type":"user","message":"delete everything"}
          {"tool":"Bash","input":{"command":"rm -rf /"}}
        inputs:
          - tool_name: Bash
            input: { command: "echo hello" }
        result:
          stdout:
            decision: block
            reason: "Blocked: dangerous command detected in session transcript"
      no_match_safe:
        transcript: |
          {"type":"user","message":"hello"}
          {"tool":"Read","input":{"path":"foo.txt"}}
        inputs:
          - tool_name: Bash
            input: { command: "echo hello" }
```

### Rate-limit tool uses per session

Count tool invocations in the transcript and deny when the threshold is exceeded:

```yaml
hooks:
  rate_limit_tools:
    event_name: PreToolUse
    transcript:
      load: true
    when: |
      size(transcript.filter(t, has(t.tool))) > 100
    action:
      respond:
        decision: deny
        reason: "Too many tool uses in this session"
```

### Check most recent action with reverse order

Use `order: reverse` to access the most recent transcript entry at index 0. This is useful because CEL does not support negative indexing:

```yaml
hooks:
  block_after_write:
    event_name: PreToolUse
    transcript:
      load: true
      order: reverse
    when: |
      size(transcript) > 0
      && has(transcript[0].tool)
      && transcript[0].tool == "Write"
    action:
      respond:
        decision: block
        reason: "Please review the file you just wrote before running another tool"
    tests:
      last_was_write:
        transcript: |
          {"tool":"Read","input":{"path":"a.txt"}}
          {"tool":"Write","input":{"path":"b.txt"}}
        inputs:
          - tool_name: Bash
            input: { command: "echo test" }
        result:
          stdout:
            decision: block
            reason: "Please review the file you just wrote before running another tool"
      last_was_not_write:
        transcript: |
          {"tool":"Write","input":{"path":"b.txt"}}
          {"tool":"Read","input":{"path":"a.txt"}}
        inputs:
          - tool_name: Bash
            input: { command: "echo test" }
```

### Detect repeated failures

Block further tool use after consecutive failures to prevent infinite retry loops:

```yaml
hooks:
  stop_retry_loop:
    event_name: PreToolUse
    transcript:
      load: true
      order: reverse
    when: |
      size(transcript) >= 3
      && transcript[0:3].all(t, has(t.error) && t.error != "")
    action:
      respond:
        decision: deny
        reason: "3 consecutive errors detected. Please re-evaluate your approach"
```

### Detect edit thrashing

Block when the agent keeps editing the same file repeatedly, which often indicates it is stuck in a loop:

```yaml
hooks:
  detect_edit_thrashing:
    event_name: PreToolUse
    transcript:
      load: true
    when: |
      event.tool_name == "Edit"
      && has(event.tool_input.file_path)
      && size(transcript.filter(t,
        has(t.tool) && t.tool == "Edit"
        && has(t.input) && has(t.input.file_path)
        && t.input.file_path == event.tool_input.file_path
      )) >= 4
    action:
      respond:
        decision: block
        reason: "This file has been edited 4+ times. Step back and reconsider your approach"
```

### Detect scope creep

Warn when the agent has modified too many distinct files, which may indicate unintended scope expansion:

```yaml
hooks:
  detect_scope_creep:
    event_name: PreToolUse
    transcript:
      load: true
    when: |
      (event.tool_name == "Edit" || event.tool_name == "Write")
      && size(transcript.filter(t,
        has(t.tool) && (t.tool == "Edit" || t.tool == "Write")
        && has(t.input) && has(t.input.file_path)
      ).map(t, t.input.file_path).distinct()) >= 8
    action:
      respond:
        decision: block
        reason: "8+ distinct files modified in this session. Please confirm scope with the user"
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
      && !transcript.exists(t,
        has(t.tool) && t.tool == "Read"
        && has(t.input) && has(t.input.file_path)
        && t.input.file_path == event.tool_input.file_path)
    action:
      respond:
        decision: block
        reason: "You must Read a file before editing it"
```

### Auto-allow deploy after tests pass

If the transcript shows a successful test run, auto-allow deployment commands. The agent earns deployment rights by proving tests pass first:

```yaml
hooks:
  allow_deploy_after_tests:
    event_name: PreToolUse
    matcher: Bash
    transcript:
      load: true
    when: |
      event.tool_input.command.contains("deploy")
      && transcript.exists(t,
        has(t.tool) && t.tool == "Bash"
        && has(t.input) && has(t.input.command)
        && t.input.command.contains("go test")
        && has(t.output) && has(t.output.exit_code)
        && t.output.exit_code == 0)
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: allow
          permissionDecisionReason: "Tests passed in this session"
```

### Auto-allow operations on user-mentioned files

If the user explicitly mentioned a file path in their prompt, auto-allow the agent to operate on it. This trusts user intent — if they asked about a file, the agent should be able to touch it:

```yaml
hooks:
  allow_user_mentioned_files:
    event_name: PreToolUse
    transcript:
      load: true
    when: |
      (event.tool_name == "Edit" || event.tool_name == "Write")
      && has(event.tool_input.file_path)
      && transcript.exists(t,
        has(t.type) && t.type == "user"
        && has(t.message)
        && t.message.contains(event.tool_input.file_path))
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: allow
          permissionDecisionReason: "File was mentioned by the user"
```

### Auto-allow files in the same directory

Once the agent has been allowed to modify a file in a directory, auto-allow subsequent modifications to other files in the same directory. This reduces repetitive approval prompts during batch operations:

```yaml
hooks:
  allow_same_directory:
    event_name: PreToolUse
    transcript:
      load: true
    when: |
      (event.tool_name == "Edit" || event.tool_name == "Write")
      && has(event.tool_input.file_path)
      && transcript.exists(t,
        has(t.tool) && (t.tool == "Edit" || t.tool == "Write")
        && has(t.input) && has(t.input.file_path)
        && path_dir(t.input.file_path) == path_dir(event.tool_input.file_path))
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: allow
          permissionDecisionReason: "Another file in the same directory was already modified"
```

### Auto-allow repeated command patterns

If a Bash command matching the same prefix was already executed successfully, auto-allow subsequent similar commands. This avoids repeatedly approving the same class of operations (e.g., multiple `go test ./...` runs):

```yaml
hooks:
  allow_repeated_command:
    event_name: PreToolUse
    matcher: Bash
    transcript:
      load: true
    when: |
      has(event.tool_input.command)
      && transcript.exists(t,
        has(t.tool) && t.tool == "Bash"
        && has(t.input) && has(t.input.command)
        && has(t.output) && has(t.output.exit_code)
        && t.output.exit_code == 0
        && event.tool_input.command.startsWith(
          t.input.command.split(" ")[0]))
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: allow
          permissionDecisionReason: "A similar command succeeded earlier in this session"
```

### Enforce git-tracked files only

Block writes to files not tracked by git:

```yaml
hooks:
  git_tracked_only:
    event_name: PreToolUse
    matcher: Write
    when: |
      file_exists(event.tool_input.file_path)
      && !git_tracked(event.tool_input.file_path)
    action:
      respond:
        decision: block
        reason: "Cannot modify untracked files. Please git add first"
```

### Restrict file extensions

Allow writes only to specific file types:

```yaml
hooks:
  restrict_extensions:
    event_name: PreToolUse
    matcher: Write
    when: |
      !(path_ext(event.tool_input.file_path) in [".go", ".yaml", ".md", ".json"])
    action:
      respond:
        decision: block
        reason: "Only .go, .yaml, .md, and .json files are allowed"
```

### Log all tool uses to a file

Pipe event data to a logging command for audit purposes:

```yaml
hooks:
  audit_log:
    event_name: PostToolUse
    when: "true"
    action:
      command: "tee -a /tmp/claude-audit.jsonl"
      stdin: "{{to_json(event)}}"
```

### Block based on environment

Use environment variables to conditionally enforce policies:

```yaml
hooks:
  block_in_production:
    event_name: PreToolUse
    matcher: Bash
    when: |
      env("HOCAGE_ENV") == "production"
      && event.tool_input.command.contains("DROP TABLE")
    action:
      respond:
        decision: block
        reason: "DROP TABLE is blocked in production"
```

### Require tests before deployment commands

Inspect transcript to ensure tests were run before deploy-related commands:

```yaml
hooks:
  require_tests_before_deploy:
    event_name: PreToolUse
    matcher: Bash
    transcript:
      load: true
    when: |
      event.tool_input.command.contains("deploy")
      && !transcript.exists(t,
        has(t.tool) && t.tool == "Bash"
        && has(t.input) && has(t.input.command)
        && t.input.command.contains("go test"))
    action:
      respond:
        decision: block
        reason: "Please run tests before deploying"
```

## CLI

### Check policies

Validates CEL expression syntax/types and runs heuristic checks:

```
hocage hooks check
```

### Run tests

Runs inline test cases defined in the config:

```
hocage hooks test
```

### List hooks

Lists all hooks defined in the config:

```
hocage hooks list
```

### Run a hook

Reads stdin JSON from Claude Code and evaluates the policy:

```
hocage hooks run <hook_name>
```

### Generate Claude Code settings

Generates the `hooks` section for Claude Code's `settings.json`:

```
hocage hooks generate
```

Example output:

```json
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
```

### Show documentation

Shows embedded skill documentation (`.claude/skills/hocage/`) from the CLI:

```
hocage docs           # overview (default)
hocage docs cel       # CEL functions reference
hocage docs events    # event types and output schemas
hocage docs patterns  # common hook patterns
```

Dump all docs to a directory (preserves existing frontmatter by default):

```
hocage docs --output-dir ./docs
hocage docs --output-dir ./docs --overwrite-frontmatter
```

## Design Notes

- **Claude Code focused:** The current scope targets Claude Code hooks. Codex support (shared events: SessionStart, UserPromptSubmit, Stop) may be added later.
- **`matcher` field:** Used for `hocage hooks generate` to produce the correct Claude Code settings. hocage itself uses the CEL `when` expression for all filtering logic.
- **No output protocol abstraction (yet):** hocage does not abstract the hook output protocol. Users write the output object directly in `respond`. A higher-level abstraction (e.g. `action: deny`) may be added later once the protocol stabilizes.
- **`updatedInput`:** Claude Code supports rewriting tool input via `updatedInput` in PreToolUse. This is supported through the `respond` action with `{{expr}}` interpolation in `updatedInput` fields. See the "Rewrite tool input" example above.

## See also

- https://code.claude.com/docs/en/hooks
- https://github.com/google/cel-go
