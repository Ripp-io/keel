---
title: Conditionals & Variables
weight: 4
aliases:
  - /docs/guide/carve-outs/
---

# Conditionals & Variables

Customize compiled output per-agent or per-environment from shared configuration files.
Write one instruction file or JSON config that adapts to whatever agent you compile for.

## Quick Examples

### Variable substitution

Define variables in `keel.yml`:

```yaml
variables:
  api_url: "https://api.example.com"
  environment: "production"
```

Reference them in instruction files with `${keel:name}`:

```markdown
Deploy to ${keel:environment} using ${keel:api_url}.
```

Compiled output:

```markdown
Deploy to production using https://api.example.com.
```

### Agent-specific instructions

Use conditional markers in markdown to include content only for certain agents:

```markdown
## Version Control Workflow

<!-- keel-if: keel.agent == "claude" -->
Use worktrees for parallel feature work.
<!-- keel-elif: keel.agent == "cursor" -->
Use branch switching for feature work.
<!-- keel-endif -->
```

When compiling for Claude, only the worktree block appears.
When compiling for Cursor, only the branch switching block appears.
All marker lines are stripped from the output.

### Agent-specific JSON

Target an MCP server or other JSON object to a specific agent:

```json
{
  "mcpServers": {
    "claude-only-server": {
      "_keel": { "condition": "keel.agent == 'claude'" },
      "command": "claude-specific-mcp"
    }
  }
}
```

This server only appears when compiling for Claude.
The `_keel` key is stripped from the final output.

## Variables

### Defining variables

```yaml
variables:
  api_url: "https://api.example.com"
  environment: "production"
```

Variables merge across layers — later layers override earlier ones for the same key.

### Auto-injected variables

Keel automatically injects these variables:

| Variable | Description |
|----------|-------------|
| `keel.agent` | Target agent name (`claude`, `cursor`, etc.) |
| `keel.scope` | Compilation scope (`project`, `user`, etc.) |
| `keel.name` | Project name from keel.yml |
| `keel.output` | Output directory path |
| `keel.version` | Keel build version |
| `keel.bin.project` | Path to project-level bin directory (`.keel/bin/`) |
| `keel.bin.global` | Path to global bin directory (`~/.keel/bin/`) |
| `keel.layers.count` | Number of layers compiled |
| `keel.state` | State directory path (`.keel/state/`) |
| `keel.deps.<name>.path` | Full path to an installed dependency binary |
| `keel.deps.<name>.dir` | Directory containing an installed dependency |

`keel.agent` and `keel.scope` are always available and cannot be overridden.
Dep variables are injected when `--allow-install` is used.

### Substitution syntax

Reference variables with `${keel:name}` in instructions, MCP configs, hooks, settings, and environment files:

```markdown
Agent: ${keel:keel.agent}
Wiki: ${keel:company_wiki}
```

Unknown variables (not defined in any layer) are left as-is in the output.
Substitution runs at finalization, after [snippets]({{< relref "snippets" >}}) and conditionals have been processed.

## Markdown Conditionals

HTML comment markers define conditional sections in instruction files.

### Syntax

```markdown
<!-- keel-if: expression -->
Content for matching agents.
<!-- keel-elif: expression -->
Content for other agents.
<!-- keel-else -->
Fallback content.
<!-- keel-endif -->
```

Blocks are sequential: `keel-if`, then zero or more `keel-elif`, then an optional `keel-else`, closed by `keel-endif`.
Multiple conditional groups can appear in the same file.
Content outside any conditional markers is shared — it appears in all agents' output.

### List membership

Match against multiple agents in a single condition:

```markdown
<!-- keel-if: keel.agent in ["claude", "cursor"] -->
Hook-injected coordination messages.
<!-- keel-else -->
Manual polling for coordination.
<!-- keel-endif -->
```

### Nesting

Conditionals can be nested:

```markdown
<!-- keel-if: keel.agent == "claude" -->
Claude-specific content.
<!-- keel-if: keel.scope == "project" -->
Project-scoped Claude content.
<!-- keel-endif -->
<!-- keel-endif -->
```

Conditional markers inside fenced code blocks are treated as literal text, not as directives.
This lets you document conditional syntax without triggering the parser.

## JSON Conditionals

Use `_keel.condition` to target JSON objects to specific agents or environments:

```json
{
  "_keel": { "condition": "keel.agent == 'claude'" },
  "command": "claude-mcp"
}
```

```json
{
  "_keel": { "condition": "keel.agent in ['claude', 'cursor']" },
  "command": "multi-agent-mcp"
}
```

```json
{
  "_keel": { "condition": "keel.agent != 'cursor'" },
  "command": "any-but-cursor-mcp"
}
```

Conditions can be combined with [force-locks]({{< relref "governance" >}}):

```json
{
  "_keel": {
    "condition": "keel.agent == 'claude'",
    "force": true
  },
  "command": "required-claude-mcp"
}
```

This server is Claude-only and cannot be overridden by downstream layers.

## Expression Reference

| Operator | Example | Description |
|----------|---------|-------------|
| `==` | `keel.agent == 'claude'` | Equality |
| `!=` | `keel.agent != 'cursor'` | Inequality |
| `in` | `keel.agent in ['claude', 'cursor']` | List membership |
| `not in` | `keel.agent not in ['gemini']` | List exclusion |
| `\|\|` | `keel.agent == 'claude' \|\| keel.agent == 'cursor'` | Logical OR |
| `&&` | `keel.scope == 'project' && keel.agent == 'claude'` | Logical AND |

The left side is a dotted variable path.
The right side is a quoted string (single or double quotes) for `==`/`!=`, or a bracket-delimited list for `in`/`not in`.
`&&` binds tighter than `||`.

## When to Use What

| Approach | Use when |
|----------|----------|
| Conditionals | Same concept, different implementation per agent (e.g., worktree vs branch switching) |
| [Agent-targeted layers]({{< relref "layers#agent-targeted-layers" >}}) | An entire layer is specific to one agent (`schema: v1-claude`) |
| [Snippets]({{< relref "snippets" >}}) | Reusing the same text block across multiple files |
| Variables | Injecting environment or layer-level values into instructions |

If a large section is entirely agent-specific with no shared content, prefer agent-targeted layers over conditionals.

## What's Next

- **[Snippets]({{< relref "snippets" >}})** — template includes (complementary to conditionals).
- **[DSL Reference]({{< relref "dsl" >}})** — the `variables` field and `_keel` fields.
- **[Governance]({{< relref "governance" >}})** — force-locks on conditional objects.
