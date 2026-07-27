---
title: Keel
type: docs
bookToC: false
---

<div class="landing-hero">

# Keel

### One config, every AI agent, governed from day one.

Your AI agent config is scattered across projects, duplicated between teams, and impossible to enforce.
Keel compiles layered configuration into any agent's native format — Claude Code, Cursor, and beyond.

<a class="landing-cta" href="{{< relref "docs/getting-started" >}}">Get Started →</a>

</div>

## One Command to a Configured Agent

Pull a community config and compile — Keel detects your agent automatically:

```bash
keel add 'https://github.com/affaan-m/everything-claude-code.git#main'
```

Stack your own config on top:

```bash
keel add ./my-tools
```

Every `keel add` updates `.keel/config.yml` and recompiles.
The result is a managed config that your whole team can use:

```yaml
# .keel/config.yml
layers:
  - 'https://github.com/affaan-m/everything-claude-code.git#main'
  - ./my-tools

output:
  claude: .
```

```bash
keel compile    # repeatable builds from config
```

Same layers, different agents — add outputs:

```bash
keel compile --output claude=. --output cursor=.
```

---

## What Keel Does

<div class="landing-features">

<div class="landing-feature">

### Community

Pull skills, commands, and configs from any Git repo or HTTP archive.
Existing repos with `.claude/` or `.cursor/` work as layers with no changes by the author.

[Browse catalog →]({{< relref "docs/getting-started/community-catalog" >}})

</div>

<div class="landing-feature">

### Cross-Agent

Compile the same config to Claude Code, Cursor, and more.
Switch agents without rewriting your setup.
Native config works as a layer — no migration required.

[Learn more →]({{< relref "docs/guide/transpilation" >}})

</div>

<div class="landing-feature">

### Layers

Stack configuration from community → company → team → project → personal.
Each layer adds or overrides the layers before it.
Ship scripts and binary dependencies alongside your config.

[Learn more →]({{< relref "docs/guide/layers" >}})

</div>

<div class="landing-feature">

### Import

Already have Claude or Cursor config? Import it as a Keel layer in one command.
Keel reads your native config and produces a `keel.yml` with companion files.

[Learn more →]({{< relref "docs/guide/transpilation#quick-examples" >}})

</div>

<div class="landing-feature">

### Templating

Variables, conditionals, and reusable snippets built into the compiler.
[A/B test prompts]({{< relref "docs/guide/prompt-testing" >}}), adapt instructions per-agent, create parameterized skill templates — all at compile time.

[Learn more →]({{< relref "docs/guide/conditionals" >}})

</div>

<div class="landing-feature">

### Governance

Higher-trust layers lock values that downstream layers can't override.
Force-lock security hooks, mandate MCP servers, enforce permissions.
Lockfile pins layer hashes for reproducible builds.

[Learn more →]({{< relref "docs/guide/governance" >}})

</div>

</div>

---

## Use Cases

<div class="landing-usecases">

<div class="landing-usecase">

#### Personal Toolkit
Assemble your global agent config from community repos and personal tools in one command.

[Guide →]({{< relref "docs/guide/personal-toolkit" >}})

</div>

<div class="landing-usecase">

#### Multi-Agent Portability
Write config once, compile to Claude Code and Cursor from the same source.

[Guide →]({{< relref "docs/guide/transpilation" >}})

</div>

<div class="landing-usecase">

#### Enterprise Policy
Enforce security hooks, mandate MCP servers, and lock permissions across every team.

[Guide →]({{< relref "docs/guide/enterprise-example" >}})

</div>

<div class="landing-usecase">

#### CI/CD Code Review
Run governed, headless agents in pipelines with force-locked guardrails.

[Guide →]({{< relref "docs/guide/governance" >}})

</div>

<div class="landing-usecase">

#### Developer Onboarding
New developer joins, runs `keel add`, gets the full agent environment — instructions, MCP servers, permissions, skills, tools.

[Guide →]({{< relref "docs/guide/layers" >}})

</div>

<div class="landing-usecase">

#### Prompt A/B Testing
Compile prompt variants with `--var`, run both against the same task, compare results.

[Guide →]({{< relref "docs/guide/prompt-testing" >}})

</div>

</div>

---

## From Personal Toolkit to Enterprise Policy

Keel scales with you.
Start by assembling your personal config from community sources.
When your team needs consistency, add a shared layer.
When your org needs governance, add force-locks and policies.

<div class="landing-paths">

<div class="landing-path">

#### Individual Developer

Assemble your agent config from community repos and personal tools.
`keel add` installs each layer and compiles in one step.

```
~/.claude/
├── CLAUDE.md          ← community + personal instructions
├── settings.json      ← your model preferences
├── commands/          ← community + your commands
├── skills/            ← community skills
└── agents/            ← community agents
```

[Personal Toolkit →]({{< relref "docs/guide/personal-toolkit" >}})

</div>

<div class="landing-path">

#### Team Lead

Define team conventions once, compile into every project.
New developers run `keel compile` and get the full setup.

```yaml
# .keel/config.yml
layers:
  - ./team-config
  - ./project

output:
  claude: .
```

[Create a Layer →]({{< relref "docs/getting-started/create-a-layer" >}})

</div>

<div class="landing-path">

#### Platform Engineer

Enforce company-wide security policies across all teams.
Force-lock hooks, mandate scanners, lock down permissions.

```yaml
# .keel/config.yml
layers:
  - ./company
  - ./team
  - ./project

output:
  claude: .
  cursor: .
```

[Enterprise Example →]({{< relref "docs/guide/enterprise-example" >}})

</div>

</div>

---

## Zero Risk to Try

Keel compiles into your existing project — it doesn't replace your config, it adds to it.
Your manually created files are never touched.
`keel clean` removes only what Keel created, leaving everything else intact.

Already have a `.claude/` or `.cursor/` directory?
It works as a Keel layer as-is — no migration, no conversion, no lock-in.

## Open Source, Self-Hosted, No Vendor Lock-In

Keel is a CLI compiler — not a platform, not a SaaS, not an agent.
It works with any AI coding agent that reads config from the filesystem.
Your config stays in Git, follows your PR process, and runs wherever you do.

> The keel is the structural backbone of a ship — everything else is built on top of it, it keeps the vessel stable, and it's invisible from above.
> That's this project: the infrastructure underneath the AI agent that provides structure, governance, and stability, but the agent never knows it's there.
> It just sees the same native config files it always has.

<a class="landing-cta" href="{{< relref "docs/getting-started" >}}">Get Started →</a>
