---
title: Agent Migration
weight: 8
---

# Agent Migration

Switch AI agents without rewriting your configuration.
When a team moves from Cursor to Claude Code (or the reverse), Keel handles the translation.

## The Migration Path

1. Import your existing native config into a Keel DSL layer
2. Compile for the new agent
3. Done — one command to switch targets

### Import existing config

```bash
keel import --from ./my-project --to ./my-layer
```

Keel auto-detects the source format and produces a `keel.yml` with companion files.
Instructions, permissions, hooks, MCP servers, commands, agents, and skills are all captured.

### Compile for the new agent

```bash
keel compile --layer ./my-layer --output claude=.
```

Or target both agents at once:

```bash
keel compile --layer ./my-layer --output claude=. --output cursor=.
```

## What Translates Cleanly

Instructions, MCP servers, portable hooks, and permissions translate losslessly between agents.
See [Transpilation]({{< relref "transpilation#what-translates" >}}) for the full portability matrix.

## What Doesn't

Some features are agent-specific and lose information during cross-compilation.
Keel warns about lossy translations at compile time — nothing is silently dropped.

Common gaps:

| Feature | Issue |
|---------|-------|
| Claude plugins | Dropped for Cursor (no equivalent) |
| Claude agent memory | Dropped for Cursor (no equivalent) |
| Cursor environment config | Dropped for Claude (no equivalent) |
| Permission `ask` tier | Dropped for Cursor (only allow/deny) |

Run `keel validate --lint --agent <target>` to find non-portable patterns before you migrate:

```bash
keel validate ./my-layer --lint --agent claude
```

## Gradual Migration

You don't have to switch all at once.
Compile for both agents and let team members move at their own pace:

```yaml
# .keel/config.yml
layers:
  - ./layers/*

output:
  claude: .
  cursor: .
```

Everyone runs `keel compile` and gets config for whichever agent they use.
Team members on the old agent keep working.
Team members on the new agent are already configured.

## Future-Proofing

Once your config is in Keel DSL, adding a new agent target is one line in your config file.
The investment in migration pays off every time the agent landscape shifts — you absorb the change in the compiler, not in your config.

## What's Next

- **[Transpilation]({{< relref "transpilation" >}})** — the full portability matrix and pipeline details.
- **[Layers]({{< relref "layers" >}})** — layer formats and auto-detection.
- **[CLI Reference]({{< relref "../reference/cli#keel-import" >}})** — `keel import` flags and examples.
