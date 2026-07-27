---
title: Compilation Pipeline
weight: 0
---

# Compilation Pipeline

How Keel transforms layered configuration into agent-native output.

## High-Level Flow

Every `keel compile` follows the same pipeline regardless of how many agents are targeted.

```mermaid
flowchart TB
    subgraph Input
        L1[Layer 1<br>keel.yml + files]
        L2[Layer 2<br>keel.yml + files]
        LN[Layer N<br>native / DSL]
        IL[".keel/.claude/<br>implicit layer"]
    end

    subgraph Read ["Read (per layer)"]
        DSL[DSL Reader<br>keel.yml parser]
        NR[Native Reader<br>Claude / Cursor]
        FR[Foreign Reader<br>cross-agent]
    end

    subgraph Fold ["Fold (GATHER)"]
        ACC[Accumulator<br>collect contributions]
    end

    subgraph Govern ["Governance"]
        AG[ApplyGovernance<br>settings, categories,<br>file policies]
        AAG[ApplyAgentGovernance<br>hooks settings lock]
    end

    subgraph Transform ["Transform"]
        TAF[TransformForAgent<br>keel.agent, frontmatter,<br>categories, hooks strategy]
    end

    subgraph Diagnostic
        EAW[EmitAgentWarnings<br>capabilities, portability]
    end

    subgraph Finalize ["Finalize (RESOLVE)"]
        MF[Filter MCP conditions]
        VS[Variable substitution]
        CE[Conditional evaluation]
        SR[Snippet resolution]
    end

    subgraph Assemble ["Assemble"]
        AAO[AssembleAgentOutput<br>settings assembly]
    end

    subgraph Write ["Write (OUTPUT)"]
        CW2[Claude Writer<br>CLAUDE.md, settings.json]
        CUR[Cursor Writer<br>.cursor/rules/, hooks.json]
    end

    L1 --> DSL
    L2 --> DSL
    LN --> NR
    LN --> FR
    IL --> NR

    DSL --> ACC
    NR --> ACC
    FR --> ACC

    ACC --> AG --> AAG --> TAF --> EAW

    EAW --> MF --> VS --> CE --> SR

    SR --> AAO

    AAO --> CW2
    AAO --> CUR
```

## Pipeline Phases

### 1. Read

Each layer is read into a `LayerIR` by the appropriate reader.
The reader selection depends on the layer's format:

```mermaid
flowchart LR
    Layer --> Check{has keel.yml?}
    Check -->|yes| DSL[DSL Reader]
    Check -->|no| Format{layer format}
    Format -->|matches target| Native[Native Reader +<br>Convention Reader]
    Format -->|different agent| Foreign[Foreign Reader<br>transpilation]

    DSL --> IR[LayerIR]
    Native --> IR
    Foreign --> IR
```

| Reader | When used | Cacheable? |
|--------|-----------|------------|
| DSL Reader | Layer has `keel.yml` | Yes (agent-independent) |
| Native Reader | Format matches target agent | No (agent-specific) |
| Foreign Reader | Format is a different agent | No (transpilation) |
| Convention Reader | `FormatKeel` without `keel.yml` | No (wraps native) |

When compiling for multiple agents, DSL layer reads are cached and reused.
Native and foreign layer reads run per-agent since they produce different IRs.

### 2. Fold

The accumulator is a **pure merge engine** with zero agent knowledge.
It merges each layer's IR into accumulated state using content-type-specific strategies.

```mermaid
flowchart TB
    subgraph FoldConfig
        CAT[Categories<br>superset of all agents]
    end

    subgraph "Per Layer"
        IR[LayerIR] --> META[Metadata<br>last-writer-wins]
        IR --> VARS[Variables<br>merge + protect auto-vars]
        IR --> INST[Instructions<br>append blocks]
        IR --> SET[Settings<br>deep-merge]
        IR --> PERM[Permissions<br>deep-merge]
        IR --> MCP_C[MCP<br>collect contributions]
        IR --> HOOK_C[Hooks<br>collect contributions]
        IR --> CATS[Categories<br>last-writer-wins per file]
        IR --> ENV[Environment<br>deep-merge]
        IR --> DEPS[Deps<br>first-occurrence-wins]
    end

    META --> STATE[AccumulationState]
    VARS --> STATE
    INST --> STATE
    SET --> STATE
    PERM --> STATE
    MCP_C --> STATE
    HOOK_C --> STATE
    CATS --> STATE
    ENV --> STATE
    DEPS --> STATE
```

