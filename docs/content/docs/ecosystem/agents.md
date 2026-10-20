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
| **Antigravity CLI** | ✅ | ✅ CLI | Shipped | `~/.gemini/`, `GEMINI.md`, `mcp_config.json` | ✅ |
| **Goose** | ✅ | ✅ CLI | Shipped | `~/.config/goose/`, `.goosehints` | ✅ |
| **Codex CLI** | ✅ | ✅ CLI | Shipped | `AGENTS.md`, `~/.codex/config.toml` | ✅ |
| **Kilo Code** | ✅ | ✅ CLI | Shipped | `.kilo/kilo.jsonc`, `AGENTS.md` | ✅ |
| **Qwen Code** | ✅ | ✅ CLI | Shipped | `~/.qwen/`, `QWEN.md` | ✅ |
| **Cline** | ✅ | ✅ CLI | Shipped | `.clinerules` | ✅ |
| **Aider** | ✅ | ✅ CLI | Shipped | `CONVENTIONS.md` | — |
| **GitHub Copilot** | ✅ | ⚠️ | Shipped | `.github/copilot-instructions.md` | ✅ |
| **OpenCode** | ✅ | ✅ CLI/Dev/Runner | Shipped | `AGENTS.md`, `.opencode/opencode.json` | ✅ |
| **Kiro** | ✅ | ✅ CLI | Shipped | `.kiro/`, `AGENTS.md`, JSON agents | ✅ |
| **Amp** | ✅ | ⚠️ | Shipped | `.amp/settings.json` | ✅ |
| **OpenHands** | ✅ | — | Shipped | `config.toml`, `.openhands/` | ✅ |
| **Junie** | ✅ | ⚠️ | Shipped | `.junie/` | ✅ |
| **Mistral Vibe** | ✅ | ✅ CLI | Shipped | `.vibe/config.toml` | ✅ |
| **Agent Plugins** | ✅ | — | Shipped | `plugin.json`, `skills/`, `mcp.json` (v1.0 standard) | ✅ |

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

### OpenCode

Open-source AI coding agent with instruction and settings support.

| | |
|---|---|
| Config | `AGENTS.md`, `.opencode/opencode.json` |
| Auth | Multi-provider (Anthropic, OpenAI, Copilot, Vertex, local) |
| Features | Embedded MCP and permissions in settings JSON |
| Skills | Supports Agent Skills standard |

### Cline

VS Code extension for AI coding with instruction-based configuration.

| | |
|---|---|
| Config | `.clinerules` (instruction-only) |
| Features | Single instruction file in project root |
| Skills | Supports Agent Skills standard |

### Aider

Terminal-based AI pair programming tool.

| | |
|---|---|
| Config | `CONVENTIONS.md` (instruction-only) |
| Features | Single instruction file in project root |
| Skills | Instruction-only (no skill directories) |

### GitHub Copilot

GitHub's AI code completion and chat assistant.

| | |
|---|---|
| Config | `.github/copilot-instructions.md` (instruction-only) |
| Features | Single instruction file per repository |
| Skills | Supports Agent Skills standard |

### Antigravity CLI

Google's CLI coding agent (replaced Gemini CLI, June 2026).

| | |
|---|---|
| Config | `~/.gemini/`, `GEMINI.md`, `mcp_config.json` |
| Auth | Google API key or Vertex AI |
| Skills | `.agents/skills/{name}/SKILL.md` |

### Goose

Block's open-source AI coding agent.

| | |
|---|---|
| Config | `~/.config/goose/config.yaml`, `.goosehints` |
| Auth | Multi-provider (Anthropic, OpenAI, Google, Azure, Bedrock) |
| Skills | Supports Agent Skills standard |

### Kilo Code

VS Code extension and CLI, forked from Roo Code.

| | |
|---|---|
| Config | `.kilo/kilo.jsonc`, `AGENTS.md` |
| Auth | Multi-provider |
| Skills | `.kilo/skills/{name}/SKILL.md` |

### Codex CLI

OpenAI's CLI coding agent.

| | |
|---|---|
| Config | `~/.codex/config.toml`, `AGENTS.md` |
| Auth | OpenAI API key |
| Skills | Supports Agent Skills standard |

### Qwen Code

Alibaba's multi-provider CLI agent (Gemini CLI fork).

| | |
|---|---|
| Config | `~/.qwen/settings.json`, `QWEN.md` |
| Auth | Multi-provider (DashScope, OpenAI, Anthropic, Gemini) |
| Skills | Supports Agent Skills standard |

### Kiro

AWS CLI coding agent (Q Developer CLI successor).

| | |
|---|---|
| Config | `.kiro/`, `AGENTS.md`, JSON agents |
| Auth | Kiro API key or AWS credentials |
| Skills | `.kiro/skills/{name}/SKILL.md` |

### Amp

Sourcegraph's AI coding agent.

| | |
|---|---|
| Config | `.amp/settings.json`, `AGENTS.md` |
| Auth | Sourcegraph API |
| Skills | Supports Agent Skills standard |

### OpenHands

Open-source AI agent platform (compiler only — server-based, no CLI container).

| | |
|---|---|
| Config | `config.toml`, `.openhands/` |
| Auth | Multi-provider |
| Features | Web-based agent platform with Docker sandbox |

### Junie

JetBrains AI coding agent (IDE-coupled, compiler only).

| | |
|---|---|
| Config | `.junie/guidelines.md`, `AGENTS.md` |
| Features | Instruction-only (IDE-coupled) |
| Skills | `.junie/skills/{name}/SKILL.md` |

### Mistral Vibe

Mistral's AI coding agent.

| | |
|---|---|
| Config | `.vibe/config.toml`, `AGENTS.md` |
| Auth | Mistral API key |
| Skills | Supports Agent Skills standard |

### Agent Plugins 1.0

Cross-agent plugin packaging standard ([agent-plugins.org](https://agent-plugins.org)).
Backed by Vercel, AWS, Microsoft, OpenAI, Google.

| | |
|---|---|
| Config | `plugin.json`, `skills/`, `mcp.json` |
| Consumers | ChatGPT, Codex, Cursor, Copilot, Kiro, VS Code |
| Skills | `skills/{name}/SKILL.md` (Agent Skills standard) |
| Provenance | Keel adds `io.ripp.keel` extension with compilation metadata |

## Config Format Landscape

Agents use four primary config formats.
Keel adapters emit the right format for each target.

| Format | Agents |
|---|---|
| **JSON** | Claude Code, Cursor, OpenCode, Antigravity CLI, Kiro, Amp, Qwen Code |
| **JSONC** | Kilo Code |
| **YAML** | Goose, TRAE Agent, fast-agent, Aider |
| **TOML** | Codex CLI, Mistral Vibe, VT Code, OpenHands |
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
