---
title: Adding an Adapter
weight: 1
---

# Adding an Adapter

This guide walks through adding support for a new AI coding agent to Keel's compiler.

## Overview

Keel uses a plugin pattern for agent support.
Each agent has a compiler adapter (reader + writer + profile) that translates between the IR and the agent's native config format.
Adapters self-register at init time and share a common compilation pipeline.

## Architecture

```
                 ┌─────────────┐
                 │  Keel DSL   │
                 │  keel.yml   │
                 └──────┬──────┘
                        │
                        ▼
                 ┌─────────────┐
                 │   Resolve   │
                 │   Layers    │
                 └──────┬──────┘
                        │
                        ▼
                 ┌─────────────┐
                 │  Fold       │
                 │  (GATHER)   │
                 └──────┬──────┘
                        │
                        ▼
                 ┌─────────────┐
                 │  Governance │
                 │  + Transform│
                 │  + Finalize │
                 │  + Assemble │
                 └──────┬──────┘
                        │
              ┌─────────┼──────────┐
              ▼         ▼          ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
        │  Claude  │ │  Cursor  │ │ AGENTS.md│ │  New     │
        │  Writer  │ │  Writer  │ │  Writer  │ │  Writer  │
        └──────────┘ └──────────┘ └──────────┘ └──────────┘
```

## Step 1: Study Existing Adapters

Read these files to understand the pattern:

- `pkg/compiler/profile.go` -- the `AgentProfile` struct and `CompileWithProfile()` pipeline.
- `pkg/compiler/claude/claude.go` -- Claude adapter (profile, compiler registration).
- `pkg/compiler/claude/reader.go` -- Claude native reader.
- `pkg/compiler/claude/writer.go` -- Claude writer.
- `pkg/compiler/cursor/` -- Cursor equivalents.
- `pkg/compiler/compiler.go` -- `AdapterCompiler` interface and `RegisterCompiler()`.
- `pkg/compiler/ir.go` -- `LayerIR`, `AccumulatedResult`, `Writer` interface.

## Step 2: Create the Adapter Package

Create `pkg/compiler/myagent/`:

### myagent.go -- Profile and compiler registration

```go
package myagent

import (
    "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
    "gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
    compiler.RegisterCompiler("myagent", func() compiler.AdapterCompiler {
        return &Compiler{}
    })
}

type Compiler struct{}

func (c *Compiler) Name() string { return "myagent" }

func (c *Compiler) Compile(
    layers []config.Layer,
    outputDir string,
    opts compiler.CompileOptions,
) (*compiler.CompileResult, error) {
    return compiler.CompileWithProfile(ProjectProfile(), layers, outputDir, opts)
}

func ProjectProfile() compiler.AgentProfile {
    return compiler.AgentProfile{
        Name:            "myagent",
        OutputRoot:      ".myagent",
        InstructionOutput: "MYAGENT.md",
        SettingsFile:       "settings.json",
        SettingsLockKey:    "settings.json",
        MCPOutput:          ".myagent/mcp.json",
        HooksInSettings:    false, // true if hooks embed in settings file

        // Overlay categories this agent supports
        Categories: []compiler.CategoryDef{
            {DirName: "commands", LockCategory: "command"},
            {DirName: "agents", LockCategory: "agent"},
            {DirName: "skills", LockCategory: "skill"},
            {DirName: "rules", LockCategory: "rule"},
        },

        // Filenames to check when reading native layers (priority order)
        SettingsCandidates:    []string{"settings.json"},
        InstructionCandidates: []string{"MYAGENT.md", "myagent.md"},

        // Key transforms (identity if agent uses PascalCase hooks)
        HookKeyTransform:  func(s string) string { return s },
        ToolNameTransform: func(s string) string { return s },

        // Capabilities: which DSL features this agent supports
        Capabilities: compiler.Capabilities{
            Hooks:       true,
            MCP:         true,
            Skills:      true,
            Permissions: true,
            Commands:    true,
            Agents:      true,
            Rules:       true,
            Bin:         true,
            Deps:        true,
            // Set false for unsupported features:
            Plugins:     false,
            AgentMemory: false,
            Environment: false,
            Loop:        false,
            WorktreeInclude: false,
        },
    }
}
```

### reader.go -- Native reader

