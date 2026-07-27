---
title: "Keel + Open Policy Agent (OPA)"
weight: 10
bookHidden: true
---

# Keel + Open Policy Agent (OPA)

[OPA](https://www.openpolicyagent.org/) ([repo](https://github.com/open-policy-agent/opa)) is a CNCF-graduated general-purpose policy engine.
Keel is a configuration compiler with a built-in governance model.
Both deal with "policy" — but they mean different things by it, operate at different times, and enforce at different layers.

## Where each system sits

```
┌───────────────────────────────────────────────┐
│  Runtime decisions (API auth, K8s admission)  │  OPA evaluates
├───────────────────────────────────────────────┤
│  Container execution (network, filesystem)    │  OPA can gate
├───────────────────────────────────────────────┤
│  AI agent session (Claude, Cursor, Theia)     │  Keel configures
├───────────────────────────────────────────────┤
│  Config compilation (layers, merge, locks)    │  Keel enforces
├───────────────────────────────────────────────┤
│  Config authoring (layers in git repos)       │  Keel governs
└───────────────────────────────────────────────┘
```

OPA operates at runtime — it answers "is this request allowed right now?"
Keel operates at build time — it answers "what configuration should this agent see, and which parts are immutable?"

## What each system does

| Concern | OPA | Keel |
|---|---|---|
| When it runs | Runtime — queries evaluated per-request | Build time — config compiled at container boot |
| Policy language | Rego (declarative, Datalog-inspired) | `_keel` metadata in JSON/YAML + frontmatter in Markdown |
| Policy scope | Anything expressible as structured data (API auth, K8s admission, Terraform plans, network rules) | AI agent configuration specifically (instructions, permissions, hooks, MCP servers, model selection) |
| Enforcement model | Decision engine — returns allow/deny/structured output; caller enforces | Compiler — force-locked values are physically absent from downstream layers; nothing to bypass |
| Trust model | External — policies loaded from bundles, verified by signature | Structural — layer order is trust order; earlier layers (company, org) outrank later layers (project, user) |
| Distribution | Bundles (`.tar.gz` with `.manifest` + `.signatures.json`) pulled from HTTP/S3/GCS/OCI | Git repos, mounted volumes, or baked into container images |
| State | Stateless per-query, but maintains policy + data cache | Stateless — pure compilation, no runtime daemon |

## Why not just use OPA for everything

The short answer: OPA evaluates policy at runtime against structured input.
Keel's governance problem is compile-time configuration composition — and the enforcement mechanism is elimination, not evaluation.

### OPA doesn't compose configuration

Keel's core job is merging 5+ config layers from different trust levels into a single agent environment.
This involves:

- **Concatenating** Markdown instructions with section headers (every layer contributes)
- **Deep-merging** JSON settings (later layers override specific keys)
- **Overlaying** directories of skill files (all layers contribute files, last writer wins per filename)
- **Additive union** of permission arrays (org sets baseline, teams extend, nobody removes)
- **Force-locking** values marked immutable by higher-trust layers

OPA has no concept of "merge strategy" or "layer precedence."
It evaluates a query against data and returns a decision.
You could write Rego policies that *validate* a merged config, but you can't use OPA to *produce* the merge.

### OPA's enforcement is advisory; Keel's is structural

OPA returns `{"allow": false}` — the calling system must act on it.
If the caller ignores the decision, the policy is bypassed.

Keel's force-lock enforcement works differently.
When a higher-trust layer force-locks an MCP server:

```json
{
  "mcpServers": {
    "security-scanner": {
      "_keel": { "force": true },
      "command": "security-scan",
      "args": ["--strict"]
    }
  }
}
```

The compiler strips `_keel` from output and registers the lock in its `forceRegistry`.
When a downstream layer tries to override `security-scanner`, the entry is silently removed before merge.
The agent never sees a conflict — the locked value is the only value that exists in the compiled output.
There is no runtime check to bypass because the alternative value was never written.

### OPA doesn't know about AI agent config formats

OPA is domain-agnostic — it works with any structured data.
But it has no concept of:

- What a CLAUDE.md file is or how instructions should be composed
- How `.cursor/rules` files map to Cursor's behavior
- That MCP server configs in `settings.json` vs `mcp.json` have different schemas
- That agent hooks need specific event names and command formats

Keel's adapters handle all of this.
The compiler knows how to emit Claude-native, Cursor-native, or Theia-native output from a single source.
OPA would need a separate integration for each agent format — and even then, it could only validate, not produce.

## Where OPA complements Keel

Despite the above, OPA fills real gaps that Keel doesn't cover:

### 1. Runtime policy for container execution

Keel compiles config and launches the agent.
After boot, it has no runtime policy engine.
OPA could enforce rules during the agent session:

- Gate outbound network requests from the container
- Enforce file-system access controls beyond read-only mounts
- Rate-limit API calls or token consumption per session
- Evaluate whether a specific MCP tool invocation is permitted given the current context

Keel decides what tools the agent *can see*.
OPA could decide what the agent *can do with them* at runtime.

### 2. Validation of compiled output

Keel compiles configuration deterministically, but doesn't validate the *result* against organizational policy.
OPA could run as a post-compilation gate:

```bash
keel compile --dry-run | opa eval -d org-policy.rego -i - "data.keel.violations"
```

Example Rego policy validating compiled Keel output:

```rego
package keel

violations contains msg if {
    some server in input.mcpServers
    not startswith(input.mcpServers[server].command, "/approved/")
    msg := sprintf("MCP server %q uses unapproved command path", [server])
}

violations contains msg if {
    input.settings.model != "claude-sonnet-4-20250514"
    msg := sprintf("Model %q is not on the approved list", [input.settings.model])
}
```

Keel ensures the merge is correct.
OPA ensures the result is compliant.

### 3. CI runner authorization

Keel's CI runner executes agents headlessly with a prompt.
OPA could gate which prompts are allowed, which repos the runner can access, and what budget ceiling applies — decisions that depend on runtime context (who triggered the pipeline, what branch, what time) that the compiler doesn't have.

### 4. Bundle distribution for Keel's future layer registry

OPA's bundle format (signed `.tar.gz` with manifest and revision tracking, pulled from HTTP/S3/GCS/OCI) is a mature, production-tested distribution mechanism.
When Keel builds its distributable layer package system, OPA's bundle infrastructure — signing, caching, delta updates, long-polling — is a proven model to follow or adopt directly.

## What each system brings to the other

| OPA brings to Keel | Keel brings to OPA |
|---|---|
| Runtime policy evaluation after compilation | A concrete domain (AI agent config) where policy matters |
| Post-compilation compliance validation | Compile-time enforcement that eliminates entire categories of runtime checks |
| Mature bundle signing and distribution | Layer composition with merge strategies — something OPA bundles don't support |
| Rego — a real policy language for complex rules | Structural enforcement (force-locks) that can't be bypassed by ignoring a decision |
| CNCF ecosystem integration (K8s, Envoy, Terraform) | Cross-agent portability — one policy source, multiple agent targets |

## Summary

OPA and Keel are not alternatives — they operate at different times on different problems.
Keel handles compile-time configuration governance: which layers merge how, what's immutable, and what the agent sees.
OPA handles runtime policy evaluation: is this action allowed given the current context.
The natural architecture uses both — Keel compiles the workspace, OPA validates the output and guards the session.

## What's Next

- **[All Comparisons]({{< relref "comparisons" >}})** — how Keel relates to every tool in this space.
- **[Governance]({{< relref "../guide/governance" >}})** — how Keel's force-lock system works.
- **[Getting Started]({{< relref "../getting-started" >}})** — try Keel in two minutes.
