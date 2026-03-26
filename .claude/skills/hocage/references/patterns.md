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

---

## Transcript Patterns

The following patterns use `transcript` for stateful policy evaluation across the session.

## 15. Block After Dangerous Command in Transcript

Detect prior dangerous actions in the session and block further tool use:

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

## 16. Rate-Limit Tool Uses

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

## 17. Check Most Recent Action (Reverse Order)

Use `order: reverse` so `transcript[0]` is the most recent entry (CEL has no negative indexing):

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

## 18. Detect Repeated Failures

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

## 19. Detect Edit Thrashing

Block when the agent keeps editing the same file repeatedly:

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

## 20. Detect Scope Creep

Warn when the agent has modified too many distinct files:

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

## 21. Require Read Before Edit

Prevent editing a file that was never read in the session:

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

## 22. Auto-Allow Deploy After Tests Pass

If the transcript shows a successful test run, auto-allow deployment:

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

## 23. Auto-Allow User-Mentioned Files

If the user mentioned a file path in their prompt, auto-allow operations on it:

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

## 24. Auto-Allow Same Directory

Once a file in a directory was modified, auto-allow subsequent modifications in the same directory:

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

## 25. Auto-Allow Repeated Commands

If a similar command succeeded earlier, auto-allow it:

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

## 26. Test with Inline Transcript

Tests can provide inline JSONL transcript data or reference a file:

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
        reason: "Dangerous command detected"
    tests:
      with_inline_transcript:
        transcript: |
          {"type":"user","message":"clean up"}
          {"tool":"Bash","input":{"command":"rm -rf /tmp/junk"}}
        inputs:
          - tool_name: Bash
            input: { command: "echo hello" }
        result:
          stdout:
            decision: block
            reason: "Dangerous command detected"
      with_transcript_file:
        transcript_file: testdata/dangerous_session.jsonl
        inputs:
          - tool_name: Bash
            input: { command: "echo hello" }
        result:
          stdout:
            decision: block
            reason: "Dangerous command detected"
```
