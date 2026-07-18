# Transcript Hook Patterns

Stateful policies using the session `transcript` (requires `transcript.load: true`). Entries are `dyn` maps with varying shape — always guard field access with `has()`. For basic patterns, see `patterns.md`.

## 1. Block After Dangerous Command in Transcript

Also shows testing with transcripts: `transcript:` takes inline JSONL, `transcript_file:` a path to a JSONL file. `hocage hooks check` validates both.

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
      match_from_file:
        transcript_file: testdata/dangerous_session.jsonl
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

## 2. Rate-Limit Tool Uses

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

## 3. Check Most Recent Action (Reverse Order)

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
```

## 4. Detect Repeated Failures

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

## 5. Detect Edit Thrashing

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

## 6. Detect Scope Creep

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

## 7. Require Read Before Edit

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

## 8. Auto-Allow via Transcript

Grant permission automatically based on session history, using PreToolUse `hookSpecificOutput`. Full example — allow deploy after tests passed in this session:

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

Variants — same action shape, different `when` conditions:

**File mentioned by the user in a prompt:**

```yaml
when: |
  (event.tool_name == "Edit" || event.tool_name == "Write")
  && has(event.tool_input.file_path)
  && transcript.exists(t,
    has(t.type) && t.type == "user"
    && has(t.message)
    && t.message.contains(event.tool_input.file_path))
```

**Another file in the same directory was already modified:**

```yaml
when: |
  (event.tool_name == "Edit" || event.tool_name == "Write")
  && has(event.tool_input.file_path)
  && transcript.exists(t,
    has(t.tool) && (t.tool == "Edit" || t.tool == "Write")
    && has(t.input) && has(t.input.file_path)
    && path_dir(t.input.file_path) == path_dir(event.tool_input.file_path))
```

**A command with the same program name succeeded earlier:**

```yaml
when: |
  has(event.tool_input.command)
  && transcript.exists(t,
    has(t.tool) && t.tool == "Bash"
    && has(t.input) && has(t.input.command)
    && has(t.output) && has(t.output.exit_code)
    && t.output.exit_code == 0
    && event.tool_input.command.startsWith(
      t.input.command.split(" ")[0]))
```
