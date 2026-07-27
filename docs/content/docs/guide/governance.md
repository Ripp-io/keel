---
title: Governance
weight: 3
---

# Governance

Higher-trust layers lock values that downstream layers cannot override.
This is how a company enforces security policies that teams and individuals cannot weaken.

## Quick Examples

### Force-lock an MCP server

A company layer force-locks its security scanner — no downstream layer can remove or modify it:

```json
{
  "mcpServers": {
    "sonarqube": {
      "_keel": { "force": true },
      "command": "sonar-mcp",
      "args": ["--token", "${SONAR_TOKEN}"]
    }
  }
}
```

The `_keel` key is governance metadata — Keel strips it from the final output.
The agent never sees it.

### Additive permissions

A company layer grants base permissions with an additive merge policy:

```yaml
# 01-company/keel.yml
permissions:
  rules:
    - source: permissions.json
  policy:
    merge: additive
```

```json
{ "allow": ["Read", "Grep"] }
```

A project layer adds its own permissions:

```json
{ "allow": ["Bash(npm:*)", "Write"] }
```

Result: `["Read", "Grep", "Bash(npm:*)", "Write"]` — the union of both.
The project cannot remove the company's `Read` and `Grep`.

## The Trust Model

Layers compile in the order specified.
Earlier layers have higher trust.

```
Layer 1 (Company)   — highest trust, can lock anything
Layer 2 (Org)       — can lock, but cannot override Company locks
Layer 3 (Team)      — can lock, but cannot override Company or Org locks
Layer 4 (Project)   — can add, but cannot override locked values
Layer 5 (User)      — lowest trust, subject to all locks
```

Once a value is locked, it stays locked for all subsequent layers.

## Force-Lock Reference

Force-locks prevent downstream layers from overriding specific values.
Every lockable type uses the same concept — `force: true` — with syntax that fits the file format.

### In markdown (instruction files)

```markdown
---
force: true
---
# Security Standards

These standards cannot be overridden by downstream layers.
```

### In JSON (MCP, permissions, hooks)

```json
{
  "_keel": { "force": true },
  "command": "sonar-mcp"
}
```

### In hook entries

```json
{
  "PreToolUse": [
    {
      "type": "command",
      "command": "echo security-check",
      "force": true
    }
  ]
}
```

### In `keel.yml` (per-file and per-repo)

```yaml
files:
  "commands/security-scan.md":
    force: true
  "skills/compliance-check/":
    force: true

repos:
  critical-service:
    url: https://gitlab.example.com/org/critical.git
    path: services/critical
    branch: main
    force: true
```

## Merge Policies

Merge policies control how values combine across layers, independent of force-locks.

### Permissions

```yaml
permissions:
  rules:
    - source: permissions.json
  policy:
    merge: additive
```

| Policy | Behavior |
|--------|----------|
| `additive` | Allow/deny arrays are unioned — downstream can only add, not remove |
| `replace` | Last-writer-wins — each layer replaces the previous entirely (default) |

### Per-file merge strategy

Control how specific files merge across layers:

```yaml
files:
  "permissions.json":
    merge: additive
  "settings.json":
    merge: deep-merge
```

Valid strategies: `deep-merge`, `replace`, `additive`.

## MCP Governance

Control which MCP servers are allowed or denied across the layer stack:

```yaml
mcp:
  servers:
    - source: mcp.json
  policy:
    allowed: [sonarqube, gitlab]
    denied: [unsafe-server]
    enableAll: true
```