The fold only has `FoldConfig { Categories []CategoryDef }`.
No agent name, no agent-specific flags.
MCP, hooks, settings, categories, and file policies are all collected as **ordered contributions** (not merged) so that governance and agent-specific processing can happen in separate phases.

### 3. ApplyGovernance

Processes all contributions collected during fold in fold-index order.
This enforces governance policies (force-locks, merge strategies) independently of agent identity.

```mermaid
flowchart TB
    STATE[AccumulationState<br>with contributions] --> SETTINGS

    SETTINGS["Process settings contributions<br>deep-merge, force-lock enforcement"] --> CATS
    CATS["Process category contributions<br>overlay, force-lock enforcement"] --> POLICIES
    POLICIES["Register file policies<br>force-locks for subsequent layers"]

    POLICIES --> GOVERNED[AccumulationState<br>governed]
```

This phase maintains the original fold ordering: a layer's own content is accepted before its force-locks affect subsequent layers.

### 4. ApplyAgentGovernance

Agent-specific governance that requires knowledge of the target agent.
Currently handles hooks settings lock filtering.

```mermaid
flowchart TB
    GOVERNED[AccumulationState<br>governed] --> HSL

    HSL["ApplyHooksSettingsLock()<br>if hooks in settings + locked:<br>filter post-lock hook contributions"]

    HSL --> READY[AccumulationState<br>agent-governed]
```

### 5. TransformForAgent

All agent-specific structural transforms happen here.
This is where agent identity enters the pipeline for content transformation.

```mermaid
flowchart TB
    READY[AccumulationState<br>agent-governed] --> AGENT

    AGENT["Set keel.agent variable<br>set output root"] --> STRAT
    STRAT["Apply hooks strategy<br>Claude: last-writer-wins<br>Cursor: additive merge"] --> FRONT
    FRONT["Translate frontmatter<br>background ↔ is_background"] --> PRUNE
    PRUNE["Prune categories<br>remove unsupported"] --> META
    META["Populate content block metadata"]

    META --> TRANSFORMED[AccumulationState<br>agent-transformed]
```

### 6. EmitAgentWarnings

Diagnostic warnings for capability and portability issues.
Non-destructive — only adds warnings to the accumulation state.

```mermaid
flowchart TB
    TRANSFORMED[AccumulationState<br>agent-transformed] --> CAP

    CAP["Capability warnings<br>unsupported features"] --> PORT
    PORT["Portability warnings<br>lossy transpilation"]

    PORT --> READY[AccumulationState<br>ready for finalize]
```

### 7. ProcessDeps

Dependency installation and variable injection.
Installs external binaries, verifies them, and injects `keel.deps.<name>.path` and `keel.deps.<name>.dir` variables.

### 8. Finalize

Template resolution runs on the agent-transformed accumulated state.
The `keel.agent` variable is now set, so agent-conditional content resolves correctly.
MCP condition filtering happens here with full variable access.

```mermaid
flowchart LR
    subgraph "Content Processing"
        MF["MCP condition filtering<br>filter contributions<br>then deep-merge"] --> A
        A[Raw content with<br>variables + conditionals] --> B["Snippet resolution<br>keel:include name"]
        B --> C["Conditional evaluation<br>keel-if: keel.agent == claude"]
        C --> D["Variable substitution<br>${keel:variable}"]
    end

    subgraph "Applied to"
        D --> I[Instructions]
        D --> CAT[Category files]
        D --> S[Settings JSON]
        D --> H[Hooks JSON]
        D --> M[MCP JSON]
        D --> E[Environment JSON]
    end
```

### 9. AssembleAgentOutput

Settings assembly runs after finalization and before writing.
This phase merges settings, permissions, hooks, sandbox, attribution, and MCP policy into the final settings map using profile-driven assembler functions.

```mermaid
flowchart TB
    AR[AccumulatedResult<br>finalized] --> ASSEMBLE

    ASSEMBLE["Merge portable settings<br>+ agent-specific settings<br>+ permissions<br>+ hooks (if HooksInSettings)<br>+ sandbox<br>+ attribution<br>+ MCP policy"] --> TRANSFORM_KEYS
    TRANSFORM_KEYS["Apply key transforms<br>hook key casing<br>permission format"]

    TRANSFORM_KEYS --> READY[AccumulatedResult<br>assembled settings]
```

