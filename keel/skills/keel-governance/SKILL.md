---
name: keel-governance
description: Add governance controls to Keel layers — force-locks, merge policies, and trust hierarchies. Use when building higher-trust layers that need to enforce policy across downstream layers.
---

# Keel Governance

Add governance controls that enforce policy across a layer hierarchy.
Higher-trust layers (company, org) lock values that downstream layers (team, project, user) cannot override.

## Core concepts

- **Force-lock**: A value that cannot be overridden by any downstream layer
- **Merge policy**: Controls how values combine across layers (additive, replace, deep-merge)
- **Trust hierarchy**: Layer order determines trust — earlier layers have higher trust
- **Governance metadata**: `_keel` blocks and `force:` fields are stripped from output

## Force-locking in keel.yml (DSL layers)

Use `force: true` on any FileRef entry:

```yaml
# Lock an entire MCP config — no downstream changes
mcp:
  servers:
    - source: mcp.json
      force: true

# Lock a specific skill directory
skills:
  - source: skills/
  - source: skills/security-review/
    force: true

# Lock permissions
permissions:
  rules:
    - source: permissions.json
      force: true
```

Force-lock applies to everything the FileRef points to — the entire file or directory.

## Force-locking in native JSON files

For granular, per-key locking in JSON files (settings, MCP, hooks), use `_keel` metadata:

### Per-server MCP lock
```json
{
  "mcpServers": {
    "security-scanner": {
      "_keel": { "force": true },
      "command": "security-scan",
      "args": ["--strict"]
    },
    "optional-tool": {
      "command": "optional-tool"
    }
  }
}
```

Only `security-scanner` is locked — `optional-tool` can be overridden.

### Per-hook lock
```json
{
  "PreToolUse": [
    {
      "matcher": "Bash",
      "hooks": [
        {
          "type": "command",
          "command": "scripts/security-check.sh",
          "force": true
        }
      ]
    }
  ]
}
```

The `force` field on hook entries locks that specific handler.

### Skill force-lock via frontmatter
```yaml
---
name: security-review
description: Mandatory security review skill
force: true
---
```

## Merge policies

Control how values combine when multiple layers contribute to the same key.

### In keel.yml
```yaml
permissions:
  rules:
    - source: permissions.json
  policy:
    merge: additive    # Downstream can add but not remove
```

### In native JSON via `_keel`
```json
{
  "_keel": { "merge": "additive" },
  "allow": ["Read", "Write"]
}
```

### Available strategies

| Strategy | Behavior | Use case |
|----------|----------|----------|
| `deep-merge` | Recursive merge, scalars last-wins | Settings, MCP (default) |
| `additive` | Downstream can add entries but not remove existing ones | Permissions |
| `replace` | Later layer completely replaces earlier | Rare — use when you need full override |

## Governance patterns

### Company security baseline

```yaml
# company-policy/keel.yml
schema: v1
name: company-security
description: Mandatory security controls

permissions:
  rules:
    - source: permissions.json
      force: true
  policy:
    merge: additive

mcp:
  servers:
    - source: mcp.json
      force: true

hooks:
  - source: hooks.json
    force: true

skills:
  - source: skills/security-review/
    force: true
```

Downstream layers can add permissions, MCP servers, and hooks — but cannot remove or modify locked ones.

### Team layer with selective locks

```yaml
# team/keel.yml
schema: v1
name: team-conventions

mcp:
  servers:
    - source: mcp.json     # Team MCP servers, overridable by projects

skills:
  - source: skills/
  - source: skills/code-review/
    force: true             # This specific skill is team-mandated
```

### Project layer with no governance

```yaml
# project/keel.yml
schema: v1
name: my-project

instructions:
  - source: instructions.md

skills:
  - source: skills/           # Project skills, overridable by user layer
```

## Verifying governance

After compiling a multi-layer stack:

```bash
keel compile \
  --layer ./company \
  --layer ./team \
  --layer ./project \
  --output claude=. --summary
```

The summary shows force-locked items:

```
Force-locked:
  hook/PreToolUse by company-security
  mcp/sonarqube by company-security
  skill/security-review/SKILL.md by company-security
```

Use `keel explain` for full provenance of every output value.

## Important rules

- Only higher-trust layers should use force-locks — project and user layers rarely need them
- `_keel` metadata is always stripped from compiled output — the agent never sees it
- Do NOT put `_keel` blocks in `keel.yml` — use `force:` on FileRef entries instead
- Force-locks apply to the entire file/directory pointed to by the FileRef
- For per-key locking within a JSON file, use `_keel` inside the JSON file itself
