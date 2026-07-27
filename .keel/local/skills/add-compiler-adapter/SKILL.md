---
name: add-compiler-adapter
description: Build a new compiler adapter that translates Keel's internal representation into a target agent's native config format. Use when adding support for a new AI coding agent to Keel's compiler tier.
---

# Add Compiler Adapter

Build a new compiler adapter for a target AI coding agent.
The adapter translates Keel's internal representation into the agent's native config files.

## Prerequisites

- The target agent must accept filesystem-based configuration (dotfiles, config directories, markdown instructions).
- You need to know the agent's config format, file locations, and settings structure.

## Step 1: Research the target agent

Check `docs/content/docs/reference/compatibility-matrix.md` — the single source of truth for all cross-agent portability claims.
It contains the complete file inventory, hook events, handler types, frontmatter specs, settings keys, and portability annotations for all supported agents.

If the target agent is not yet in the matrix, read `docs/content/docs/ecosystem/agents.md` for the agent's entry, then run a config surface audit using the `config-surface-audit` skill to gather the data needed to add the agent to the matrix.

Key details to extract from the schema (or research if no schema exists):

- Config format and file locations (e.g., `~/.gemini/`, `GEMINI.md`)
- Settings file format (JSON, YAML, TOML)
- Skills path convention (e.g., `.gemini/skills/{name}/SKILL.md`)
- MCP server config location and format
- Hooks support and format
- Instruction file names and conventions
- Portability gaps vs existing agents (for `portability.go` updates)
- Auth model (for container adapter later)

If the agent is not yet in `docs/content/docs/ecosystem/agents.md`, run a `config-surface-audit` first — it will add the agent to the matrix as part of its output.

## Step 2: Study the existing adapters

Read these files to understand the adapter pattern:

1. `pkg/compiler/compiler.go` -- the `CompileFunc` type signature and the `compilers` registry map.
2. `pkg/compiler/registry.go` -- the `RegisterCompiler()` function that adapters call from their `init()`.
3. `pkg/compiler/profile.go` -- the `AgentProfile` struct and the `CompileWithProfile()` shared skeleton. This is the core of the compilation pipeline. Study every field of `AgentProfile` and understand what each one controls.
4. `pkg/compiler/claude/claude.go` -- the Claude adapter. Note how `Compile` calls `CompileWithProfile(Profile(opts.Scope), ...)` and how the `init()` function registers both the agent metadata and the compiler function.
5. `pkg/compiler/aider/aider.go` -- a minimal adapter. Study this for the simplest pattern: just instructions, no settings/MCP/hooks.
6. `pkg/compiler/cursor/cursor.go` -- the Cursor adapter. Note the differences from Claude: `.mdc` frontmatter wrapping, `cli-config.json` instead of `settings.json`, hook key transform, extra `rules` category.
7. `pkg/config/layers.go` -- the `AgentMeta` struct and `RegisterAgent()` pattern. Note how marker files/dirs drive format auto-detection.

Key observations:
- Adapters live in subpackages: `pkg/compiler/{agent}/{agent}.go`, `reader.go`, `writer.go`
- Each adapter registers itself via `init()` — no central registration file
- Adapters delegate to `CompileWithProfile()` -- the adapter itself is mostly a profile definition
- The `SettingsAssembler` function translates DSL permissions and settings into the agent's native format
- `HookKeyTransform` and `ToolNameTransform` adapt naming conventions across agents
- `Categories` defines overlay directories (commands, agents, skills, plus agent-specific ones)

## Step 3: Create the adapter subpackage

Create `pkg/compiler/<agent>/` as a new directory.

### File structure

```
pkg/compiler/<agent>/
  <agent>.go       — Compile function, Profile, init() registration
  reader.go        — NativeReader implementation
  writer.go        — Writer implementation
```

### A. The main adapter file (`<agent>.go`)

```go
package <agent>

import (
    "gitlab.com/ripp.io/oss/keel/pkg/compiler"
    "gitlab.com/ripp.io/oss/keel/pkg/config"
)

func init() {
    config.RegisterAgent(config.AgentMeta{Format: "<agent>", Files: []string{"<MARKER>.md"}, Dirs: []string{".<agent>"}})
    compiler.RegisterCompiler("<agent>", Compile)
}

// Compile produces <Agent>-native configuration output.
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
    return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for <Agent>.
func Profile(scope compiler.Scope) compiler.AgentProfile {
    if scope == compiler.ScopeUser {
        return UserProfile()
    }
    return ProjectProfile()
}

// ProjectProfile returns the project-level AgentProfile for <Agent>.
func ProjectProfile() compiler.AgentProfile {
    caps := compiler.DefaultCapabilities()
    // Set caps based on what the agent supports

    return compiler.AgentProfile{
        Name:              "<agent>",
        OutputRoot:        ".<agent>",
        InstructionOutput: "<AGENT>.md",
        SettingsFile:      "settings.json",
        MCPOutput:         ".mcp.json",
        HooksInSettings:   true,
        Categories:        compiler.BaseCategories(),
        InstructionCandidates: []string{
            "<AGENT>.md",
            "instructions.md",
            "INSTRUCTIONS.md",
            "AGENTS.md",
        },
        HookKeyTransform:  compiler.IdentityTransform(),
        ToolNameTransform: compiler.IdentityTransform(),
        Capabilities:      caps,
        SettingsAssembler: func(settings map[string]any, result *compiler.AccumulatedResult) {
            // Translate DSL to agent-native format
        },
    }
}
```

