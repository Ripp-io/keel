---
title: Enterprise Example
weight: 12
---

# Enterprise Example

A complete 5-layer enterprise setup showing every major Keel feature in context.
Create `.keel/config.yml` with `layers: ["layers/*"]` and `output: {claude: out}`, then run `keel compile`.

```
01-company    Keel DSL       Highest trust — security policies, shared snippets, force-locks
02-org        Claude native  Existing Claude config contributed as-is
03-team       Keel DSL       Team conventions, conditionals, code review agent
04-project    Keel DSL       Project config, snippet references, skills
05-user       Keel DSL       Lowest trust — personal preferences
```

## Layer Cascade

How files from each layer merge into compiled output:

<div class="cascade-wrap">
<table class="cascade-table">
<thead>
<tr>
  <th>Layer</th>
  <th>Instructions</th>
  <th>Permissions</th>
  <th>MCP</th>
  <th>Hooks</th>
  <th>Settings</th>
  <th>Commands</th>
  <th>Agents &amp; Skills</th>
  <th>Repos</th>
</tr>
</thead>
<tbody>
<tr class="layer-1">
  <td>01-company<span class="layer-name">Acme Corp</span></td>
  <td><div class="cell-content">instructions.md<span class="sub">+ variables, snippets</span></div></td>
  <td><div class="cell-content">permissions.json<span class="sub">Read, Edit</span><span class="sub force">additive policy</span></div></td>
  <td><div class="cell-content"><span class="force">sonarqube</span><span class="sub">denied: untrusted-server</span></div></td>
  <td><div class="cell-content"><span class="force">PreToolUse</span><span class="sub">secrets/ write block</span></div></td>
  <td><div class="cell-content">settings.claude<span class="sub">theme: dark</span></div></td>
  <td><div class="cell-content"><span class="force">security-scan.md</span></div></td>
  <td class="cell-empty"></td>
  <td class="cell-empty"></td>
</tr>
<tr class="layer-2">
  <td>02-org<span class="layer-name">Platform Eng</span></td>
  <td><div class="cell-content">CLAUDE.md<span class="sub">org standards</span></div></td>
  <td><div class="cell-content">.claude/settings<span class="sub">+Write, +Bash(git:*)</span></div></td>
  <td><div class="cell-content">jira</div></td>
  <td class="cell-empty"></td>
  <td class="cell-empty"></td>
  <td><div class="cell-content">compliance-check.md</div></td>
  <td><div class="cell-content">skills/review/</div></td>
  <td class="cell-empty"></td>
</tr>
<tr class="layer-3">
  <td>03-team<span class="layer-name">Data Platform</span></td>
  <td><div class="cell-content">instructions.md<span class="sub">conditionals, snippet ref</span></div></td>
  <td class="cell-empty"></td>
  <td><div class="cell-content">gitlab</div></td>
  <td class="cell-empty"></td>
  <td class="cell-empty"></td>
  <td><div class="cell-content">deploy.md</div></td>
  <td><div class="cell-content">agents/code-reviewer.md</div></td>
  <td><div class="cell-content">utils<span class="sub">shared-ui</span></div></td>
</tr>
<tr class="layer-4">
  <td>04-project<span class="layer-name">Demo Project</span></td>
  <td><div class="cell-content">instructions.md<span class="sub">snippet ref</span></div></td>
  <td><div class="cell-content">permissions.json<span class="sub">+Write, +npm, +node, +rg</span></div></td>
  <td class="cell-empty"></td>
  <td class="cell-empty"></td>
  <td><div class="cell-content">settings.cursor<span class="sub">model: sonnet</span></div></td>
  <td><div class="cell-content">build.md<span class="sub">status.md</span></div></td>
  <td><div class="cell-content">skills/debug/</div></td>
  <td><div class="cell-content">gateway<span class="sub">ui, service</span></div></td>
</tr>
<tr class="layer-5">
  <td>05-user<span class="layer-name">Dev Prefs</span></td>
  <td class="cell-empty"></td>
  <td><div class="cell-content">permissions.json<span class="sub">+grep, +ls</span></div></td>
  <td class="cell-empty"></td>
  <td class="cell-empty"></td>
  <td><div class="cell-content">settings.claude<span class="sub">model: opus</span></div></td>
  <td><div class="cell-content">my-shortcuts.md</div></td>
  <td class="cell-empty"></td>
  <td class="cell-empty"></td>
</tr>
</tbody>
<tbody>
<tr class="cascade-divider">
  <td></td>
  <td>▼ concatenate</td>
  <td>▼ additive</td>
  <td>▼ deep-merge</td>
  <td>▼ deep-merge</td>
  <td>▼ deep-merge</td>
  <td>▼ overlay</td>
  <td>▼ overlay</td>
  <td>▼ deep-merge</td>
