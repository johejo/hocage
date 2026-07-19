# Common Hook Patterns

Copy-paste recipes for common hocage use cases. For transcript-based (stateful) patterns, see `transcript-patterns.md`.

## 1. Block Dangerous Commands (PreToolUse)

Also shows the test pattern: `result.stdout` asserts a match, an empty `result:` asserts no match (no output).

```yaml
hooks:
  block_rm_rf:
    event_name: PreToolUse
    matcher: Bash
    when: event.tool_input.command.contains("rm -rf")
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
        result:
```

## 2. Inspect Shell Scripts Before Execution (PreToolUse)

Claude Code sometimes writes a throwaway script and then runs `bash /tmp/x.sh` —
the Bash hook only sees the interpreter invocation. Find shell-interpreter
invocations with `sh_argv`, read the script from disk with `read_file` (guarded
by `read_file_ok` so an uninspectable script denies), and scan its body with the
same parser. Inline bodies (`bash -c '...'`, `bash <<EOF`) need no file read —
`sh_commands` recurses into them itself, which also covers "write and run in one
command" where the script file does not exist yet when the hook fires.

```yaml
hooks:
  inspect_shell_scripts:
    event_name: PreToolUse
    matcher: Bash
    # Deny rm in the command itself and inside script files it executes.
    when: >-
      "rm" in sh_commands(event.tool_input.command)
      || sh_argv(event.tool_input.command).exists(argv,
           argv.size() >= 2
           && path_base(argv[0]) in ["sh", "bash", "zsh", "dash", "ksh", "mksh"]
           && argv.slice(1, argv.size()).exists(a,
                a.startsWith("/")
                && (!read_file_ok(a) || "rm" in sh_commands(read_file(a)))))
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "rm detected in the command or in a script it executes"
    tests:
      should_block_inline:
        inputs:
          - tool_input: { command: "rm -rf /tmp/x" }
          - tool_input: { command: "bash -c 'rm -rf /tmp/x'" }
          - tool_input: { command: "bash <<EOF\nrm -rf /tmp/x\nEOF" }
        result:
          stdout:
            hookSpecificOutput:
              hookEventName: PreToolUse
              permissionDecision: deny
              permissionDecisionReason: "rm detected in the command or in a script it executes"
      should_block_unreadable_script:
        inputs:
          - tool_input: { command: "bash /nonexistent/script.sh" }
        result:
          stdout:
            hookSpecificOutput:
              hookEventName: PreToolUse
              permissionDecision: deny
              permissionDecisionReason: "rm detected in the command or in a script it executes"
      should_allow:
        inputs:
          - tool_input: { command: "echo 'rm -rf /'" }
        result:
```

Notes:

- `read_file` alone is fail-open (missing/unreadable/oversize/non-UTF-8 → `""`);
  composing `!read_file_ok(a) || ...` fails closed when the script cannot be
  fully inspected.
- Relative script paths resolve against the hook process cwd, which can differ
  from the Bash tool's shell cwd — the recipe only trusts absolute paths.
- Runtime-generated bodies (`bash <(echo ...)`, `bash "$SCRIPT"`, `curl | sh`)
  have no static text to scan, but a fully non-literal word resolves to `""` in
  `sh_argv`, so they can still be denied fail-closed:

  ```yaml
  when: >-
    sh_argv(event.tool_input.command).exists(argv,
      path_base(argv[0]) in ["sh", "bash", "zsh", "dash", "ksh", "mksh"] && "" in argv)
  ```

- The `tests:` above deliberately avoid disk state; script-file cases are better
  verified with `hocage hooks run <name>` and a real event.

## 3. Block Writes Outside Project

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

## 4. Auto-Format After Write (PostToolUse)

The command string is literal; event data reaches it only through `env:`, so
shell metacharacters in file paths cannot execute.

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

Or skip the shell entirely with the argv-list form:

```yaml
    action:
      command: ["gofmt", "-w", { cel: event.tool_input.file_path }]
```

## 5. Rewrite Tool Input (updatedInput)

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
            command:
              cel: '"echo " + squote(event.tool_input.command) + " was blocked"'
```

`{cel: ...}` nodes embed the expression's typed result, so an `updatedInput`
field can also be a whole object: `updatedInput: {cel: '{"command": "echo safe"}'}`.

## 6. Inject System Message (UserPromptSubmit)

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

## 7. Send Webhook (HTTP Action)

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
          X-Hook-Event: { cel: event.hook_event_name }
        timeout: "5s"
```

## 8. Pipe Event to Command via Stdin

```yaml
hooks:
  pipe_event:
    event_name: PreToolUse
    matcher: Bash
    when: "true"
    action:
      command: "cat"
      stdin: { cel: to_json(event) }
```

Audit-log variant — append every tool use to a JSONL file:

```yaml
hooks:
  audit_log:
    event_name: PostToolUse
    when: "true"
    action:
      command: "tee -a /tmp/claude-audit.jsonl"
      stdin: { cel: to_json(event) }
```

## 9. Conditions on Filesystem / Git State

`ctx.project_root` is `""` outside a git repository — guard with `ctx.project_root != ""` if needed.

```yaml
hooks:
  check_config_exists:
    event_name: PreToolUse
    matcher: Bash
    when: 'file_exists(path_join([ctx.project_root, ".config.yaml"]))'
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: allow
          permissionDecisionReason: "Config exists"

  only_tracked_files:
    event_name: PreToolUse
    matcher: Write
    when: 'git_tracked(event.tool_input.file_path) || !file_exists(event.tool_input.file_path)'
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: allow
          permissionDecisionReason: "File is tracked or new"
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

## 11. Use cel.bind() for Complex Expressions

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
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "sudo rm is not allowed"
```

## 12. Environment Variable Checks

```yaml
hooks:
  prod_guard:
    event_name: PreToolUse
    matcher: Bash
    when: 'env("NODE_ENV") == "production" && event.tool_input.command.contains("drop")'
    action:
      respond:
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "Destructive commands blocked in production"
```

## 13. Session Lifecycle Notifications

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

  agent_stop_notify:
    event_name: Stop
    when: "true"
    action:
      command: "ntfy publish --title 'Claude Code' 'Session completed'"
```

## 14. Restrict File Extensions

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
        hookSpecificOutput:
          hookEventName: PreToolUse
          permissionDecision: deny
          permissionDecisionReason: "Only .go, .yaml, .md, and .json files are allowed"
```
