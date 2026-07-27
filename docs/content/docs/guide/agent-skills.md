---
title: "Agent Skills"
weight: 6
---

# Agent Skills Compatibility

Keel is compatible with the [Agent Skills](https://agentskills.io/) standard.
SKILL.md files following the Agent Skills format work natively in Keel layers — no wrapper syntax, no conversion step.
The compiler preserves standard frontmatter, strips Keel governance fields, and writes each skill to the correct output path for the target agent.

## How Skills Flow Through Layers

Skills use the **overlay** merge strategy.
Every layer that contributes a `skills/` directory adds files to the compiled output.
When two layers provide a skill with the same directory name, last writer wins — the later layer's version replaces the earlier one.

A higher-trust layer can **force-lock** a skill to prevent downstream layers from overriding it.
For example, a company layer can force-lock a security review skill that no project or user layer can replace.

```yaml
# keel.yml
skills:
  - source: skills/
  - source: skills/security-review/
    force: true
```

In this configuration, `source: skills/` contributes every skill directory found under `skills/`.
The second entry force-locks `security-review` — downstream layers that provide a skill with the same name are rejected during compilation.

## SKILL.md Format

Each skill is a directory containing a `SKILL.md` file (or `skill.md`) with YAML frontmatter and a Markdown body.
The directory name must match the `name` field in the frontmatter.

```
skills/
  security-review/
    SKILL.md
    scripts/       # optional
    references/    # optional
    assets/        # optional
```

### Frontmatter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Lowercase alphanumeric (including Unicode) + hyphens, max 64 characters. Must match the directory name. |
| `description` | Yes | Short summary, max 1024 characters. |
| `license` | No | SPDX license identifier (e.g. `MIT`, `Apache-2.0`). |
| `compatibility` | No | Environment requirements, max 500 characters (e.g. `Requires Python 3.11`). |
| `allowed-tools` | No | Tools the skill is permitted to use. |
| `metadata` | No | Arbitrary key-value pairs. |

### Example

```markdown
---
name: security-review
description: Review code for security vulnerabilities
---
Perform a security review of the code.
Check for injection, XSS, CSRF, and auth issues.
```

See the [Agent Skills specification](https://agentskills.io/) for the full format reference.

## Keel-Specific Extensions

Keel adds governance metadata to the SKILL.md frontmatter.
These fields control compilation behavior and are **stripped from the output** — the agent never sees them.

| Field | Type | Description |
|-------|------|-------------|
| `force` | `bool` | Lock this skill so downstream layers cannot override it. |
| `scope` | `string` | Path scope for the skill's applicability. |
| `merge` | `string` | Merge strategy override. |

A SKILL.md with Keel extensions looks like this in the source layer:

```markdown
---
name: security-review
description: Review code for security vulnerabilities
force: true
scope: /
---
Perform a security review of the code.
```

After compilation, the output contains only standard Agent Skills fields:

```markdown
---
name: security-review
description: Review code for security vulnerabilities
---
Perform a security review of the code.
```

`keel validate` accepts Keel extension fields without warning.
Only the compiler acts on them — they are governance metadata, not skill metadata.

## Validation

`keel validate` checks each skill directory against the Agent Skills specification.
All findings are **warnings**, not errors — Keel is permissive by design.

### Validation Rules

| Rule | Severity | Description |
|------|----------|-------------|
| SKILL.md exists | Warning | Each skill directory must contain a `SKILL.md` (or `skill.md`) file. |
| Frontmatter present | Warning | The file must contain YAML frontmatter delimited by `---`. |
| Frontmatter parseable | Warning | The YAML frontmatter must parse without errors. |
| Name required | Warning | `name` must be present and non-empty. |
| Name format | Warning | `name` must be lowercase alphanumeric (including Unicode) + hyphens, no leading/trailing/consecutive hyphens. |
| Name length | Warning | `name` must not exceed 64 characters (NFKC-normalized). |
| Name match | Warning | `name` must match the containing directory name (NFKC-normalized). |
| Description required | Warning | `description` must be present and non-empty. |
| Description length | Warning | `description` must not exceed 1024 characters. |
| Compatibility length | Warning | `compatibility`, if present, must not exceed 500 characters. |
| Metadata type | Warning | `metadata`, if present, must be a YAML mapping (not a scalar or list). |
| Unexpected fields | Warning | Frontmatter keys not in the Agent Skills spec (excluding Keel extensions) are flagged. |

Run validation against a layer:

```bash
keel validate /path/to/layer
```

## Supported Agents

The same SKILL.md file works across all Agent Skills-compatible agents without modification.
Keel compiles skills to the correct output path for each target.

| Target Agent | Output Path |
|-------------|-------------|
| Claude Code | `.claude/skills/<skill-name>/SKILL.md` |
| Cursor | `.cursor/skills/<skill-name>/SKILL.md` |

The Agent Skills standard is supported by 35+ agents, including Claude Code, Cursor, Codex, Gemini CLI, GitHub Copilot, and others.
Write the skill once.
Keel compiles it to every target.

## What's Next

- **[Agent Support]({{< relref "../ecosystem/agents" >}})** — which agents support the Agent Skills standard.
- **[Validation]({{< relref "validation" >}})** — how `keel validate` checks skill compliance.
- **[DSL Reference]({{< relref "dsl#skills" >}})** — the `skills` field in `keel.yml`.