</tr>
<tr class="cascade-compiled">
  <td>Compiled<span class="layer-name">Claude output</span></td>
  <td><div class="cell-content">CLAUDE.md<span class="sub">4 sections</span></div></td>
  <td><div class="cell-content">Read, Edit, Write<span class="sub">git, npm, node, rg, find, grep, ls</span></div></td>
  <td><div class="cell-content"><span class="force">sonarqube</span><span class="sub">jira, gitlab</span></div></td>
  <td><div class="cell-content"><span class="force">PreToolUse</span></div></td>
  <td><div class="cell-content">theme: light<span class="sub">model: opus</span></div></td>
  <td><div class="cell-content">6 commands<span class="sub">security-scan 🔒</span></div></td>
  <td><div class="cell-content">code-reviewer<span class="sub">review, debug</span></div></td>
  <td><div class="cell-content">5 repos<span class="sub">merged by name</span></div></td>
</tr>
</tbody>
</table>
</div>

## What Happens at Compile Time

`.keel/config.yml`:

```yaml
layers:
  - layers/*

output:
  claude: out
  cursor: out-cursor
```

```bash
keel compile
```

| Feature | How it resolves |
|---------|-----------------|
| Instructions | Concatenated from all 5 layers with `# --- layer ---` headers |
| Variables | `${keel:company_wiki}` → `https://wiki.acme-corp.com` in compiled instructions |
| Snippets | `<!-- keel:include reasoning-protocol -->` replaced with company snippet content |
| Conditionals | Claude output gets worktree instructions; Cursor gets branch switching |
| Permissions | Additive union: company `Read, Edit` + project `Write, Bash(npm:*)...` + user `Bash(grep:*)...` |
| MCP servers | 3 servers accumulated: sonarqube (force-locked), jira, gitlab |
| Hooks | Company's secrets write-blocker present and immutable |
| Commands | 6 commands overlaid from all layers; `security-scan` is force-locked |
| Agents | `code-reviewer` from team layer |
| Skills | `review` from org + `debug` from project |
| Settings | Model is `opus` (user overrides project's `sonnet` — not locked) |
| Repos | 5 repos deep-merged: 2 from team + 3 from project |

---

## Layer Details

## 01-company — Acme Corp

```
01-company/
├── keel.yml
├── instructions.md
├── permissions.json
├── mcp.json
├── hooks.json
├── settings.claude.json
├── commands/
│   └── security-scan.md
└── snippets/
    ├── reasoning-protocol.md
    └── security-checklist.md
```

**Features**: [variables]({{< relref "dsl#variables" >}}), [snippets]({{< relref "snippets" >}}), [force-locks]({{< relref "governance#force-lock-reference" >}}), [additive permissions]({{< relref "governance#permissions" >}}), [MCP governance]({{< relref "governance#mcp-governance" >}}), [files governance]({{< relref "dsl#files" >}})

```yaml
# keel.yml
schema: v1
name: Acme Corp
description: Enterprise-wide security standards and shared practices

variables:
  company_wiki: "https://wiki.acme-corp.com"
  jira_url: "https://acme-corp.atlassian.net"

instructions:
  - source: instructions.md

permissions:
  rules:
    - source: permissions.json
  policy:
    merge: additive

mcp:
  servers:
    - source: mcp.json
  policy:
    denied: [untrusted-server]

hooks:
  - source: hooks.json

commands:
  - source: commands/

snippets:
  - source: snippets/

settings:
  claude: settings.claude.json

files:
  "commands/security-scan.md":
    force: true
```

```markdown
<!-- instructions.md -->
# Acme Corp Engineering Standards

All engineering teams follow these standards:

- Use conventional commits: `feat:`, `fix:`, `chore:`, `docs:`
- All code must pass security scanning before merge
- No secrets in source code — use environment variables or vault references
- Follow the principle of least privilege for permissions
- Reference Jira tickets in commit messages

Company wiki: ${keel:company_wiki}
Jira: ${keel:jira_url}
```

```json
// permissions.json — additive merge metadata
{
  "allow": ["Read", "Edit"],
  "_keel": { "merge": "additive" }
}
```

```json
// mcp.json — force-locked SonarQube server
{
  "mcpServers": {
    "sonarqube": {
      "command": "npx",
      "args": ["sonarqube-mcp-server"],
      "env": { "SONAR_TOKEN": "${SONAR_TOKEN}" },
      "force": true
    }
  }
}
```

```json
// hooks.json — force-locked write blocker
{
  "PreToolUse": [
    {
      "matcher": "Write(**/secrets/**)",
      "hooks": [
        {"type": "command", "command": "echo 'Blocked: writing to secrets/ is not allowed' && exit 2"}
      ],
      "force": true
    }
  ]
}
```

```markdown
<!-- snippets/reasoning-protocol.md -->
Before each significant action:
1. State what you're about to do
2. Explain why
3. Identify risks
4. Proceed only if risks are acceptable
```

```markdown
<!-- snippets/security-checklist.md -->
Before committing code, verify:
- No hardcoded secrets, tokens, or API keys
- No credentials in configuration files
- Environment variables used for all sensitive values
- File permissions are appropriately restrictive
```

---

## 02-org — Platform Engineering

Claude native layer — existing config contributed without rewriting.

```
02-org/
├── CLAUDE.md
├── .mcp.json
└── .claude/
    ├── settings.json
    ├── commands/
    │   └── compliance-check.md
    └── skills/
        └── review/
            └── SKILL.md
```

**Features**: [cross-format compilation]({{< relref "transpilation" >}}) (native Claude layer compiles to Cursor too)

```markdown
<!-- CLAUDE.md -->
# Platform Engineering Standards

All services must expose health check endpoints at `/health`.
Use structured logging in JSON format.
Reference Jira tickets in all commit messages.
Follow the org API design guidelines for REST endpoints.
```

```json
// .mcp.json — adds Jira server (accumulated with company's SonarQube)
{
  "mcpServers": {
    "jira": {
      "command": "npx",
      "args": ["@anthropic-ai/claude-code-jira-mcp"],
      "env": { "JIRA_TOKEN": "${JIRA_TOKEN}", "JIRA_URL": "${JIRA_URL}" }
    }
  }
}
```

---

## 03-team — Data Platform Team

```
03-team/
├── keel.yml
├── instructions.md
├── mcp.json
├── commands/
│   └── deploy.md
└── agents/
    └── code-reviewer.md
```

**Features**: [conditionals]({{< relref "conditionals" >}}), [snippet references]({{< relref "snippets" >}}), [agents]({{< relref "dsl#agents" >}}), [repos]({{< relref "dsl#repos" >}})

```yaml
# keel.yml
schema: v1
name: Data Platform Team
description: Team conventions, deployment workflow, code review standards

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json

commands:
  - source: commands/

agents:
  - source: agents/

repos:
  utils:
    url: https://gitlab.acme-corp.com/platform/utils.git
    path: utils
    branch: main
  shared-ui:
    url: https://gitlab.acme-corp.com/platform/shared-ui.git
    path: shared-ui
    branch: main
    required: true
```

```markdown
<!-- instructions.md — conditionals + snippet reference -->
# Data Platform Team

We use GitLab for source control and CI/CD.
Feature branches follow: `feature/{ticket}-{description}`.
All merge requests require at least one approval.
Deploy to staging before production — never skip.

## Version Control Workflow

<!-- keel-if: keel.agent == "claude" -->
Use worktrees for parallel feature work.
Keep the main worktree on the default branch.
Create a new worktree for each feature branch.
<!-- keel-elif: keel.agent == "cursor" -->
Use branch switching for feature work.
Stash or commit changes before switching branches.
<!-- keel-endif -->

## Pre-Commit Checklist

<!-- keel:include security-checklist -->
```

```markdown
<!-- agents/code-reviewer.md -->
You are a code reviewer for the Data Platform team.

Review merge requests against team standards:
- Conventional commits
- Feature branch naming (`feature/{ticket}-{description}`)
- Test coverage for new code
- No direct commits to main
- Structured logging in new services

Be thorough but constructive. Suggest improvements, don't just flag problems.
```

---

## 04-project — Demo Project

```
04-project/
├── keel.yml
├── instructions.md
├── permissions.json
├── settings.cursor.json
├── commands/
│   ├── build.md
│   └── status.md
└── skills/
    └── debug/
        └── SKILL.md
```

**Features**: [snippet references]({{< relref "snippets" >}}), [skills]({{< relref "dsl#skills" >}}), [repos]({{< relref "dsl#repos" >}}) (merged with team repos)

```yaml
# keel.yml
schema: v1
name: Demo Project
version: dev
description: Multi-service project — gateway, UI, and backend

instructions:
  - source: instructions.md

permissions:
  rules:
    - source: permissions.json

commands:
  - source: commands/

skills:
  - source: skills/

settings:
  model: sonnet
  cursor: settings.cursor.json

repos:
  gateway:
    url: https://gitlab.acme-corp.com/demo/gateway.git
    path: gateway
    branch: main
    required: true
  ui:
    url: https://gitlab.acme-corp.com/demo/ui.git
    path: ui
    branch: main
  service:
    url: https://gitlab.acme-corp.com/demo/service.git
    path: service
    branch: main
    required: true
```

```markdown
<!-- instructions.md — references company snippet -->
# Demo Project

A multi-service project with a gateway, UI, and backend service.

## Architecture

- `gateway/` — API gateway, routes requests to backend services
- `ui/` — Frontend web application
- `service/` — Core backend service with PostgreSQL

## Development Workflow

<!-- keel:include reasoning-protocol -->

## Conventions

- Follow existing patterns in each service
- Gateway handles auth and routing — no business logic
- Service owns the data model and migrations
- UI communicates only through the gateway API
```

---

## 05-user — Developer Preferences

```
05-user/
├── keel.yml
├── permissions.json
├── settings.claude.json
└── commands/
    └── my-shortcuts.md
```

```yaml
# keel.yml
schema: v1
name: Developer Preferences
description: Personal model and workflow preferences

permissions:
  rules:
    - source: permissions.json

commands:
  - source: commands/

settings:
  model: opus
  claude: settings.claude.json
```

## What's Next

- **[Layers]({{< relref "layers" >}})** — how layers stack, merge, and resolve.
- **[Governance]({{< relref "governance" >}})** — force-locks and merge policies.
- **[Snippets]({{< relref "snippets" >}})** — cross-layer text reuse.
- **[Conditionals & Variables]({{< relref "conditionals" >}})** — agent-specific content.