### B. Key AgentProfile fields

| Field | What to set |
|-------|-------------|
| `Name` | Agent identifier, lowercase (e.g., `"gemini"`) |
| `OutputRoot` | Dot-directory (e.g., `".gemini"`) |
| `InstructionOutput` | Compiled instruction file path relative to outputDir |
| `SettingsFile` | Settings filename inside OutputRoot |
| `SettingsLockKey` | Force-lock key for the whole settings file |
| `MCPOutput` | MCP config file path relative to outputDir |
| `HooksInSettings` | `true` if hooks go inside settings file, `false` for separate `hooks.json` |
| `Categories` | `compiler.BaseCategories()` plus agent-specific ones (e.g., Cursor adds `rules`) |
| `InstructionCandidates` | Filenames to look for in native layers, in priority order |
| `HookKeyTransform` | Function to transform hook event names. Use `compiler.IdentityTransform()` if the agent uses the same casing as Keel's internal representation |
| `ToolNameTransform` | Function to transform MCP tool names |
| `MCPNativeLockCategory` | Force-lock category for MCP file in native layers. Empty string if no file-level lock |
| `Capabilities` | Which features the agent supports (permissions, hooks, MCP, etc.) |
| `SettingsAssembler` | Function that translates DSL to the agent's native settings format |

### C. The SettingsAssembler function

This is a closure that receives:
- `settings map[string]any` — the mutable settings map to populate
- `result *compiler.AccumulatedResult` — the accumulated IR with permissions, hooks, MCP, etc.

Study `pkg/compiler/claude/claude.go` lines 84-106 for the Claude example.
Study `pkg/compiler/aider/aider.go` for a minimal example (no settings at all).

Common patterns:
- Permissions: `compiler.AssemblePermissions()` or custom translation
- Hooks: `compiler.AssembleHooks()`
- Sandbox: `compiler.AssembleSandboxEmbed()`
- MCP policy: `compiler.AssembleMCPPolicy()`

### D. Reader and Writer (optional initially)

You can stub these out initially and implement later when adding `keel import` support.

```go
// reader.go
package <agent>

import "gitlab.com/ripp.io/oss/keel/pkg/compiler"

func init() {
    compiler.RegisterReader("<agent>", func() compiler.NativeReader { return &Reader{} })
}

type Reader struct{}

func (r *Reader) Read(dir string) (*compiler.NativeLayer, error) {
    // TODO: implement
    return nil, nil
}
```

```go
// writer.go
package <agent>

import "gitlab.com/ripp.io/oss/keel/pkg/compiler"

func init() {
    compiler.RegisterWriter("<agent>", func() compiler.Writer { return &Writer{} })
}

type Writer struct{}

func (w *Writer) Write(ir *compiler.IntermediateRepresentation, outputDir string) error {
    // TODO: implement
    return nil
}
```

## Step 4: Registration is automatic

The `init()` function in your `<agent>.go` file already handles registration.
Both `config.RegisterAgent()` and `compiler.RegisterCompiler()` are called from the same `init()`.

No need to edit any central registry files — the Go `init()` pattern handles it automatically when the package is imported.

## Step 5: Add settings field (if needed)

If the agent supports agent-specific settings files (e.g., `settings.<agent>.json`), add a field to the `Settings` struct in `pkg/schema/schema.go`:

```go
type Settings struct {
    Claude string `yaml:"claude,omitempty"`
    Cursor string `yaml:"cursor,omitempty"`
    <Agent> string `yaml:"<agent>,omitempty"`
    Model  string `yaml:"model,omitempty"`
}
```

## Step 6: Add golden test fixtures

Create test layer directories in `pkg/compiler/testdata/`:

1. Add agent-specific content to existing test layers if the layers should also compile for the new agent.
2. Create expected output in `pkg/compiler/testdata/expected/<agent>/` with the files the compiler should produce.
3. Write a test function `TestCompile<Agent>_FullEnterpriseStack` in `pkg/compiler/compiler_<agent>_test.go` following the pattern of `pkg/compiler/compiler_claude_test.go`.

The existing `TestCompilerRegistryMatchesAgents` test will automatically verify that the new agent has both a compiler and an `AgentMeta` entry.

## Step 7: Run the test suite

```bash
go test ./...
go vet ./...
```

All tests must pass, including the registry consistency test.

## Checklist

- [ ] Target agent researched (config format, file locations, settings structure)
- [ ] Adapter subpackage created at `pkg/compiler/<agent>/`
- [ ] `<agent>.go` with `Compile`, `Profile`, and `init()` registration
- [ ] `AgentProfile` fully populated with correct paths and transforms
- [ ] `SettingsAssembler` translates DSL to agent-native format
- [ ] `reader.go` and `writer.go` stubbed or implemented
- [ ] Settings struct extended if needed in `pkg/schema/schema.go`
- [ ] Golden test fixtures created in `pkg/compiler/testdata/`
- [ ] `TestCompilerRegistryMatchesAgents` passes
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
