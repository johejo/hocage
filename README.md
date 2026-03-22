# hocage

Coding Agent Hooks Policy Framework Using CEL

Define declarative policies for [Claude Code hooks](https://code.claude.com/docs/en/hooks.md) using [CEL (Common Expression Language)](https://cel.dev/).

## Config

```yaml
hooks:
  <hook_name>:
    event_name: <event_name>   # e.g. PreToolUse, Stop, UserPromptSubmit
    matcher: <matcher>         # optional: tool name for PreToolUse (e.g. Bash, Write)
    when: <cel_expression>     # CEL expression that evaluates to bool
    action:
      respond: <object>       # object serialized as JSON to stdout
      # or
      command: <string>       # external command to execute
    tests:
      <test_case_name>:
        inputs:               # list of stdin JSON inputs
          - <event_object>
        result:
          stdout: <object>    # expected stdout (compared as JSON)
          # stdout is omitted when `when` evaluates to false (action not executed)
```

`respond` and `command` are mutually exclusive. The presence of either determines the action type.

### CEL Variable Bindings

The stdin JSON from Claude Code is bound to the `event` variable. For example, a PreToolUse event for the Bash tool receives:

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

The `event` namespace keeps raw input separate from hocage built-in variables that may be added in the future (e.g. `cwd`, `env`, `project_root`).

### Action

| field | description |
|-------|-------------|
| `respond` | Serializes an object as JSON to stdout. |
| `command` | Executes an external command. |

hocage evaluates the CEL `when` expression and, if true, executes the action. The user is responsible for producing the correct output for the hook protocol (JSON format, exit code, etc.).

See the [Claude Code hooks documentation](https://code.claude.com/docs/en/hooks.md) for the expected output format per event.

### CEL Expressions in `command` and `respond`

The `command` field supports CEL expression interpolation using `{{expr}}` syntax:

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

### Examples

Block `rm -rf` (PreToolUse):

```yaml
hooks:
  block_rm_rf:
    event_name: PreToolUse
    matcher: Bash
    when: event.tool_input.command.contains("rm -rf")
    action:
      respond:
        decision: block
        reason: "rm -rf is not allowed"
    tests:
      should_block:
        inputs:
          - tool_input: { command: "rm -rf /" }
          - tool_input: { command: "sudo rm -rf /tmp" }
        result:
          stdout:
            decision: block
            reason: "rm -rf is not allowed"

      should_allow:
        inputs:
          - tool_input: { command: "ls -la" }
          - tool_input: { command: "rm file.txt" }
        result:

```

Block writes outside the project:

```yaml
hooks:
  block_write_outside_project:
    event_name: PreToolUse
    matcher: Write
    when: '!event.tool_input.file_path.startsWith("/home/user/project")'
    action:
      respond:
        decision: block
        reason: "Writing outside the project directory is not allowed"
```

Auto-format Go files after write:

```yaml
hooks:
  format_go:
    event_name: PostToolUse
    matcher: Write
    when: event.tool_input.file_path.endsWith(".go")
    action:
      command: "gofmt -w {{event.tool_input.file_path}}"
```

Send notification on session stop:

```yaml
hooks:
  notify_stop:
    event_name: Stop
    when: "true"
    action:
      command: "ntfy publish --title 'Claude Code' 'Session completed'"
```

Rewrite tool input with `updatedInput` (PreToolUse):

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

Inject system message on user prompt:

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

## Implementation

- Language: Go
- Library: [cel-go](https://github.com/google/cel-go)

## Design Notes

- **Claude Code focused:** The current scope targets Claude Code hooks. Codex support (shared events: SessionStart, UserPromptSubmit, Stop) may be added later.
- **`matcher` field:** Used for `hocage hooks generate` to produce the correct Claude Code settings. hocage itself uses the CEL `when` expression for all filtering logic.
- **No output protocol abstraction (yet):** hocage does not abstract the hook output protocol. Users write the output object directly in `respond`. A higher-level abstraction (e.g. `action: deny`) may be added later once the protocol stabilizes.
- **`updatedInput`:** Claude Code supports rewriting tool input via `updatedInput` in PreToolUse. This is supported through the `respond` action with `{{expr}}` interpolation in `updatedInput` fields. See the example below.

## See also

- https://code.claude.com/docs/en/hooks.md
- https://github.com/google/cel-go
