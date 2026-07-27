---
name: keel-skill
description: Write an Agent Skills-compliant SKILL.md file with proper frontmatter, directory structure, and optional companion files. Use when creating skills for any Agent Skills-compatible agent.
---

# Write a Keel Skill

Create a skill that follows the [Agent Skills](https://agentskills.io/) standard.
Skills written to this spec work across 35+ compatible agents including Claude Code, Cursor, Codex, and Gemini CLI.

## Step 1: Create the directory

Each skill lives in its own directory.
The directory name must match the `name` field in the frontmatter.

```
skills/
  my-skill/
    SKILL.md              # Required — skill definition
    instructions.md       # Optional — companion instructions
    scripts/              # Optional — helper scripts
    references/           # Optional — reference material
    assets/               # Optional — images, data files
```

## Step 2: Write SKILL.md frontmatter

### Required fields

```yaml
---
name: my-skill
description: What this skill does (max 1024 chars)
---
```

### Optional fields (Agent Skills spec)

```yaml
---
name: my-skill
description: What this skill does
license: MIT
compatibility: Requires Node.js 18+
paths:
  - src/**/*.ts
  - tests/**/*.ts
allowed-tools:
  - Read
  - Write
  - Bash
metadata:
  author: my-org
  version: "1.0"
disable-model-invocation: false
---
```

### Name rules

- Lowercase alphanumeric + hyphens only
- No leading, trailing, or consecutive hyphens
- Max 64 characters
- Must match the directory name exactly

## Step 3: Write the skill body

The body is markdown that instructs the agent on how to perform the skill.
Write clear, actionable steps.

```markdown
---
name: security-review
description: Review code for security vulnerabilities
---

# Security Review

Perform a security review of the code changes.

## What to check

- Input validation on all external data
- No hardcoded credentials or API keys
- SQL injection protection (parameterized queries)
- XSS protection (output encoding)
- Authentication and authorization checks

## How to report

For each finding:
1. State the vulnerability type
2. Show the affected code
3. Explain the risk
4. Suggest a fix with a code example
```

## Step 4: Add companion files (optional)

Companion files provide additional context without inflating the SKILL.md body:

- **`instructions.md`** — detailed instructions loaded alongside the skill
- **`scripts/`** — shell scripts the skill can reference via hooks or commands
- **`references/`** — reference material (coding standards, API docs, checklists)

Companion files are copied to the output alongside SKILL.md during compilation.

## Step 5: Keel governance extensions

Keel adds governance fields to the frontmatter.
These control compilation behavior and are **stripped from output** — the agent never sees them.

```yaml
---
name: security-review
description: Review code for security vulnerabilities
force: true       # Lock this skill — downstream layers cannot override
---
```

| Field | Type | Purpose |
|-------|------|---------|
| `force` | bool | Prevent downstream layers from overriding this skill |
| `scope` | string | Path scope for applicability |
| `merge` | string | Merge strategy override |

## Step 6: Validate

```bash
keel validate /path/to/layer
```

Validation checks:
- SKILL.md exists in each skill directory
- Frontmatter is valid YAML
- `name` is present, correctly formatted, and matches directory name
- `description` is present and within length limits
- Unknown frontmatter fields are flagged (warnings, not errors)

## Cross-agent portability

The Agent Skills spec fields are portable across all compatible agents.
Claude Code adds extension fields (`when_to_use`, `arguments`, `model`, `effort`, `context`, `agent`, `hooks`, `shell`, `argument-hint`, `user-invocable`) that are Claude-only.

When compiling a Claude skill for Cursor, extension fields are dropped with warnings.
When compiling a Cursor skill for Claude, no data is lost (Cursor uses only the base spec).

Write skills using only the base spec fields for maximum portability.
Use Claude extensions only when you need Claude-specific behavior and accept the portability trade-off.
