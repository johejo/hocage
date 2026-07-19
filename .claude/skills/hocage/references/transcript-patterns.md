# Transcript Hook Patterns

Stateful policies using the session `transcript` (requires `transcript.load: true`).

## Real Transcript Shape

The file at `transcript_path` is the Claude Code session JSONL. Each line has a
top-level `type` — message lines (`user`, `assistant`, `system`) carry a
`message: {role, content}` plus metadata (`uuid`, `timestamp`, `sessionId`,
`cwd`, ...), and non-message lines (`mode`, `file-history-snapshot`,
`attachment`, `queue-operation`, ...) are interleaved throughout. Tool calls are
NOT top-level entries: they live inside assistant entries as
`message.content[]` blocks of type `tool_use` (`{id, name, input}`), and their
results arrive later as user entries with `tool_result` blocks plus a top-level
`toolUseResult` (for Bash: `stdout`, `stderr`, `interrupted` — there is no
`exit_code`).

Don't navigate that by hand. Use the helpers:

- `tool_calls(transcript)` — flattened tool calls `{id, name, input, result}`,
  where `result` (absent while the call is still running) has `is_error`,
  `content`, and the `toolUseResult` fields.
- `user_messages(transcript)` — list of real user message texts (meta and
  tool_result-only entries skipped).

Both respect transcript order, so with `transcript.order: reverse`,
`tool_calls(transcript)[0]` is the most recent call. For raw entry access
beyond these helpers, always guard with `has()` — entry shapes vary by line
type.

## 1. Block After Dangerous Command in Transcript

Also shows testing with transcripts: `transcript:` takes inline JSONL,
`transcript_file:` a path to a JSONL file. `hocage hooks check` validates both.

```yaml
hooks:
  block_after_dangerous:
    event_name: PreToolUse
    transcript:
      load: true
    when: |
      tool_calls(transcript).exists(c,
        c.name == "Bash"
        && has(c.input.command)
        && c.input.command.contains("rm -rf"))
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "Blocked: dangerous command detected in session transcript"
    tests:
      match_dangerous:
        transcript: |
          {"type":"user","message":{"role":"user","content":"delete everything"},"uuid":"u1"}
          {"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"rm -rf /"}}]},"uuid":"a1"}
        inputs:
          - hook_event_name: PreToolUse
            tool_name: Bash
            tool_input: { command: "echo hello" }
        result:
          stdout:
            hookSpecificOutput:
              hookEventName: PreToolUse
              permissionDecision: deny
              permissionDecisionReason: "Blocked: dangerous command detected in session transcript"
      match_from_file:
        transcript_file: testdata/dangerous_session.jsonl
        inputs:
          - hook_event_name: PreToolUse
            tool_name: Bash
            tool_input: { command: "echo hello" }
        result:
          stdout:
            hookSpecificOutput:
              hookEventName: PreToolUse
              permissionDecision: deny
              permissionDecisionReason: "Blocked: dangerous command detected in session transcript"
      no_match_safe:
        transcript: |
          {"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1"}
          {"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"foo.txt"}}]},"uuid":"a1"}
        inputs:
          - hook_event_name: PreToolUse
            tool_name: Bash
            tool_input: { command: "echo hello" }
```

## 2. Rate-Limit Tool Uses

```yaml
hooks:
  rate_limit_tools:
    event_name: PreToolUse
    transcript:
      load: true
    when: |
      size(tool_calls(transcript)) > 100
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "Too many tool uses in this session"
```

## 3. Check Most Recent Action (Reverse Order)

Use `order: reverse` so `tool_calls(transcript)[0]` is the most recent call
(CEL has no negative indexing):

```yaml
hooks:
  block_after_write:
    event_name: PreToolUse
    transcript:
      load: true
      order: reverse
    when: |
      size(tool_calls(transcript)) > 0
      && tool_calls(transcript)[0].name == "Write"
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "Please review the file you just wrote before running another tool"
```

## 4. Detect Repeated Failures

Block further tool use after consecutive failures to prevent infinite retry
loops. A call's `result.is_error` is true when the tool errored; `has(c.result)`
guards calls that have no result yet. `.slice()` comes from the Lists extension.

```yaml
hooks:
  stop_retry_loop:
    event_name: PreToolUse
    transcript:
      load: true
      order: reverse
    when: |
      cel.bind(calls, tool_calls(transcript),
        size(calls) >= 3
        && calls.slice(0, 3).all(c, has(c.result) && c.result.is_error))
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "3 consecutive errors detected. Please re-evaluate your approach"
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
      && size(tool_calls(transcript).filter(c,
        c.name == "Edit"
        && has(c.input.file_path)
        && c.input.file_path == event.tool_input.file_path
      )) >= 4
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "This file has been edited 4+ times. Step back and reconsider your approach"
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
      && size(tool_calls(transcript).filter(c,
        (c.name == "Edit" || c.name == "Write")
        && has(c.input.file_path)
      ).map(c, c.input.file_path).distinct()) >= 8
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "8+ distinct files modified in this session. Please confirm scope with the user"
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

## 8. Auto-Allow via Transcript

Grant permission automatically based on session history, using PreToolUse
`permissionDecision: allow`. Full example — allow deploy after tests passed in
this session (`!c.result.is_error` is the success signal; real transcripts have
no exit code):

```yaml
hooks:
  allow_deploy_after_tests:
    event_name: PreToolUse
    matcher: Bash
    transcript:
      load: true
    when: |
      event.tool_input.command.contains("deploy")
      && tool_calls(transcript).exists(c,
        c.name == "Bash"
        && has(c.input.command)
        && c.input.command.contains("go test")
        && has(c.result)
        && !c.result.is_error)
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
  && user_messages(transcript).exists(m,
    m.contains(event.tool_input.file_path))
```

**Another file in the same directory was already modified:**

```yaml
when: |
  (event.tool_name == "Edit" || event.tool_name == "Write")
  && has(event.tool_input.file_path)
  && tool_calls(transcript).exists(c,
    (c.name == "Edit" || c.name == "Write")
    && has(c.input.file_path)
    && path_dir(c.input.file_path) == path_dir(event.tool_input.file_path))
```

**A command with the same program name succeeded earlier:**

```yaml
when: |
  has(event.tool_input.command)
  && tool_calls(transcript).exists(c,
    c.name == "Bash"
    && has(c.input.command)
    && has(c.result) && !c.result.is_error
    && event.tool_input.command.startsWith(
      c.input.command.split(" ")[0]))
```

Inverse (require instead of auto-allow) — negate the
`tool_calls(transcript).exists(...)` condition and respond with
`permissionDecision: deny` instead of `allow`, e.g. block deploy commands
unless a `go test` run appears in the transcript.
