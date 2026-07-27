package compiler

// PIPELINE STAGES
//
// The Keel compilation pipeline follows a clean assembly-line model:
//
//   FOLD      → Fold: accumulate all content, variables, contributions
//   GOVERN    → Force-locks, conflict tracking, policy enforcement
//   TRANSFORM → Agent-specific structural transforms (key casing, format mapping)
//   RESOLVE   → Variable resolution, condition evaluation, template rendering
//   ASSEMBLE  → Post-pipeline: settings assembly, guard checks
//   OUTPUT    → Write files to disk (pure serialization)
//
// Each stage has one job. No stage does work that belongs to another stage.
// When adding new features, identify which stage the work belongs to and
// put it there — don't scatter processing across multiple stages.
//
// Current mapping:
//   FOLD      = Fold()
//   GOVERN    = ApplyGovernance() + ApplyAgentGovernance()      ─┐
//   TRANSFORM = TransformForAgent() + ProcessDeps()              │ runPostFoldPipeline()
//   RESOLVE   = Finalize()                                      ─┘
//   ASSEMBLE  = runAssemblyStage()                              — see compile_pipeline.go
//   OUTPUT    = Write()
//
// GOVERN through RESOLVE are unified in runPostFoldPipeline() — both
// CompileWithProfile and WriteSharedOutput call this shared function.
// Agent-specific stages (AgentGovernance, TransformForAgent) are skipped
// when profile is nil (shared phase).

// SECTION GUIDE
//
// Adding a new DSL section to Keel requires changes in these locations:
//
// 1. Schema (pkg/schema/schema.go):
//    - Add field to KeelConfig struct
//    - Add bounds check in Validate()
//
// 2. Reader (pkg/compiler/reader_dsl.go):
//    - Add read function: readNewSection()
//    - Call it from KeelDSLReader.Read()
//
// 3. IR (pkg/compiler/ir.go):
//    - For content: use ContentBlocks["section-name"]
//    - For config maps: use ConfigBlocks["section-name"]
//    - For file overlays: use Categories["section-name"]
//    - For new merge semantics: add field to LayerIR + CompilationContent
//
// 4. Accumulator (pkg/compiler/accumulator.go):
//    - Content blocks and config blocks: handled generically, no change needed
//    - Categories: handled generically via FoldConfig, no change needed
//    - New merge semantics: add handling in Fold()
//
// 5. Profile (pkg/compiler/profile.go):
//    - Content blocks: add ContentBlockDef to profile
//    - Config blocks: add ConfigBlockDef to profile
//    - Categories: add CategoryDef to profile
//    - Update Capabilities if needed
//
// 6. Writer:
//    - Content/config blocks: handled by WriteCommonOutput, no change needed
//    - Categories: handled by WriteCommonOutput, no change needed
//    - New semantics: add writing logic
//
// 7. Adapter profiles (claude/claude.go, cursor/cursor.go, etc.):
//    - Add block/category definitions to profile
//    - Set capability flag
//
// Most new sections fit into ContentBlocks, ConfigBlocks, or Categories.
// These require changes ONLY in steps 1, 2, 5, and 7 — the generic
// accumulation and writing infrastructure handles the rest.
//
// EXAMPLES:
//
// Content block (like loop or worktree-include):
//   - Schema: add []schema.FileRef field to KeelConfig
//   - Reader: call readContentBlock("section-name", config.SectionName)
//   - Profile: add ContentBlockDef{Name: "section-name", JoinSeparator: "\n\n", OutputPath: ".claude/section.md"}
//   - Adapter: add content block def to profile's ContentOutputs
//
// Config block (like environment):
//   - Schema: add []schema.FileRef field to KeelConfig
//   - Reader: call readConfigBlock("section-name", config.SectionName)
//   - Profile: add ConfigBlockDef{Name: "section-name", OutputPath: ".cursor/section.json"}
//   - Adapter: add config block def to profile's ConfigOutputs
//
// Category (like commands, agents, skills):
//   - Schema: add []schema.FileRef field to KeelConfig
//   - Reader: call readCategory("section-name", config.SectionName)
//   - Profile: add CategoryDef{DirName: "section-name", LockCategory: "section-name-singular"}
//   - Adapter: add category def to profile's Categories
//
// New merge semantics (like MCP with conditional filtering):
//   - Schema: add field to KeelConfig
//   - Reader: parse section into map[string]any or custom type
//   - IR: add field to LayerIR and CompilationContent
//   - Accumulator: add Fold logic in Fold() method
//   - Writer: add output logic
//   - Adapter: set capability flag if needed
