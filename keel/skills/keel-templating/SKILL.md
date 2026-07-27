---
name: keel-templating
description: Use Keel's templating features — variables, conditionals, and snippets — to create dynamic, agent-aware configuration. Use when building layers that adapt per-agent, per-environment, or share reusable content blocks.
---

# Keel Templating

Create dynamic configuration that adapts based on the target agent, environment, or custom variables.
Keel processes templates at compile time — the agent sees fully resolved content, no template syntax.

## Variables

### Defining variables

In `keel.yml`:

```yaml
variables:
  api_url: "https://api.example.com"
  environment: "production"
  team: "platform"
```

### Using variables

In any markdown file (instructions, skills, commands, agents):

```markdown
Deploy to ${keel:environment} using ${keel:api_url}.
```

In JSON files (mcp.json, settings, hooks):

```json
{
  "mcpServers": {
    "api": {
      "env": { "BASE_URL": "${keel:api_url}" }
    }
  }
}
```

### Variable precedence (highest to lowest)

1. CLI `--var key=value` flags
2. CLI `--var-file vars.env` entries
3. Later layer `variables:` in keel.yml (overrides earlier layers)
4. Earlier layer `variables:` in keel.yml

### Auto-injected variables

These are always available without declaring them:

- `${keel:keel.agent}` — target agent (`claude`, `cursor`)
- `${keel:keel.scope}` — compilation scope (`project`, `user`)
- `${keel:keel.name}` — project name from keel.yml
- `${keel:keel.output}` — output directory path
- `${keel:keel.version}` — Keel build version
- `${keel:keel.bin.project}` — project bin dir
- `${keel:keel.bin.global}` — global bin dir
- `${keel:keel.layers.count}` — number of layers compiled
- `${keel:keel.deps.<name>.path}` — installed dep binary path
- `${keel:keel.deps.<name>.dir}` — installed dep directory

Auto-injected variables are protected — CLI `--var` cannot override them.

### Debugging variables

```bash
keel compile --layer ./my-layer --output claude=. --show-vars
```

Prints the full resolved variable map after compilation.

## Conditionals

Gate content on variables in markdown files.
Conditional blocks are evaluated at compile time and only matching content appears in output.

### Basic if/else

```markdown
<!-- keel-if: keel.agent == "claude" -->
Use worktrees for parallel feature work.
Run `/doctor` if the environment looks wrong.
<!-- keel-elif: keel.agent == "cursor" -->
Use branch switching for feature work.
Check the coordination MCP for team updates.
<!-- keel-else -->
Follow standard git branching workflow.
<!-- keel-endif -->
```

### Operators

| Operator | Example | Meaning |
|----------|---------|---------|
| `==` | `keel.agent == "claude"` | Equals |
| `!=` | `environment != "production"` | Not equals |
| `in` | `keel.agent in "claude,cursor"` | Value is in comma-separated list |
| `not in` | `keel.agent not in "cursor"` | Value is not in list |
| `&&` | `keel.agent == "claude" && environment == "prod"` | Both true |
| `\|\|` | `team == "platform" \|\| team == "infra"` | Either true |

### Conditionals in JSON

MCP server entries can be conditionally included:

```json
{
  "mcpServers": {
    "claude-only-tool": {
      "_keel": { "if": "keel.agent == \"claude\"" },
      "command": "claude-tool"
    },
    "shared-tool": {
      "command": "shared-tool"
    }
  }
}
```

The `_keel.if` condition is evaluated at compile time.
Entries that don't match are excluded from output.

### Rules

- Conditional markers must be on their own line
- Markers inside fenced code blocks (` ``` `) are treated as literal text
- Nesting is not supported — keep conditionals flat
- Undefined variables in conditions evaluate as empty string

## Snippets

Reusable content blocks shared across layers and files.

### Defining snippets

Create `.md` files in a `snippets/` directory:

```
snippets/
  review-checklist.md
  deploy-steps.md
```

Reference from keel.yml:

```yaml
snippets:
  - source: snippets/
```

### Using snippets

In any markdown file:

```markdown
## Review Process

<!-- keel:include review-checklist -->
```

The snippet name is the filename without `.md`.

### Parameterized snippets

Define placeholders in snippet content with `{{key}}`:

```markdown
<!-- snippets/checklist.md -->
- [ ] Run {{tool}} against the changes
- [ ] Verify {{environment}} deployment
- [ ] Update {{doc_link}}
```

Supply parameters at the include site:

```markdown
<!-- keel:include checklist tool="eslint" environment="staging" doc_link="README.md" -->
```

### Snippets with conditionals

Snippets can contain conditional blocks — they're resolved after inclusion:

```markdown
<!-- snippets/workflow.md -->
<!-- keel-if: keel.agent == "claude" -->
Use worktrees for isolation.
<!-- keel-else -->
Use branches for isolation.
<!-- keel-endif -->
```

This works because Keel's pipeline is: include snippets → evaluate conditionals → substitute variables.

### Snippet cross-layer accumulation

Snippets accumulate across layers.
A snippet defined in an earlier layer is available to later layers.
If two layers define the same snippet name, last writer wins.

## Patterns

### A/B prompt testing

Use variables to compile prompt variants:

```yaml
variables:
  review_style: detailed    # default
```

```markdown
<!-- keel-if: review_style == "concise" -->
Flag bugs and security issues only. No style feedback.
<!-- keel-elif: review_style == "detailed" -->
Review thoroughly: bugs, security, naming, test coverage, architecture.
<!-- keel-endif -->
```

```bash
keel compile --layer . --output claude=output/a -v review_style=concise
keel compile --layer . --output claude=output/b -v review_style=detailed
```

### Environment-specific config

```yaml
variables:
  environment: "development"
```

```markdown
Deploy to ${keel:environment}.

<!-- keel-if: environment == "production" -->
Run the full test suite before deploying. No shortcuts.
<!-- keel-else -->
Run smoke tests. Full suite is optional.
<!-- keel-endif -->
```

### Agent-specific MCP servers

```json
{
  "mcpServers": {
    "claude-memory": {
      "_keel": { "if": "keel.agent == \"claude\"" },
      "command": "memory-server"
    },
    "cursor-index": {
      "_keel": { "if": "keel.agent == \"cursor\"" },
      "command": "index-server"
    },
    "shared-api": {
      "command": "api-server"
    }
  }
}
```

### Reusable instruction components

```yaml
# keel.yml
snippets:
  - source: snippets/

instructions:
  - source: instructions.md
```

```markdown
<!-- instructions.md -->
# Project Setup

<!-- keel:include setup-steps -->

# Code Review

<!-- keel:include review-checklist tool="eslint" -->

# Deployment

<!-- keel:include deploy-steps environment="${keel:environment}" -->
```

Each snippet is maintained independently and composed at compile time.
