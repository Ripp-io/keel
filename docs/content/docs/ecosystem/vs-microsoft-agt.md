---
title: "Keel + Microsoft Agent Governance Toolkit"
weight: 10
bookHidden: true
---

# Keel + Microsoft Agent Governance Toolkit

[Microsoft AGT](https://opensource.microsoft.com/blog/2026/04/02/introducing-the-agent-governance-toolkit-open-source-runtime-security-for-ai-agents/) (April 2026) is an open-source, MIT-licensed toolkit for runtime governance of AI agents.
It covers all 10 OWASP Agentic AI Top 10 risks with deterministic policy enforcement at sub-millisecond latency.
It's the most serious open-source alternative in the governance space — but operates at a completely different layer than Keel.

## Where each system sits

```
                Microsoft AGT                         Keel
                ─────────────                         ────
When            Runtime (intercepts agent actions)     Compile-time (produces config files)
What            Policy enforcement on live agents      Config composition and governance
How             OPA/Rego/Cedar policy engine            DSL with merge strategies + force-locks
Scope           Any AI agent framework                 AI coding agents specifically
Language        Python, TypeScript, Rust, Go, .NET     Go CLI
License         MIT                                    Open source
```

## The key difference: compile-time vs runtime

Keel and AGT solve governance at different points in the lifecycle:

```
                    ┌─────────────┐
Config layers ───►  │    Keel      │ ───► Native agent config
                    │  (compile)   │       (CLAUDE.md, settings.json)
                    └─────────────┘
                           │
                           ▼
                    ┌─────────────┐
Agent executes ───► │    AGT       │ ───► Governed runtime behavior
                    │  (runtime)   │       (policy checks per action)
                    └─────────────┘
```

Keel ensures the agent *starts* with the right configuration.
AGT ensures the agent *behaves* correctly while running.

## What each system does well

| Capability | Microsoft AGT | Keel |
|---|---|---|
| Runtime policy enforcement | ✅ Sub-ms deterministic interception | ❌ Not in scope |
| OWASP Agentic Top 10 coverage | ✅ 10/10 | ❌ Not in scope |
| Kill switch / circuit breakers | ✅ Agent Runtime package | ❌ Not in scope |
| Compliance auditing (SOC 2, HIPAA, SEC) | ✅ Agent Compliance package | ⚠️ Provenance manifests support auditability |
| Agent-to-agent security | ✅ Agent Mesh package | ❌ Not in scope |
| Multi-layer config composition | ❌ | ✅ 6 merge strategies |
| Per-key config governance | ❌ | ✅ Force-locks with provenance |
| Cross-agent config portability | ❌ | ✅ 17 agents shipped |
| Instruction templating | ❌ | ✅ Variables, conditionals, snippets |
| Config drift prevention | ❌ | ✅ Lockfile verification |

## When to use which

**Microsoft AGT** is the right choice if:
- You need runtime guardrails on agent behavior (tool misuse, goal hijacking, data exfiltration)
- You need compliance verification against regulatory frameworks
- You're building custom agents with LangChain, CrewAI, or other frameworks
- You need agent-to-agent communication security

**Keel** is the right choice if:
- You need to compose and govern agent *configuration* across teams
- You use AI coding agents (Claude, Cursor) and need cross-agent portability
- You need config-as-code with Git-based review workflows
- You need per-value governance that prevents downstream config override

## What each brings to the other

These tools are complementary — they cover different halves of the governance lifecycle.

Keel compiles the right configuration into the agent's native format.
AGT guards the running agent against policy violations.

A complete governance stack could use Keel for compile-time config governance and AGT for runtime behavioral enforcement.
OPA (which AGT uses internally) could also validate Keel's compiled output as a bridge between the two.

Sources:
- [Microsoft Open Source Blog: Introducing AGT](https://opensource.microsoft.com/blog/2026/04/02/introducing-the-agent-governance-toolkit-open-source-runtime-security-for-ai-agents/)
- [GitHub: microsoft/agent-governance-toolkit](https://github.com/microsoft/agent-governance-toolkit)
- [Architecture Deep Dive](https://techcommunity.microsoft.com/blog/linuxandopensourceblog/agent-governance-toolkit-architecture-deep-dive-policy-engines-trust-and-sre-for/4510105)

## What's Next

- **[All Comparisons]({{< relref "comparisons" >}})** — how Keel relates to every tool in this space.
- **[Governance]({{< relref "../guide/governance" >}})** — how Keel's force-lock system works.
- **[Getting Started]({{< relref "../getting-started" >}})** — try Keel in two minutes.
