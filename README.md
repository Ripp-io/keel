# Keel

[![pipeline](https://gitlab.com/ripp.io/oss/agentic/keel/badges/main/pipeline.svg)](https://gitlab.com/ripp.io/oss/agentic/keel/-/pipelines)
[![coverage](https://gitlab.com/ripp.io/oss/agentic/keel/badges/main/coverage.svg)](https://gitlab.com/ripp.io/oss/agentic/keel/-/graphs/main/charts)
[![release](https://gitlab.com/ripp.io/oss/agentic/keel/-/badges/release.svg)](https://gitlab.com/ripp.io/oss/agentic/keel/-/releases)

**Governed configuration compiler for AI coding agents.**

Running one AI agent is easy. Configure it once, start coding.
Running AI agents across an organization — with consistent policy, multiple tools, and config that doesn't drift — is an infrastructure problem nobody has solved.

Keel is a compiler that merges layered configuration into native output for Claude Code, Cursor, and any future agent.
Company policy, team standards, project specifics, and personal preferences stack into a single governed environment.
The agent sees clean native config. It never knows it was compiled.

```bash
keel compile \
  --layer ./company-policy \
  --layer ./team-config \
  --layer ./project \
  --output claude=. --output cursor=.
```

One command. Both agents configured. Policy enforced.

> The keel is the structural backbone of a ship — everything else is built on top of it, it keeps the vessel stable, and it's invisible from above.
> That's this project: the infrastructure underneath the AI agent that provides structure, governance, and stability, but the agent never knows it's there.
> It just sees the same native config files it always has.

---

## Why a Compiler

AI agents discover config from the filesystem — dotfiles, project directories, markdown.
This works for one developer on one project.

It breaks at every organizational boundary:
- No way to enforce "this security hook is immutable"
- No way to merge config from five sources with clear precedence
- No portability between tools — switch agents, rewrite everything
- No governance — anyone can override anything

Keel treats agent config as a **build step**.
Layers merge deterministically, governance is enforced during compilation, policy metadata is stripped from output, and the agent receives the same native files it already knows how to read.

## What You Get

- **Layer stacking** — company → org → team → project → personal, with deterministic merge strategies
- **Cross-agent portability** — one config source compiles to Claude, Cursor, or any future target
- **Governance** — force-lock security hooks, mandate MCP servers, enforce permission baselines
- **Variables & conditionals** — adapt instructions per-agent, per-environment, at compile time
- **Zero lock-in** — existing `.claude/` or `.cursor/` directories work as layers with no migration
- **Agent Skills compatible** — SKILL.md files work across [35+ compatible agents](https://agentskills.io/)

## Use Cases

- **[Personal toolkit](https://ripp-io.gitlab.io/oss/agentic/keel/docs/guide/personal-toolkit/)** — assemble global agent config from community repos and personal tools
- **[Multi-agent portability](https://ripp-io.gitlab.io/oss/agentic/keel/docs/guide/transpilation/)** — write once, compile to Claude and Cursor from the same source
- **[Enterprise policy](https://ripp-io.gitlab.io/oss/agentic/keel/docs/guide/enterprise-example/)** — enforce hooks, mandate MCP servers, lock permissions across teams
- **[Developer onboarding](https://ripp-io.gitlab.io/oss/agentic/keel/docs/guide/layers/)** — one command, full agent environment
- **[Prompt A/B testing](https://ripp-io.gitlab.io/oss/agentic/keel/docs/guide/prompt-testing/)** — compile prompt variants with `--var`, compare results

## Quick Start

### Install

```bash
# Requires Go 1.26+
go install gitlab.com/ripp.io/oss/agentic/keel/cmd/keel@latest
```

### First compile

```bash
# Pull a community config and compile it for Claude
keel compile \
  --layer 'https://github.com/affaan-m/everything-claude-code.git#main' \
  --output claude=~ --scope user

# Or compile your own layers
keel compile --layer ./my-config --output claude=.

# Preview without writing
keel compile --dry-run --layer ./my-config --output claude=.

# Validate a layer
keel validate ./my-config

# Explain what produced each value
keel explain .
```

## Documentation

Full docs, guides, and reference: **[ripp-io.gitlab.io/oss/agentic/keel](https://ripp-io.gitlab.io/oss/agentic/keel/)**

## License

Apache 2.0
