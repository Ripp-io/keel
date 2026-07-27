---
title: Getting Started
weight: 1
bookCollapseSection: false
---

# Getting Started

Install Keel, pull community config, and compile your AI agent's configuration.

## Install

```bash
go install gitlab.com/ripp.io/oss/agentic/keel/cmd/keel@latest
```

> Requires Go 1.26+

Verify:

```bash
keel --help
```

<details>
<summary>Build from source</summary>

```bash
git clone git@gitlab.com:ripp.io/oss/agentic/keel.git
cd keel
make build-cli
sudo mv dist/keel /usr/local/bin/
```
</details>

## Configure Your Agent

Pull a popular community config and compile it as your global setup:

```bash
keel compile \
  --layer 'https://github.com/affaan-m/everything-claude-code.git#main' \
  --output claude=~ --scope user
```

That's it.
Keel clones the repo, auto-detects its Claude native config (`.claude/`, `CLAUDE.md`), and compiles it to your user-level paths.

> **Safe to run on existing projects.** Keel writes alongside your existing config — it never overwrites files it didn't create.
> Run `keel clean` at any time to remove only what Keel produced, leaving your files untouched.

## Add Your Own Config

Create a personal layer to add your own preferences on top:

```bash
mkdir -p my-tools
```

`my-tools/keel.yml`:

```yaml
schema: v1
name: my-toolkit

instructions:
  - source: instructions.md

settings:
  model: opus
```

`my-tools/instructions.md`:

```markdown
# My Preferences

Use worktrees for parallel feature work.
Commit messages reference the ticket number.
```

Now stack your config on top of the community layer:

```bash
keel compile \
  --layer 'https://github.com/affaan-m/everything-claude-code.git#main' \
  --layer ./my-tools \
  --output claude=~ --scope user
```

Community defaults load first, your personal config gets the final say.

## Onboard in 30 Seconds

A new team member clones the repo, runs one command, and has a fully configured AI agent environment:

```bash
git clone https://gitlab.example.com/team/project.git
cd project
keel compile
```

That's the entire onboarding workflow for AI tools.
No "ask Sarah for her CLAUDE.md" or "copy the config from the wiki."
The `.keel/config.yml` in the repo defines exactly what layers to compile, what governance applies, and what agents to target.

Everyone gets the same setup — same permissions, same MCP servers, same instructions, same commands.
Personal preferences go in `.keel/local/` (gitignored), so individuals can customize without affecting the shared config.

## Make It Repeatable

For project-level config, create a `.keel/config.yml` so anyone on the team can run `keel compile` with no arguments:

```yaml
layers:
  - ./layers/*

output:
  claude: .
  cursor: .
```

```bash
keel compile
```

Commit `.keel/config.yml` to your repo.
CLI flags are for quick experiments and CI overrides — the config file is the standard project workflow.

## Compile for Multiple Agents

Add agents to your config file and every `keel compile` targets them all.
The same layers compile for any supported agent — one source, every agent.

## What's Next

- **[Community Catalog]({{< relref "community-catalog" >}})** — browse popular skills, commands, and agent configs you can add as layers.
- **[Agent Migration]({{< relref "../guide/agent-migration" >}})** — already have Cursor or Claude config? Import it and compile for any agent.
- **[Personal Toolkit]({{< relref "../guide/personal-toolkit" >}})** — assemble your global agent config from community and personal sources.
- **[Create a Layer]({{< relref "create-a-layer" >}})** — write your own `keel.yml`, stack layers, and compile shared config for your team.
- **[Enterprise Example]({{< relref "../guide/enterprise-example" >}})** — a complete 5-layer setup with governance, snippets, conditionals, and cross-layer references.
