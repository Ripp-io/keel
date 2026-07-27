---
title: Multi-Repo Workspaces
weight: 15
---

# Multi-Repo Workspaces

Compile consistent AI agent config across a workspace of repositories.
Same governance, same MCP servers, same base rules — with per-repo overrides where needed.

## The Problem

A project that spans multiple repos needs the agent to understand the full workspace.
Without Keel, you manually configure each repo's agent setup, duplicating instructions, MCP servers, and permissions.
Config drifts between repos.
New repos start with a blank slate.

## The Pattern

Use the `repos:` field in your layer to declare the workspace.
Use shared layers for governance and base config.
Use per-repo layers for repo-specific overrides.

### Shared workspace layer

```yaml
# layers/workspace/keel.yml
schema: v1
name: platform-workspace
description: Shared config across all platform repos

instructions:
  - source: instructions.md

permissions:
  rules:
    - source: permissions.json
  policy:
    merge: additive

mcp:
  servers:
    - source: mcp.json

repos:
  api-gateway:
    url: https://gitlab.example.com/platform/api-gateway.git
    path: api-gateway
    branch: main
    required: true
  user-service:
    url: https://gitlab.example.com/platform/user-service.git
    path: user-service
    branch: main
  shared-lib:
    url: https://gitlab.example.com/platform/shared-lib.git
    path: libs/shared-lib
    branch: main
```

### Per-repo overrides

A second layer adds repo-specific config without duplicating the shared setup:

```yaml
# layers/api-gateway-overrides/keel.yml
schema: v1
name: api-gateway-config
description: API gateway-specific instructions and tools

instructions:
  - source: instructions.md

commands:
  - source: commands/
```

```markdown
<!-- layers/api-gateway-overrides/instructions.md -->
# API Gateway

This service handles routing, authentication, and rate limiting.
No business logic belongs here — delegate to downstream services.
```

### Compile

```yaml
# .keel/config.yml
layers:
  - ./layers/workspace
  - ./layers/api-gateway-overrides

output:
  claude: .
  cursor: .
```

```bash
keel compile
```

The compiled output includes the full repo map, shared instructions, shared MCP servers, and the gateway-specific additions.

## Repos Merge by Name

When multiple layers define repos, they merge by name using deep-merge.
This lets a workspace layer declare the base repo list and a project layer add or extend it:

```yaml
# Workspace layer
repos:
  api-gateway:
    url: https://gitlab.example.com/platform/api-gateway.git
    path: api-gateway
    branch: main

# Project layer — adds a repo, extends an existing one
repos:
  api-gateway:
    required: true    # adds required flag to existing definition
  frontend:
    url: https://gitlab.example.com/platform/frontend.git
    path: frontend
    branch: main
```

Result: three repos total, with `api-gateway` carrying both the workspace URL and the project's `required: true`.

## Governance Across Repos

Force-lock repo definitions at the workspace level so downstream layers can't remove or redirect them:

```yaml
repos:
  critical-service:
    url: https://gitlab.example.com/platform/critical-service.git
    path: services/critical
    branch: main
    force: true
```

A team layer cannot change the URL, path, or branch of a force-locked repo.
It can still add new repos — force-locks prevent modification, not addition.

## CI: Consistent Config Across Repos

In a CI pipeline, each repo compiles from the same shared layers:

```bash
# In each repo's CI job
keel compile \
  --layer 'https://gitlab.example.com/platform/keel-config.git#main' \
  --layer . \
  --output claude=.
```

The shared layer is a remote Git reference.
Each repo adds itself as the final layer for project-specific overrides.
Every repo gets the same base config, the same governance, and repo-specific additions.

## What's Next

- **[DSL Reference: Repos]({{< relref "dsl#repos" >}})** — every field in the repos definition.
- **[Layers]({{< relref "layers#remote-layers" >}})** — remote layer URL format, caching, and offline mode.
- **[Governance]({{< relref "governance" >}})** — force-locking repos and other values.
- **[Enterprise Example]({{< relref "enterprise-example" >}})** — 5-layer setup with repos from multiple layers.
