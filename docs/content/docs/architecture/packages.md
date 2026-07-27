---
title: Package Structure
weight: 1
---

# Package Structure

Keel's package organization follows a clean dependency graph with no circular dependencies.
Each package has a single responsibility and clear boundaries.

## Package Map

```
pkg/schema/     → DSL input types (KeelConfig, Settings, FileRef)
pkg/manifest/   → Post-compilation (CompiledManifest, EffectiveState, drift, cleanup)
pkg/compiler/   → IR types, pipeline, readers, writers, adapters
pkg/report/     → Compliance reporting (uses manifest types directly)
pkg/config/     → Layer resolution, agent registry
pkg/merge/      → Deep merge, variable substitution, conditionals
pkg/validate/   → Layer validation, linting
pkg/policy/     → Merge policy types
```

## Dependency Direction

```
schema ← manifest ← compiler → report
  ↑                    ↓
  └─────── config ─────┘
           ↓
         merge
           ↓
       validate
           ↓
        policy
```

Dependencies flow in one direction.
No circular dependencies.

## pkg/schema

DSL input types.
Defines the structure of `keel.yml` files.

**Key types:**
- `KeelConfig` — root structure of a keel.yml file
- `Settings` — portable agent settings
- `FileRef` — reference to a file or directory
- `PermissionsPolicy` — merge policy for permissions
- `DepSpec` — external dependency specification

**Used by:** readers, validation, config resolution

**Dependencies:** none (pure data types)

## pkg/manifest

Post-compilation types.
Everything written to `.keel.json` after compilation.

**Key types:**
- `CompiledManifest` — complete compilation record
- `EffectiveState` — authoritative compile-time snapshot
- `GovernanceRecord` — policy decisions made during compilation
- `FileOp` — record of a file written to disk
- `ForceLock` — governance lock record

**Key functions:**
- `DetectDrift()` — file hash comparison
- `DetectSemanticDrift()` — effective state diff
- `CleanAllOutput()` — remove compiled files
- `PruneStaleOutput()` — incremental cleanup

**Used by:** compiler (writes manifests), drift detection, clean, report

**Dependencies:** `pkg/schema` (for layer metadata)

## pkg/compiler

The compilation pipeline.
IR types, readers, writers, and agent adapters.

**Key types:**
- `LayerIR` — agent-agnostic layer representation
- `AccumulationState` — internal working state during fold
- `AccumulatedResult` — fully resolved output
- `CompileResult` — compilation output with file records
- `AgentProfile` — agent adapter configuration

**Key functions:**
- `CompileWithProfile()` — main compilation pipeline
- `Fold()` — merge layers into accumulated state
- `Finalize()` — variable substitution and template resolution
- `Write()` — serialize to disk

**Package structure:**
```
pkg/compiler/
├── ir.go               — LayerIR, AccumulatedResult
├── accumulator.go      — AccumulationState, Fold
├── profile.go          — AgentProfile, CompileWithProfile
├── reader_dsl.go       — KeelDSLReader
├── portable.go         — portable settings classification
├── section_guide.go    — guide for adding new sections
├── drift.go            — drift detection (delegates to pkg/manifest)
├── cleanup.go          — clean/prune (delegates to pkg/manifest)
├── claude/             — Claude adapter
│   ├── claude.go       — profile and capabilities
│   ├── reader.go       — native reader
│   └── writer.go       — native writer
├── cursor/             — Cursor adapter
│   ├── cursor.go       — profile and capabilities
│   ├── reader.go       — native reader
│   └── writer.go       — native writer
└── agentsmd/           — AGENTS.md adapter
    ├── agentsmd.go     — profile and capabilities
    └── writer.go       — writer only (no native reader)
```

**Used by:** CLI commands

**Dependencies:** `pkg/schema`, `pkg/manifest`, `pkg/merge`, `pkg/policy`

## pkg/report

Compliance report generation.