The assembler uses the agent profile's `SettingsDefaults` (e.g., Cursor's `version: 1`) and `SettingsAssembler` function to produce agent-specific settings structure.

### 10. Write

Each agent's writer serializes the `AccumulatedResult` to disk.
Writers contain no merge logic, no variable substitution, no governance enforcement, and no settings assembly.
They are pure serializers.
Common output (categories, content blocks, config blocks) is handled by `WriteCommonOutput`.
All files are written with content hashes for drift detection.

```mermaid
flowchart LR
    AR[AccumulatedResult<br>assembled] --> WCO[WriteCommonOutput<br>categories + content/config blocks<br>with content hashes]
    AR --> CW[Claude Writer]
    AR --> KW[Cursor Writer]

    WCO --> COMMON[agents/, skills/, commands/<br>loop, worktree-include]

    CW --> CM[CLAUDE.md]
    CW --> CS[.claude/settings.json<br>pre-assembled]
    CW --> CMP[.mcp.json]

    KW --> KR[.cursor/rules/00-keel.mdc]
    KW --> KS[.cursor/cli-config.json<br>pre-assembled]
    KW --> KH[.cursor/hooks.json]
    KW --> KMP[.cursor/mcp.json]
```

## Post-Compilation

After writing files to disk, the compiler captures a complete record of what was compiled.
This record powers compliance reporting, drift detection, and governance enforcement.

### Compliance Story

```
              COMPLIANCE STORY

  "What was compiled?"         → keel report (from manifest)
  "Has it changed?"            → keel drift (file hash comparison)
  "What specifically changed?" → keel drift --semantic (effective state diff)
  "Were policies violated?"    → keel drift --policy (force-lock check)
```

### Data Flow

```mermaid
flowchart TB
    subgraph Compilation
        ACC[AccumulatedResult] --> BES[BuildEffectiveState]
        ACC --> BGR[buildGovernanceRecord]
        ACC --> W[Writer + hashes]
    end

    BES --> MAN
    BGR --> MAN
    W --> MAN

    MAN[".keel.json manifest<br>layers, file_ops, force_locks<br>governance, effective_state"]

    MAN --> RPT["keel report<br>JSON / YAML / text"]
    MAN --> DFT["keel drift<br>file hash comparison"]
    MAN --> SEM["keel drift --semantic<br>effective state diff"]
    MAN --> POL["keel drift --policy<br>force-lock violations"]
```

The manifest contains:
- **Layer BOM** — complete layer stack with content hashes
- **File operations** — every file written with content hash and strategy
- **Force locks** — governance records for locked content
- **Governance record** — policy decisions (permissions policy, MCP policy, override conflicts, hook lock strategy)
- **Effective state** — authoritative compile-time snapshot (permissions, MCP servers, hooks, settings, instructions, categories)
- **Dependencies** — installed binaries with paths and scope

All files get content hashes, not just user-editable files — this enables precise drift detection for overlay files (commands, agents, skills).

## Multi-Agent Compilation

When compiling for multiple agents (`--output claude=. --output cursor=.`), the `CompileMulti` function shares work across agents.

```mermaid
sequenceDiagram
    participant CLI as CLI
    participant CM as CompileMulti
    participant Cache as LayerCache
    participant C1 as Claude Pipeline
    participant C2 as Cursor Pipeline

    CLI->>CM: CompileMulti(agents, layers, opts)
    CM->>Cache: Create shared cache

    Note over CM: For each agent (sorted)

    CM->>C1: Compile("claude", layers, opts + cache)
    C1->>Cache: ReadOrCache(layer1) - DSL
    Cache-->>C1: LayerIR (read from disk)
    C1->>Cache: ReadOrCache(layer2) - DSL
    Cache-->>C1: LayerIR (read from disk)
    C1->>C1: Fold → Govern → Transform → Finalize → Assemble → Write

    CM->>C2: Compile("cursor", layers, opts + cache)
    C2->>Cache: ReadOrCache(layer1) - DSL
    Cache-->>C2: LayerIR (cache hit!)
    C2->>Cache: ReadOrCache(layer2) - DSL
    Cache-->>C2: LayerIR (cache hit!)
    C2->>C2: Fold → Govern → Transform → Finalize → Assemble → Write

    CM-->>CLI: results map
```

