---
title: Frontmatter Reference
weight: 4
---

# Frontmatter Reference

YAML frontmatter in markdown files controls metadata for agents, skills, commands, and rules.
Keel handles frontmatter during compilation — preserving portable fields, translating where possible, and stripping Keel-specific governance fields.

## Keel Governance Fields

These fields are Keel-specific and stripped from all output.
They never appear in the agent's native config.

| Field | Applies To | Description |
|-------|-----------|-------------|
| `force` | instructions, commands, skills | Force-lock this file across layers |
| `scope` | instructions | Compilation scope targeting |
| `merge` | instructions | Merge strategy override |

```markdown
---
force: true
---
# Security Standards

This file cannot be overridden by downstream layers.
```

## Agent Frontmatter

Fields for sub-agent definition files (`.claude/agents/*.md`, `.cursor/agents/*.md`).

### Portable Fields

| Field | Claude | Cursor | Description |
|-------|--------|--------|-------------|
| `name` | ✅ | ✅ | Agent display name |
| `description` | ✅ | ✅ | Agent description |
| `model` | ✅ | ✅ | Model to use |

### Claude-Only Fields

| Field | Description |
|-------|-------------|
| `tools` | Allowed tools list |
| `disallowedTools` | Disallowed tools list |
| `permissionMode` | Permission mode |
| `maxTurns` | Maximum conversation turns |
| `skills` | Skills available to this agent |
| `mcpServers` | MCP servers for this agent |
| `hooks` | Agent-specific hooks |
| `memory` | Memory file references |
| `background` | Run in background |
| `effort` | Effort level (`low`, `medium`, `high`, `xhigh`) |
| `isolation` | Isolation mode |
| `color` | Display color |
| `initialPrompt` | Initial prompt text |

### Cursor-Only Fields

| Field | Description |
|-------|-------------|
| `readonly` | Read-only mode |
| `is_background` | Run in background (Cursor equivalent of Claude's `background`) |

### Cross-Compilation

Claude-to-Cursor is heavily lossy — most Claude agent frontmatter fields are dropped with warnings.
Cursor-to-Claude is mostly lossless — Cursor's limited set maps cleanly.

## Skill Frontmatter

Fields for skill definition files (`SKILL.md`).

### Agent Skills Spec (Portable)

These fields are from the [Agent Skills specification](https://agentskills.io/specification) and work in both Claude and Cursor:

| Field | Description |
|-------|-------------|
| `name` | Skill name (kebab-case) |
| `description` | Skill description |
| `paths` | File paths the skill applies to |
| `license` | License identifier |
| `compatibility` | Compatibility information |
| `metadata` | Arbitrary metadata object |
| `disable-model-invocation` | Prevent model from invoking |
| `allowed-tools` | Tools the skill can use |

### Claude Extensions

These fields extend the Agent Skills spec and are Claude-only:

| Field | Description |
|-------|-------------|
| `when_to_use` | Trigger description for automatic invocation |
| `argument-hint` | Hint text for arguments |
| `arguments` | Argument schema |
| `user-invocable` | Whether users can invoke directly |
| `model` | Model override |
| `effort` | Effort level |
| `context` | Additional context files |
| `agent` | Agent to delegate to |
| `hooks` | Skill-specific hooks |
| `shell` | Shell environment |

When cross-compiling Claude skills to Cursor, the extension fields are dropped with warnings.

## Command Frontmatter

Fields for slash command files.

| Field | Claude | Cursor | Description |
|-------|--------|--------|-------------|
| `description` | ✅ | ❌ | Command description |
| `allowed-tools` | ✅ | ❌ | Tools the command can use |
| `model` | ✅ | ❌ | Model override |
| `argument-hint` | ✅ | ❌ | Argument hint text |
| `disable-model-invocation` | ✅ | ❌ | Prevent model invocation |
| `user-invocable` | ✅ | ❌ | Whether users can invoke directly |

> [!WARNING]
> Cursor does not support command frontmatter at all.
> All command frontmatter is stripped with warnings when compiling for Cursor.

## Rule Frontmatter

Fields for rule files (`.claude/rules/*.md`, `.cursor/rules/*.mdc`).

| Field | Claude | Cursor | Description |
|-------|--------|--------|-------------|
| `paths` | ✅ | ❌ | File patterns for scoped activation |
| `alwaysApply` | ❌ | ✅ | Always active regardless of context |
| `globs` | ❌ | ✅ | File glob patterns (similar to Claude's `paths`) |
| `description` | ❌ | ✅ | Enables agent-requested activation mode |

There is no 1:1 mapping between Claude's `paths` and Cursor's `globs`, though they serve similar purposes.
Claude rules without `paths` load at launch (equivalent to Cursor's `alwaysApply: true`).

## Body-Level Patterns

These patterns appear in the body of markdown files (not frontmatter) and are agent-specific:

| Pattern | Agent | Description |
|---------|-------|-------------|
| `$ARGUMENTS` | Claude | Positional argument substitution |
| `$ARGUMENTS[N]` | Claude | Indexed argument access |
| `$0`, `$1`, `$2` | Claude | Positional arguments (shell-style) |
| `${CLAUDE_SKILL_DIR}` | Claude | Resolves to skill directory |
| `${CLAUDE_SESSION_ID}` | Claude | Resolves to session ID |
| `` !`command` `` | Claude | Shell execution, output embedded |
| `{{variable}}` | Cursor | Template variable (Maestro) |

The `keel validate --lint` command detects these patterns and warns about non-portable usage.

## What's Next

- **[Compatibility Matrix]({{< relref "compatibility-matrix" >}})** — exhaustive cross-agent reference for every config file, setting, and frontmatter field.
- **[Hooks Reference]({{< relref "hooks" >}})** — hook event portability.
- **[Settings Reference]({{< relref "settings" >}})** — settings key portability.
- **[Conditionals & Variables]({{< relref "../guide/conditionals" >}})** — agent-specific content and variable substitution.
