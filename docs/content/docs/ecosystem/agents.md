---
title: Agent Support
weight: 1
---

# Agent Support

Keel targets AI coding agents at two independent tiers:

- **Compiler** — can Keel compile config into this agent's native format? Any agent that reads config from the filesystem is a compiler target.
- **Container** — can Keel package the agent in a container for isolated delivery? Requires a CLI binary or headless mode.

These tiers are independent.
A desktop IDE can be a compiler target without being containerizable.
A CLI agent can be both.

## Compatibility Matrix

| Agent | Compiler | Container | Status | Config Format | Skills |
|-------|----------|-----------|--------|---------------|--------|
| **Claude Code** | ✅ | ✅ CLI/Dev/Runner | Shipped | `.claude/`, `CLAUDE.md`, JSON | ✅ |
| **Cursor Agent** | ✅ | ✅ CLI/Dev | Shipped | `.cursor/`, `.cursorrules`, JSON/MDC | ✅ |
| **AGENTS.md** | ✅ | — | Shipped | `AGENTS.md` (AAIF standard) | ✅ |
| **Theia IDE** | ✅ | ⚠️ Web | Adapter shipped | VS Code extension model | — |
| **Gemini CLI** | 🔜 | 🔜 | Planned | `~/.gemini/`, `GEMINI.md` | ✅ |
| **Goose** | 🔜 | 🔜 | Planned | `~/.config/goose/`, `.goosehints` | ✅ |
| **Codex CLI** | 🔜 | ⚠️ | Planned | `AGENTS.md`, `codex.yaml` | ✅ |
| **GitHub Copilot** | 🔜 | ⚠️ | Evaluating | `.github/copilot-instructions.md` | ✅ |
| **Amp** | 🔜 | ⚠️ | Evaluating | `.amp/settings.json` | ✅ |
| **OpenHands** | 🔜 | 🔜 | Evaluating | `config.toml`, `.openhands/` | ✅ |
| **Junie** | 🔜 | ⚠️ | Evaluating | `.junie/` | ✅ |
| **Kiro** | 🔜 | ⚠️ | Evaluating | `.kiro/`, steering files | ✅ |
| **Mistral Vibe** | 🔜 | 🔜 | Evaluating | `.vibe/config.toml` | ✅ |
| **Aider** | 🔜 | 🔜 | Planned | `~/.aider.conf.yml` | — |
| **OpenCode** | 🔜 | 🔜 | Evaluating | Filesystem config | ✅ |

✅ shipped · 🔜 planned/evaluating · ⚠️ partial/uncertain

## Shipped Agents

### Claude Code

Anthropic's CLI coding agent — the default and most mature adapter.

| | |
|---|---|
| Config | `~/.claude/`, `CLAUDE.md`, `settings.json` |
| Auth | Vertex AI or direct Anthropic API |
| Skills | `.claude/skills/{name}/SKILL.md` |

### Cursor Agent CLI

Cursor's headless coding agent (separate from the desktop IDE).

| | |
|---|---|
| Config | `~/.cursor/`, `.cursor/rules/*.mdc` |
| Auth | Cursor API key |
| Skills | `.cursor/skills/{name}/SKILL.md` |

### Theia IDE

Eclipse Foundation's web IDE with multi-provider AI support.

| | |
|---|---|
| Config | Standard VS Code extension model |
| Auth | API keys via env vars (Anthropic, OpenAI, Google, Ollama) |
| Access | Browser on port 3000 |

## Config Format Landscape

Agents use four primary config formats.
Keel adapters emit the right format for each target.

| Format | Agents |
|---|---|
| **JSON** | Claude Code, Cursor, Amp, Factory, nanobot, pi, Letta |
| **YAML** | Goose, TRAE Agent, fast-agent, Aider |
| **TOML** | Mistral Vibe, VT Code, OpenHands |
| **Markdown** | GitHub Copilot (instructions), all agents (SKILL.md) |

SKILL.md ([Agent Skills standard](https://agentskills.io/)) is the one universal format — nearly every agent in the matrix supports it.

## Adding a New Agent

Keel uses a plugin pattern for agent support.
Each agent needs a compiler adapter (Go code) and optionally a container adapter (shell scripts).

See [Adding an Adapter]({{< relref "../architecture/adapters" >}}) for the full guide.

## What's Next

- **[Compatibility Matrix]({{< relref "../reference/compatibility-matrix" >}})** — exhaustive cross-agent reference for every config file, setting, hook event, and frontmatter field.
- **[Comparisons]({{< relref "comparisons" >}})** — how Keel relates to other tools in this space.
- **[Transpilation]({{< relref "../guide/transpilation" >}})** — cross-format compilation and the portability matrix.
