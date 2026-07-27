# Demo: Enterprise Layer Compilation

Demonstrates Keel's multi-layer configuration compilation across an enterprise hierarchy.
Five layers show how config flows from company-wide security policies down to individual developer preferences.

## Scenario

A fictional enterprise (Acme Corp) with 5 configuration layers, ordered from highest trust to lowest:

```
01-company    Keel DSL       Acme Corp — security standards, shared snippets, force-locked policies
02-org        Claude native   Platform Engineering — org already uses Claude, contributes as-is
03-team       Keel DSL       Data Platform Team — team conventions, conditionals, code review agent
04-project    Keel DSL       Demo Project — project skills, snippet references, repos
05-user       Keel DSL       Developer — personal model and workflow preferences
```

Numeric prefixes ensure deterministic ordering when using glob expansion (`layers/*`).

## Why mixed formats?

The org layer (02) is Claude native — they already have a `.claude/` directory in their repos and contribute it as-is without learning the Keel DSL.
Every other layer uses the Keel DSL for full feature access.
Keel compiles all formats together, applying cross-format translation when compiling for an agent that differs from the layer's native format.

## What Each Layer Demonstrates

### 01-company — Keel DSL (highest trust)
- **Instructions**: Company coding standards with variable substitution (`${keel:company_wiki}`)
- **Variables**: `company_wiki` and `jira_url` available to all downstream layers
- **Snippets**: `reasoning-protocol` and `security-checklist` — reusable text blocks for downstream layers
- **Force-lock**: Security scan command cannot be overridden (`files` governance)
- **Force-lock**: Hook blocking writes to `secrets/` is immutable
- **MCP**: SonarQube server is force-locked — cannot be removed
- **MCP policy**: `untrusted-server` is denied
- **Permissions**: Additive merge — downstream layers can only add, not remove
- **Env passthrough**: `${SONAR_TOKEN}` resolved at runtime, never baked in

### 02-org — Claude native
- **Instructions**: `CLAUDE.md` with org practices (appended after company)
- **Permissions**: `.claude/settings.json` extends company's — adds Write and git
- **MCP**: `.mcp.json` adds Jira server (accumulated with company's SonarQube)
- **Commands**: `.claude/commands/compliance-check.md` overlaid alongside company's security-scan
- **Skills**: `.claude/skills/review/` — platform engineering review skill

### 03-team — Keel DSL
- **Instructions**: Team conventions with **conditionals** — worktree workflow for Claude, branch switching for Cursor
- **Snippet reference**: `<!-- keel:include security-checklist -->` pulls from company's snippets
- **MCP**: GitLab server (now 3 MCP servers total)
- **Commands**: Deploy command
- **Agents**: Code reviewer agent definition
- **Repos**: Team repos (utils, shared-ui) — merged with project repos downstream

### 04-project — Keel DSL
- **Instructions**: Project architecture with **snippet reference** — `<!-- keel:include reasoning-protocol -->` pulls from company
- **Permissions**: Project-level permissions — unions with company's additive base
- **Commands**: Build and status commands
- **Skills**: Debug skill for the project's multi-service architecture
- **Settings**: Sets model to sonnet
- **Repos**: Project-specific repositories (merged with team repos)

### 05-user — Keel DSL (lowest trust)
- **Permissions**: Adds grep and ls to personal permissions
- **Commands**: Personal workflow shortcuts
- **Settings**: Overrides model to opus, theme to light

## Features Demonstrated

| Feature | Where | Behavior |
|---------|-------|----------|
| **Snippets** | Company defines, team + project reference | Cross-layer text reuse |
| **Variables** | Company defines `company_wiki`, `jira_url` | Substituted in instructions |
| **Conditionals** | Team instructions | Agent-specific content per target |
| **Agents** | Team layer | Code reviewer agent definition |
| **Skills** | Org (native) + project (DSL) | Review skill + debug skill |
| **Force-lock** | Company commands, hooks, MCP | Immutable across all downstream layers |
| **Files governance** | Company `files` block | Per-file force-lock policy |
| **Additive permissions** | Company policy | Downstream can add but not remove |
| **Cross-format** | Claude native layer compiled for Cursor | Translated via IR |
| **Repos** | Team + project | Deep-merged by name |
| **Env passthrough** | MCP server tokens | Variables pass through to native output |

## Compilation Strategies

| Strategy | Example | Behavior |
|----------|---------|----------|
| **Concatenate** | Instructions from all layers | Appended with `# --- layer ---` section headers |
| **Deep merge** | Settings from all layers | Objects merge, arrays replace (last-writer-wins) |
| **Overlay** | Commands, agents, skills | All files collected, same-name files overwritten |
| **Force-lock** | Company's security-scan, secrets hook, SonarQube | Downstream layers cannot override |
| **Additive** | Permissions with additive policy | Union of all allow/deny arrays |
| **Env passthrough** | `${SONAR_TOKEN}`, `${GITLAB_TOKEN}` | Variables pass through to tool-native output |

## Building

```bash
make              # compile for both claude and cursor
make clean        # remove compiled output
```

Output is written to `output/claude/` and `output/cursor/`, which are git-ignored.
