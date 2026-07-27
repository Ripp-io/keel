---
name: keel-layer
description: Create a new Keel configuration layer with keel.yml and companion files. Use when building company policy, team config, project specifics, or user preference layers.
---

# Create a Keel Layer

Create a configuration layer for the Keel compiler.
Layers are directories with a `keel.yml` manifest and companion files that Keel merges together during compilation.

## Step 1: Determine purpose and trust level

| Purpose | Trust | Governance | Examples |
|---------|-------|------------|----------|
| Company security | Highest | Heavy force-locks | Required MCP servers, blocked permissions, mandatory hooks |
| Org standards | High | Moderate force-locks | Shared instructions, standard tooling, model settings |
| Team conventions | Medium | Light force-locks | Team skills, coding guidelines, project context |
| Project specifics | Low | Minimal | Project instructions, repo-specific skills, local MCP |
| User preferences | Lowest | None | Personal settings, editor preferences, shortcuts |

Trust level determines layer order: higher trust layers appear earlier and can force-lock values.

## Step 2: Create the directory structure

```
my-layer/
├── keel.yml                  # Layer manifest (required)
├── instructions.md           # Agent instructions
├── permissions.json          # Permission allow/deny lists
├── mcp.json                  # MCP server configuration
├── hooks.json                # Hook definitions
├── settings.claude.json      # Claude-specific settings passthrough
├── settings.cursor.json      # Cursor-specific settings passthrough
├── commands/                 # Slash command files
│   └── my-command.md
├── agents/                   # Sub-agent definitions
│   └── my-agent.md
├── skills/                   # Skill directories
│   └── my-skill/
│       └── SKILL.md
└── snippets/                 # Reusable content blocks
    └── my-snippet.md
```

Include only the sections this layer needs.

## Step 3: Write keel.yml

Start minimal and add sections as needed:

```yaml
schema: v1
name: my-layer
description: What this layer provides

instructions:
  - source: instructions.md

# Add sections as needed — see the DSL reference in the instructions context
```

### Key patterns

**FileRef with governance:**
```yaml
mcp:
  servers:
    - source: mcp.json
      force: true         # Downstream layers cannot override
```

**Directory collection:**
```yaml
skills:
  - source: skills/          # All skill directories collected
  - source: skills/critical/
    force: true             # This specific skill is locked
```

**Agent-specific settings:**
```yaml
settings:
  model: claude-sonnet-4-20250514
  claude: settings.claude.json    # Passthrough for Claude-only keys
  cursor: settings.cursor.json    # Passthrough for Cursor-only keys
```

## Step 4: Write companion files

### permissions.json
```json
{
  "allow": ["Read", "Write", "Bash(git *)"],
  "deny": ["Bash(rm -rf /)"]
}
```

### mcp.json
```json
{
  "mcpServers": {
    "my-server": {
      "command": "npx",
      "args": ["-y", "@org/mcp-server"],
      "env": { "API_KEY": "${MY_API_KEY}" }
    }
  }
}
```

Use `${VAR}` for secrets — they pass through to runtime, never baked into output.

### hooks.json
```json
{
  "PreToolUse": [
    {
      "matcher": "Bash",
      "hooks": [
        { "type": "command", "command": "scripts/validate.sh" }
      ]
    }
  ]
}
```

Hook event names use **PascalCase** in source.
Keel converts to the agent's native convention automatically.

### instructions.md

Write as if these are the only instructions the agent will see.
Keel handles merging across layers.

Use variables (`${keel:var_name}`), conditionals (`<!-- keel-if -->`), and snippet includes (`<!-- keel:include name -->`) for dynamic content.

## Step 5: Validate

```bash
keel validate /path/to/my-layer
```

Fix errors (blocking). Review warnings (non-blocking).

## Step 6: Test compilation

```bash
# Preview
keel compile --dry-run --layer /path/to/my-layer --output claude=.

# Compile and inspect
keel compile --layer /path/to/my-layer --output claude=/tmp/test --summary

# Explain provenance
keel explain /tmp/test
```

Verify: instructions merged correctly, settings deep-merged, force-locks held, governance metadata stripped from output.

## Step 7: Test in a layer stack

```bash
keel compile \
  --layer /path/to/higher-trust \
  --layer /path/to/my-layer \
  --output claude=/tmp/test --summary

keel explain /tmp/test
```

Verify: higher-trust force-locks preserved, your layer's values appear where expected, merge strategies applied correctly.