DSL layers are read once and reused.
Each agent gets its own `AccumulationState` — fold is cheap (in-memory map merges).

## Implicit Native Layers

During compilation, Keel checks for a `.keel/<agent>/` directory (e.g., `.keel/.claude/`) and automatically includes it as the lowest-priority layer if it exists. This directory can be manually populated with native configuration files that should serve as defaults.

```mermaid
flowchart TB
    subgraph "Compilation"
        IMPLICIT[".keel/.claude/<br>if exists"]
        L1[Keel layers]
        IMPLICIT -->|lowest priority| FOLD[Fold]
        L1 -->|higher priority| FOLD
        FOLD --> OUT[Compiled output<br>merges all layers]
    end
```

The implicit layer directory is never automatically created or modified by Keel — it must be manually populated if needed.

## Clean and Manifest

`keel clean` uses the `.keel.json` manifest to reverse compilation.

```mermaid
flowchart TB
    LOAD[Load .keel.json manifest] --> OWNED[Collect owned paths<br>from FileOps + Overlays]

    OWNED --> MERGE{merge-into files?}
    MERGE -->|yes| CLEAN_MCP[Remove managed<br>MCP servers only<br>preserve OAuth tokens]
    MERGE -->|no| DEL

    CLEAN_MCP --> DEL[Delete owned files]
    DEL --> EMPTY[Remove empty<br>parent directories]
```

## Prune (Incremental Compile)

On subsequent compiles, `PruneStaleOutput` removes files from the previous compile that are no longer produced.

```mermaid
flowchart LR
    OLD[Previous manifest<br>owned files] --> DIFF{in new output?}
    NEW[New compilation<br>output files] --> DIFF

    DIFF -->|no longer produced| REMOVE[Remove stale file]
    DIFF -->|still produced| KEEP[Keep / overwrite]
```

## Governance: Force-Locks

Force-locks prevent downstream layers from overwriting governed content.

```mermaid
flowchart TB
    L1["Layer 1 (org policy)<br>force: true on deploy.md"] -->|sets lock| REG[Force Registry]
    L2["Layer 2 (team)<br>writes deploy.md"] -->|check| REG
    REG -->|locked!| SKIP[Skip — layer 1 wins]
    L3["Layer 3 (project)<br>writes review.md"] -->|check| REG
    REG -->|not locked| ACCEPT[Accept — last-writer-wins]
```

Force-locks work across all content types: settings, hooks, MCP servers, categories, and file policies.
They are tracked in the manifest and stripped from agent-native output (unless `--distributable`).

## Data Types Through the Pipeline

```mermaid
flowchart LR
    subgraph Types
        LIR["LayerIR<br>one layer's content<br>agent-agnostic"]
        AS["AccumulationState<br>all layers merged<br>contributions collected"]
        AR["AccumulatedResult<br>fully resolved<br>ready to serialize"]
    end

    R[Readers] -->|produce| LIR
    LIR -->|Fold| AS
    AS -->|Govern + Transform + Finalize + Assemble| AR
    AR -->|Write| OUT[Disk output]
```

| Type | Created by | Consumed by | Agent-specific? |
|------|-----------|-------------|-----------------|
| `LayerIR` | Readers | Fold | No (format-neutral) |
| `AccumulationState` | Fold | ApplyGovernance, ApplyAgentGovernance, TransformForAgent, EmitAgentWarnings, ProcessDeps, Finalize | No until TransformForAgent |
| `AccumulatedResult` | Result() | AssembleAgentOutput, Writers | Yes (post-TransformForAgent) |
| `CompileResult` | CompileWithProfile | Manifest writer, CLI | Yes (includes file records) |

## Import Flow

`keel import` converts native agent config into a keel DSL layer.

```mermaid
flowchart LR
    SRC[Native config<br>.claude/ or .cursor/] --> NR[Native Reader]
    NR --> IR[LayerIR]
    IR --> SER[SerializeIR]
    SER --> OUT[keel.yml layer<br>+ companion files]
```

The same IR that powers compilation also powers import.
No separate conversion logic — read into IR, serialize from IR.
