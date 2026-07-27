---
title: "Keel + DevContainers"
weight: 10
bookHidden: true
---

# Keel + DevContainers

[DevContainers](https://containers.dev/) (the `.devcontainer/` spec) provide containerized development environments with filesystem isolation, reproducibility, and security hardening. They're widely used for [sandboxing AI coding agents](https://dev.to/siddhantkcode/isolating-ai-agents-with-devcontainer-a-secure-and-scalable-approach-4hi4). Keel's container tiers solve a similar delivery problem — but add configuration compilation and governance that DevContainers don't have.

## Where each system sits

```
┌───────────────────────────────────────────────┐
│  IDE connection (VS Code, Cursor remote)      │  DevContainers
├───────────────────────────────────────────────┤
│  Container sandbox (isolation, credentials)   │  Both
├───────────────────────────────────────────────┤
│  Agent + tooling installation                 │  Both
├───────────────────────────────────────────────┤
│  Config compilation (layers, merge, locks)    │  Keel only
├───────────────────────────────────────────────┤
│  Config governance (force-locks, trust)        │  Keel only
└───────────────────────────────────────────────┘
```

DevContainers define the sandbox. Keel fills the sandbox with governed configuration.

## What each system does

| Concern | DevContainers | Keel Containers |
|---|---|---|
| Primary purpose | Reproducible dev environments for any language/toolchain | Governed AI agent workspaces specifically |
| Config governance | None — Dockerfile and devcontainer.json are user-controlled | Multi-layer compilation with force-locks and merge strategies |
| Agent awareness | None — agents are just processes in the container | Adapters for Claude, Cursor, Theia with lifecycle management |
| IDE integration | First-class VS Code / Cursor remote attach | Compatible as devcontainer backend; also standalone with tmux |
| Credential management | `remoteEnv`, Docker secrets, bind-mount | Runtime env passthrough (`${VAR}`), never baked into images |
| Filesystem isolation | Volume mounts, `cap_drop`, `security_opt` | Same container-level controls, plus agent-scoped config |
| Workspace init | `postCreateCommand`, `postStartCommand` | Ordered `init.d/` scripts with idempotency, repo cloning from manifest |
| Headless / CI | Possible but not first-class | Purpose-built runner images with prompt input, budget caps, structured output |
| Cross-agent | N/A — agent-unaware | One config source compiles to any supported agent |

## Where DevContainers are sufficient

For teams that need containerized development without agent-specific governance:

- Standard dev environments (any language, any toolchain)
- IDE remote-attach workflow (VS Code, Cursor connecting to a container)
- Basic AI agent sandboxing (mount project, run agent, blast radius = container)
- Teams already invested in `.devcontainer/` across their repos

DevContainers are the standard for containerized development. If your need is "run Claude Code in a sandbox" without organizational policy, a DevContainer does the job.

## Where Keel adds value

### Configuration governance

A DevContainer gives the agent a sandbox. It doesn't control *what the agent sees inside the sandbox*. Anyone can modify the CLAUDE.md, install any MCP server, or change permissions.

Keel compiles governed configuration into the container at boot. Force-locked values, additive-only permissions, and trust-ordered instruction composition happen before the agent starts. The sandbox has structure, not just walls.

### Agent lifecycle management

DevContainers run a `postCreateCommand` and start a shell. Setting up an AI coding agent — configuring authentication backends (API key vs. Vertex AI vs. OAuth), bypassing first-run flows, launching in tmux — is left to the user.

Keel's adapter interface handles this per agent: `adapter_configure_backend`, `adapter_launch`, `adapter_write_onboarding`. Adding a new agent is one adapter, not a rewrite of the DevContainer.

### Multi-layer config from organizational sources

A DevContainer's configuration is a single `devcontainer.json` per repo. Organizational policy, team conventions, and project specifics must be manually coordinated into that one file.

Keel compiles from N layers (company → org → team → project → user) with deterministic merge strategies. Each layer is a separate git repo or directory owned by the responsible team.

### CI runner mode

DevContainers can run headlessly, but they're not designed for "take a prompt, run the agent, return structured output, exit." Keel's runner images are purpose-built for this: configurable budget limits, tool allowlists, turn caps, and pipeline-consumable output.

## How they work together

Keel containers are compatible as devcontainer backends. The integration:

1. **Keel builds the container image** with the agent, compiler, and base config baked in
2. **A `.devcontainer/devcontainer.json`** references the Keel image
3. **Developers attach VS Code or Cursor** via remote-containers to the running Keel workspace
4. **Config is compiled at boot** from mounted layers — developers get a governed environment through their familiar IDE

This gives teams the DevContainer workflow they know with Keel's governance underneath. The developer connects their IDE to a container. The container has governed config compiled in. The developer never runs `keel compile` — they just see their agent with the right instructions, permissions, and tools.

## What each system brings to the other

| DevContainers bring to Keel | Keel brings to DevContainers |
|---|---|
| IDE remote-attach workflow (VS Code, Cursor) | Configuration governance for what's *inside* the container |
| Ecosystem adoption and tooling support | Agent-aware lifecycle management (auth, launch, onboarding) |
| Standard `.devcontainer/` spec that teams already use | Multi-layer config composition from organizational sources |
| Features API (install tools, languages, runtimes declaratively) | Cross-agent portability — same container governs Claude or Cursor |
| GitHub Codespaces / cloud-hosted dev environments | CI runner mode for headless pipeline execution |

## What's Next

- **[All Comparisons]({{< relref "comparisons" >}})** — how Keel relates to every tool in this space.
- **[Governance]({{< relref "../guide/governance" >}})** — how Keel's force-lock system works.
- **[Getting Started]({{< relref "../getting-started" >}})** — try Keel in two minutes.
