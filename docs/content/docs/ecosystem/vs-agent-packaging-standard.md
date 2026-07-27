---
title: "Keel + Agent Packaging Standard (APS)"
weight: 10
bookHidden: true
---

# Keel + Agent Packaging Standard (APS)

Keel and [APS](https://agentpackaging.org/) ([repo](https://github.com/vedahegde60/agent-packaging-standard)) occupy adjacent, non-overlapping layers in the agent infrastructure stack.
APS standardizes how you package and distribute an agent as a callable service.
Keel governs how you configure and deliver an AI coding workspace.
They don't compete — they compose.

## Where each project sits

```
┌───────────────────────────────────────────────┐
│  Developer / CI Pipeline                      │
├───────────────────────────────────────────────┤
│  Keel: Workspace (repos, init scripts, tmux)  │  Tier 3
├───────────────────────────────────────────────┤
│  Keel: Container (agent + compiler, sandbox)  │  Tier 2
├───────────────────────────────────────────────┤
│  Keel: Config Compiler (layers, governance)   │  Tier 1
├───────────────────────────────────────────────┤
│  AI Coding Agent (Claude, Cursor, Theia)      │  The agent
├───────────────────────────────────────────────┤
│  APS: Agent tools/services (packaged, signed) │  Callable services
├───────────────────────────────────────────────┤
│  APS: Registry (publish, pull, search)        │  Distribution
├───────────────────────────────────────────────┤
│  APS: Package (.aps.tar.gz + manifest + sig)  │  Packaging standard
└───────────────────────────────────────────────┘
```

## What each project does

| Concern | APS | Keel |
|---|---|---|
| Unit of packaging | The agent itself (code + manifest + assets) | The agent's configuration (instructions + permissions + hooks + MCP servers) |
| What "run" means | Invoke an agent as a stdin/stdout function | Launch a governed developer workspace with the agent inside |
| Distribution | `.aps.tar.gz` via a registry API | Container images via OCI registries |
| Governance | `policies` in manifest — self-declared | Force-locks, trust-ordered layers — compiler-enforced |
| Cross-agent | N/A — agents conform to the APS format | Adapters compile one config source to Claude, Cursor, or Theia native formats |

## Integration opportunities

### 1. APS agents as governed MCP tools

APS already ships an [MCP wrapper](https://github.com/vedahegde60/agent-packaging-standard/blob/main/interop/mcp/aps_mcp_wrapper.py) that exposes APS agents as JSON-RPC tools.
Keel already manages MCP server configuration per workspace.
The connection: Keel config layers could declare APS agents as MCP tools, and the compiler would wire them into the agent's native MCP config at boot.

A security team publishes a compliance-checking APS agent to a registry.
A Keel config layer references it.
At container boot, the agent is pulled and exposed as an MCP tool — governed by the same force-lock policies as everything else.

### 2. APS policy enforcement via Keel

APS manifests declare policies (network egress, filesystem access, secrets).
These are metadata — nothing enforces them.
Keel's governance model could enforce them: translate APS policy declarations into container-level network rules, read-only mounts, and scoped secret injection.
APS says "I need X."
Keel enforces "you only get X."

### 3. APS packaging for Keel distributable layers

Keel's roadmap includes distributable config layers as registry artifacts.
APS's packaging conventions (`.tar.gz` with manifest, versioned, signed, pushed to a registry) solve the same mechanical problem.
Rather than inventing a format, Keel could adopt or extend APS packaging for config layer distribution.

### 4. APS signing for Keel layer trust

Keel's trust model is compile-time (layer order = trust level), but there's no cryptographic verification that a layer is authentic.
APS has Ed25519 signing implemented (`keygen` → `sign` → `verify`).
Keel could adopt this for layer provenance — verifying that the security policy layer you pulled is the one the security team published.

### 5. APS registry as Keel layer registry

APS ships a working FastAPI registry with publish/pull/search.
Keel needs a registry for distributable layers.
The APS registry API could be extended to handle config layers, or Keel could adopt the same API contract for its own registry.

## What each project brings to the other

| APS brings to Keel | Keel brings to APS |
|---|---|
| Package format for distributable config layers | Governance enforcement for self-declared policies |
| Ed25519 signing and verification | Multi-layer config composition with merge strategies |
| Working registry with publish/pull/search | Full workspace lifecycle (repos, init, tmux, CI) |
| Standard I/O contract for agent tools | Container sandbox with network/filesystem isolation |
| MCP/AGP/TDF ecosystem interop | Cross-agent portability (one config → many agents) |

## What's Next

- **[All Comparisons]({{< relref "comparisons" >}})** — how Keel relates to every tool in this space.
- **[Governance]({{< relref "../guide/governance" >}})** — how Keel's force-lock system works.
- **[Getting Started]({{< relref "../getting-started" >}})** — try Keel in two minutes.
