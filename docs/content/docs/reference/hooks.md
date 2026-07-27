---
title: Hooks Reference
weight: 2
---

# Hooks Reference

Hooks are event-driven handlers that run at specific points in the agent's lifecycle.
Keel defines hooks in JSON companion files and merges them across layers.

## Defining Hooks

Reference a hooks JSON file in `keel.yml`:

```yaml
hooks:
  - source: hooks.json
```

The JSON file maps event names to arrays of handlers:

```json
{
  "PreToolUse": [
    {"type": "command", "command": "echo pre-tool-check"}
  ],
  "PostToolUse": [
    {"type": "command", "command": "echo post-tool-cleanup"}
  ],
  "SessionStart": [
    {"type": "command", "command": "echo session-started"}
  ],
  "Stop": [
    {"type": "prompt", "prompt": "Summarize what was accomplished"}
  ]
}
```

Hook keys in Keel DSL use **PascalCase**.
Keel automatically converts to camelCase for Cursor output.

## Handler Types

| Type | Claude | Cursor | Description |
|------|--------|--------|-------------|
| `command` | ✅ | ✅ | Run a shell command |
| `prompt` | ✅ | ✅ | Inject a prompt into the conversation |
| `http` | ✅ | ❌ | Send an HTTP request |
| `mcp_tool` | ✅ | ❌ | Call an MCP tool |
| `agent` | ✅ | ❌ | Spawn a sub-agent |

When compiling for Cursor, non-portable handler types (`http`, `mcp_tool`, `agent`) are dropped with a warning.

### Handler Fields

| Field | Claude | Cursor | Description |
|-------|--------|--------|-------------|
| `type` | ✅ | ✅ | Handler type (see above) |
| `command` | ✅ | ✅ | Shell command to execute (for `command` type) |
| `prompt` | ✅ | ✅ | Prompt text (for `prompt` type) |
| `if` | ✅ | ❌ | Conditional expression |
| `timeout` | ✅ | ✅ | Timeout in milliseconds |
| `statusMessage` | ✅ | ❌ | Status message shown during execution |
| `once` | ✅ | ❌ | Run only once per session |
| `matcher` | ❌ | ✅ | Pattern matching for tool/file events |
| `loop_limit` | ❌ | ✅ | Maximum execution count |
| `failClosed` | ❌ | ✅ | Block operation if hook fails |

## Force-Locking Hooks

Individual hook entries can be force-locked to prevent downstream layers from overriding them:

```json
{
  "PreToolUse": [
    {
      "type": "command",
      "command": "echo security-check",
      "force": true
    }
  ]
}
```

The `force` field is stripped from the final output.

## Event Reference

### Portable Events (Claude + Cursor)

These events work in both Claude and Cursor.
Keel handles the PascalCase ↔ camelCase conversion automatically.

| Keel Event (PascalCase) | Claude | Cursor | Notes |
|-------------------------|--------|--------|-------|
| `SessionStart` | `SessionStart` | `sessionStart` | Session initialization |
| `PreToolUse` | `PreToolUse` | `preToolUse` | Before a tool is invoked |
| `PostToolUse` | `PostToolUse` | `postToolUse` | After a tool completes |
| `PostToolUseFailure` | `PostToolUseFailure` | `postToolUseFailure` | After a tool fails |
| `SubagentStart` | `SubagentStart` | `subagentStart` | When a sub-agent spawns |
| `SubagentStop` | `SubagentStop` | `subagentStop` | When a sub-agent finishes |
| `Stop` | `Stop` | `stop` | Session ending |
| `PreCompact` | `PreCompact` | `preCompact` | Before context compaction |
| `SessionEnd` | `SessionEnd` | `sessionEnd` | Session cleanup |

**Special case**: `UserPromptSubmit` in the Keel IR maps to `UserPromptSubmit` in Claude and `beforeSubmitPrompt` in Cursor.
This is an explicit rename, not a case transform.

### Claude-Only Events

These events are only available when compiling for Claude.
They are dropped with a warning when compiling for Cursor.

| Event | Description |
|-------|-------------|
| `Setup` | Initial setup |
| `UserPromptExpansion` | Prompt expansion processing |
| `PermissionRequest` | Permission requested |
| `PermissionDenied` | Permission denied |
| `PostToolBatch` | After a batch of tool calls |
| `Notification` | Notification event |
| `TaskCreated` | Task list item created |
| `TaskCompleted` | Task list item completed |
| `StopFailure` | Stop failed |
| `TeammateIdle` | Teammate agent idle |
| `InstructionsLoaded` | Instructions loaded |
| `ConfigChange` | Config file changed |
| `CwdChanged` | Working directory changed |
| `FileChanged` | File system change detected |
| `WorktreeCreate` | Git worktree created |
| `WorktreeRemove` | Git worktree removed |
| `PostCompact` | After context compaction |
| `Elicitation` | User question/clarification |
| `ElicitationResult` | User answered |

### Cursor-Only Events

These events are only available when compiling for Cursor.
They pass through as-is in native Cursor layers.

| Event | Description |
|-------|-------------|
| `beforeShellExecution` | Before shell command |
| `afterShellExecution` | After shell command |
| `beforeMCPExecution` | Before MCP tool call |
| `afterMCPExecution` | After MCP tool call |
| `beforeReadFile` | Before file read |
| `afterFileEdit` | After file edit |
| `afterAgentResponse` | After agent response |
| `afterAgentThought` | After agent thought |
| `beforeTabFileRead` | Before tab file read |
| `afterTabFileEdit` | After tab file edit |

## Merge Behavior

Hooks merge across layers using deep-merge:
- Hook arrays for the same event are concatenated.
- Force-locked entries cannot be overridden by later layers.
- Entries from later layers are appended after earlier layers' entries.

## File Structure Differences

Claude embeds hooks inside `settings.json` under the `hooks` key.
Cursor uses a separate `hooks.json` file.

Keel handles this difference automatically based on the target agent's `AgentProfile`.

## What's Next

- **[Compatibility Matrix]({{< relref "compatibility-matrix" >}})** — exhaustive cross-agent reference for all hook events, handler types, and behavioral differences.
- **[DSL Reference]({{< relref "../guide/dsl" >}})** — the `hooks` field in `keel.yml`.
- **[Governance]({{< relref "../guide/governance" >}})** — force-locking hook entries.
- **[Settings Reference]({{< relref "settings" >}})** — settings that control hook behavior.
