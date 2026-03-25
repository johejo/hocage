# Common Hook Patterns

Copy-paste recipes for common hocage use cases.

## 1. Block Dangerous Commands (PreToolUse)

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

## 2. Block Writes Outside Project

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

## 3. Auto-Format After Write (PostToolUse)

```yaml
hooks:
  format_go:
    event_name: PostToolUse
    matcher: Write
    when: event.tool_input.file_path.endsWith(".go")
    action:
      command: "gofmt -w {{event.tool_input.file_path}}"
```

## 4. Rewrite Tool Input (updatedInput)

Use `hookSpecificOutput` to rewrite tool input while allowing execution:

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

## 5. Inject System Message (UserPromptSubmit)

```yaml
hooks:
  remind_testing:
    event_name: UserPromptSubmit
    when: event.prompt.contains("deploy")
    action:
      respond:
        additionalContext: "Remember to run tests before deploying"
```

## 6. Send Webhook (HTTP Action)

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
          X-Hook-Event: "{{event.hook_type}}"
        timeout: "5s"
```

## 7. Pipe Event to Command via Stdin

```yaml
hooks:
  pipe_event:
    event_name: PreToolUse
    matcher: Bash
    when: "true"
    action:
      command: "cat"
      stdin: "{{to_json(event)}}"
```

## 8. Use ctx.project_root for Portable Paths

Note: `ctx.project_root` is empty string if not inside a git repository. Guard with `ctx.project_root != ""` if needed.

```yaml
hooks:
  check_config_exists:
    event_name: PreToolUse
    matcher: Bash
    when: 'file_exists(path_join([ctx.project_root, ".config.yaml"]))'
    action:
      respond:
        decision: allow
        reason: "Config exists"
```

## 9. Conditional with file_exists / git_tracked

```yaml
hooks:
  only_tracked_files:
    event_name: PreToolUse
    matcher: Write
    when: 'git_tracked(event.tool_input.file_path) || !file_exists(event.tool_input.file_path)'
    action:
      respond:
        decision: allow
        reason: "File is tracked or new"
```

## 10. Priority Ordering

Lower priority values run first in generated settings. Default is 0.

```yaml
hooks:
  high_priority_hook:
    event_name: PreToolUse
    matcher: Bash
    priority: 1
    when: "true"
    action:
      command: "echo high"
  low_priority_hook:
    event_name: PreToolUse
    matcher: Bash
    priority: 10
    when: "true"
    action:
      command: "echo low"
```

## 11. Test Patterns (Match vs No-Match)

When `when` evaluates to false, the action is not executed and nothing is output. Set `result:` to empty (null) to assert no-match:

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
      match_case:
        inputs:
          - tool_input: { command: "rm -rf /" }
        result:
          stdout:
            decision: block
            reason: "rm -rf is not allowed"
      no_match_case:
        inputs:
          - tool_input: { command: "ls -la" }
        result:
```

## 12. Use cel.bind() for Complex Expressions

```yaml
hooks:
  complex_check:
    event_name: PreToolUse
    matcher: Bash
    when: >-
      cel.bind(cmd, event.tool_input.command,
        cmd.contains("sudo") && cmd.contains("rm"))
    action:
      respond:
        decision: block
        reason: "sudo rm is not allowed"
```

## 13. Environment Variable Checks

```yaml
hooks:
  prod_guard:
    event_name: PreToolUse
    matcher: Bash
    when: 'env("NODE_ENV") == "production" && event.tool_input.command.contains("drop")'
    action:
      respond:
        decision: block
        reason: "Destructive commands blocked in production"
```

## 14. Session Lifecycle Notifications

```yaml
hooks:
  session_start_notify:
    event_name: SessionStart
    when: "true"
    action:
      command: "notify-send 'Claude Code session started'"

  session_end_notify:
    event_name: SessionEnd
    when: "true"
    action:
      command: "notify-send 'Claude Code session ended'"
```