```go
package myagent

import (
    "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
    compiler.RegisterReader("myagent", func() compiler.NativeReader { return &Reader{} })
}

type Reader struct{}

func (r *Reader) AgentName() string { return "myagent" }

func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
    ir := &compiler.LayerIR{
        SourceAgent: "myagent",
        LayerPath:   layerPath,
    }

    // Read instructions, settings, MCP, categories...
    // See claude/reader.go for the full pattern.

    return ir, nil
}
```

### writer.go -- Output writer

```go
package myagent

import (
    "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
    "gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

func init() {
    compiler.RegisterWriter("myagent", func() compiler.Writer { return &Writer{} })
}

type Writer struct{}

func (w *Writer) Write(
    result *compiler.AccumulatedResult,
    profile compiler.AgentProfile,
    outputDir string,
    opts compiler.CompileOptions,
) ([]schema.FileRecord, error) {
    var records []schema.FileRecord

    // Write instructions, settings, MCP, categories...
    // See claude/writer.go for the full pattern.

    return records, nil
}
```

## Step 3: Register Format Detection

Add an `AgentMeta` entry in `pkg/config/layers.go`:

```go
var Agents = []AgentMeta{
    // ... existing agents ...
    {
        Name:   "myagent",
        Format: "v1-myagent",
        Dirs:   []string{".myagent"},
        Files:  []string{"MYAGENT.md"},
    },
}
```

This tells Keel how to detect native layers for your agent and which `schema` value targets it.

## Step 4: Import the Package

Add a blank import in `cmd/keel/main.go` so the adapter's `init()` functions run:

```go
import (
    _ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/myagent"
)
```

## Step 5: Add Test Fixtures

Create a fixture layer and expected output tests.
See `examples/compiler/` for the pattern -- the fixture test suite compiles an 8-layer stack and asserts against output.

## Step 6: Run Tests

```bash
go test ./...
```

The registry consistency tests automatically verify:
- Every agent in `config.Agents` has a registered compiler.
- Every registered compiler has an entry in `config.Agents`.

## Key Decisions

When building an adapter, consider:

| Decision | Options | How to Set |
|----------|---------|------------|
| Where do hooks live? | In settings or separate file | `HooksInSettings` on profile |
| Where are instructions? | Root-level file or inside config dir | `InstructionOutput` path |
| What features does the agent support? | Feature set | `Capabilities` struct |
| How do hook keys map? | PascalCase, camelCase, other | `HookKeyTransform` function |
| How do tool names map? | Shared vocabulary or custom | `ToolNameTransform` function |

## Pipeline Integration

Your adapter plugs into the shared pipeline automatically.
`CompileWithProfile` handles the full flow:

1. **First-compile guard** -- errors if Keel-managed output files already exist (use `--force` to overwrite).
2. **Read** -- each layer read into `LayerIR` via the appropriate reader.
3. **Fold** -- agent-neutral contribution collection of all layers.
4. **Governance + Transform** — `ApplyGovernance` enforces force-locks on settings and categories. `ApplyAgentGovernance` filters hooks by settings lock. `TransformForAgent` applies hooks strategy, frontmatter translation, category pruning.
5. **ProcessDeps** -- dependency installation and variable injection.
6. **Finalize** -- variable substitution, conditional evaluation, snippet resolution, MCP condition filtering.
7. **AssembleAgentOutput** -- settings assembly using profile-driven assembler functions.
8. **Write** -- your writer serializes `AccumulatedResult` to disk.

See **[Compilation Pipeline]({{< relref "pipeline" >}})** for the full flow.

## Container Adapter

To also support the agent in Keel's container tier, create container adapter scripts:

1. `adapters/<agent>/install.sh` -- install the agent binary.
2. `adapters/<agent>/adapter.sh` -- implement the 7 lifecycle functions.

See the [runtime contract]({{< relref "/docs/reference/runtime-contract" >}}) for the adapter interface.

## What's Next

- **[IR Design]({{< relref "ir" >}})** -- the intermediate representation your adapter consumes.
- **[Compilation Pipeline]({{< relref "pipeline" >}})** -- end-to-end compilation flow.
- **[Merge Strategies]({{< relref "merge-strategies" >}})** -- how each merge strategy works.