The `allowed` and `denied` lists accumulate across layers.
A server on the `denied` list is blocked regardless of which layer defined it.
Individual servers can be force-locked (see [Quick Examples](#quick-examples) above).

## Enterprise Policy Distribution

Central security or platform teams publish governed layers that downstream teams consume via remote layer references.
This is the "managed settings" model — except it works for every agent, not just one.

### The pattern

1. A central team maintains a governance layer in a Git repo
2. The layer force-locks security policies, mandatory MCP servers, and compliance hooks
3. Downstream teams reference it as a remote layer in their `.keel/config.yml`

```yaml
# Downstream team's .keel/config.yml
layers:
  - 'https://gitlab.example.com/security/keel-policy.git#main'
  - ./layers/team
  - ./layers/project

output:
  claude: .
  cursor: .
```

The security team's force-locks carry through — downstream teams add their own config on top but cannot weaken the policies.
Updates to the central repo propagate on next `keel compile`.

### What the central layer controls

A governance layer typically force-locks:

- **Security hooks** — pre-tool-use blockers for sensitive directories
- **Mandatory MCP servers** — security scanners, compliance tools
- **Permission baselines** — additive policy so teams add permissions but never remove the base set
- **Commands** — required slash commands like `security-scan`
- **MCP deny lists** — blocking known-unsafe servers org-wide

### Distributable layers for supply chains

Normally, Keel strips all governance metadata (`_keel` keys, frontmatter `force` fields) from compiled output.
The agent sees clean, native config.

To preserve governance through a compilation chain, use `--distributable`:

```bash
keel compile --layer ./company --output claude=./distributable --distributable "company-standards"
```

The output can be used as a layer in downstream compilations, carrying its force-locks forward.
This is how governance propagates through multi-stage compilation pipelines — a company distributable feeds into an org layer, which feeds into a team layer.

See [Packaging]({{< relref "packaging" >}}) for details.

### Compared to vendor-specific governance

| Approach | Scope | Limitation |
|----------|-------|------------|
| Claude Managed Settings | Claude Code only | Binary tier override, no per-key granularity |
| KiloClaw for Organizations | KiloClaw platform only | SaaS-dependent, no local compilation |
| Keel remote governance layers | Any supported agent | Self-hosted, per-key force-locks, cross-agent |

## Config Supply Chain Security

Agent configuration files are an attack vector.
A compromised config layer can inject malicious instructions — prompt injection via the supply chain rather than the chat window.

### The threat

[CVE-2025-53773](https://www.cve.org/CVERecord?id=CVE-2025-53773) demonstrated that agent config files can carry prompt injection payloads.
When agents load instructions from config, a tampered instruction file can override safety behaviors.

### How Keel mitigates this

**Lockfile verification** — the `.keel.json` manifest records SHA-256 hashes of every layer file used during compilation.
Use `--locked` in CI to verify that layers haven't changed since the last known-good compile:

```bash
keel compile --locked
```

If any layer file has changed, the build fails.
This catches both accidental drift and malicious tampering.

**Provenance tracking** — the manifest records which layer contributed each value.
Run `keel explain` to audit exactly what instructions the agent received and who set them:

```bash
keel explain locks
keel explain files
```

**Force-locks** — governance layers can lock security-critical values so downstream layers (and potential attackers) cannot override them.
A force-locked security hook runs regardless of what other layers say.

**MCP deny lists** — block specific MCP servers by name across the entire layer stack.
A server on the deny list is blocked even if a compromised downstream layer adds it.

### Cryptographic signing

Lockfile verification checks that layers haven't changed since the last compile.
Signing goes further — it proves *who* produced the layers and *who* compiled the output.

Sign your layers:

```bash
keel key generate
keel sign ./layers/company-policy
```

Compile with signature verification:

```bash
keel compile --sign --verify-layers
```

The compiled manifest includes the signature chain — which key signed each layer and who signed the manifest.
Run `keel verify` or `keel drift --verify` to check signatures at any time.

See [Signing]({{< relref "signing" >}}) for the full guide.

### CI pipeline verification

Pin your config to a known-good state and verify on every build:

```bash
# First compile — generates .keel.json with content hashes
keel compile --layer ./my-layers --output claude=.

# Commit .keel.json alongside your code

# CI pipeline — verify nothing changed
keel compile --locked
```

This is the same model as `go.sum` or `package-lock.json` — verify dependencies haven't been tampered with.

## Inspecting Locks

The compiled manifest (`.keel.json`) records all force-locks with provenance:

```yaml
force_locked:
  - category: mcp
    key: sonarqube
    locked_by: /layers/01-company
  - category: hooks
    key: PreToolUse[0]
    locked_by: /layers/01-company
```

Use `keel explain` to see all force-locked items and which layer set them.

## What's Next

- **[Signing]({{< relref "signing" >}})** — Ed25519 cryptographic signing for layers and manifests.
- **[Layers]({{< relref "layers" >}})** — layer ordering and merge behavior.
- **[DSL Reference]({{< relref "dsl" >}})** — the `_keel` fields and `files` map.
- **[Packaging]({{< relref "packaging" >}})** — distributable layers, archives, and lockfile verification.
- **[Multi-Repo Workspaces]({{< relref "multi-repo-workspaces" >}})** — consistent governance across a workspace of repos.
- **[Enterprise Example]({{< relref "enterprise-example" >}})** — 5-layer enterprise setup showing all governance features in context.