**Key functions:**
- `Generate()` — create compliance report from manifest
- `RenderText()` — human-readable output

**Used by:** `keel report` command

**Dependencies:** `pkg/manifest` (reads manifests directly)

## pkg/config

Layer resolution and agent registry.

**Key types:**
- `LayerSource` — resolved layer location
- `AgentRegistry` — available agents and their profiles

**Key functions:**
- `ResolveLayers()` — expand globs, download remote layers
- `LoadConfigFile()` — read `.keel/config.yml`
- `RegisterAgent()` — add agent to registry

**Used by:** CLI commands, compiler

**Dependencies:** `pkg/schema`

## pkg/merge

Deep merge, variable substitution, and conditional evaluation.

**Key functions:**
- `DeepMergeMapsWithPolicies()` — merge two maps with policy handling
- `SubstituteVariables()` — replace `${keel:var}` in content
- `ProcessConditionals()` — process `keel-if/elif/else/endif` blocks
- `ResolveSnippets()` — expand `keel:include` directives

**Used by:** compiler (during Finalize)

**Dependencies:** none (pure functions)

## pkg/validate

Layer validation and linting.

**Key functions:**
- `ValidateLayer()` — check keel.yml schema
- `ValidateFileRefs()` — verify referenced files exist
- `LintMarkdown()` — check for non-portable patterns
- `ValidateAgentSkills()` — Agent Skills format validation

**Used by:** `keel validate` command

**Dependencies:** `pkg/schema`, `pkg/policy`

## pkg/policy

Merge policy types and enforcement.

**Key types:**
- `MergePolicy` — enum (additive, replace)
- `KeelMetadata` — `_keel` governance in JSON files

**Key functions:**
- `ExtractPolicy()` — read `_keel` metadata from JSON
- `StripPolicy()` — remove `_keel` metadata
- `ApplyPolicy()` — enforce merge policy

**Used by:** merge, compiler

**Dependencies:** none (pure data types)

## Design Principles

### Single Responsibility

Each package has one job:
- `schema` — define the DSL
- `manifest` — track compilation output
- `compiler` — transform layers into agent config
- `report` — format compliance reports
- `config` — resolve layer sources
- `merge` — combine data structures
- `validate` — check correctness
- `policy` — govern merges

### No Circular Dependencies

Dependencies flow in one direction.
If package A imports package B, package B never imports package A.

This makes the codebase easier to understand and test.

### Clean Separation

**Input vs. Output:**
- `pkg/schema` — input types (keel.yml)
- `pkg/manifest` — output types (.keel.json)
- `pkg/compiler` — transformation between them

**Pipeline vs. Post-Compilation:**
- `pkg/compiler` — compilation pipeline
- `pkg/manifest` — drift, cleanup, effective state

**Pure vs. Effectful:**
- `pkg/merge` — pure functions (no I/O)
- `pkg/compiler` — effectful (reads files, writes output)

### Package Growth

When adding new features, ask:
- Is this DSL input? → `pkg/schema`
- Is this compilation output? → `pkg/manifest`
- Is this pipeline logic? → `pkg/compiler`
- Is this post-compilation? → `pkg/manifest`
- Is this pure merge logic? → `pkg/merge`
- Is this validation? → `pkg/validate`
- Is this governance? → `pkg/policy`

Don't create new packages unless existing ones don't fit.
Growing existing packages is better than proliferating small ones.

## Testing

Each package has its own test files.
Tests are co-located with implementation.

**Unit tests:**
- `pkg/schema/schema_test.go` — schema validation, JSON tag completeness
- `pkg/merge/merge_test.go` — deep merge, variable substitution
- `pkg/compiler/reader_dsl_test.go` — DSL parsing

**Integration tests:**
- `examples/compiler/fixture_test.go` — end-to-end compilation
- `pkg/manifest/drift_test.go` — drift detection
- `pkg/manifest/cleanup_test.go` — clean/prune behavior

Tests depend only on the packages they test.
No test circular dependencies.
