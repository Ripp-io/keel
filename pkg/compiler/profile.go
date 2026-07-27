package compiler

import (
	"os"
)

// AgentProfile captures the per-agent differences so that the compilation
// skeleton can be shared across all agent adapters. Each adapter package
// (claude, cursor, etc.) returns a profile via ProjectProfile()/UserProfile().
//
// Lifecycle: constructed once per compilation by the adapter, passed through
// CompileWithProfile → accumulation → assembly → writing. Immutable after
// construction.
//
// This type is part of Keel's public API for adapter authors. Adding a new
// agent requires implementing ProjectProfile()/UserProfile() that return
// an AgentProfile, plus a Reader and Writer.
//
// Design note: HookKeyTransform, ToolNameTransform, and SettingsAssembler are
// function fields rather than interface methods. This enables inline construction
// via struct literals (each adapter returns a profile with closures). The tradeoff
// is that profiles cannot be compared for equality, serialized, or cached by value.
// If these capabilities become necessary, extract the function fields into a
// ProfileBehavior interface and reference it from the profile struct.
type AgentProfile struct {
	// Name is the agent identifier ("claude" or "cursor").
	Name string

	// OutputRoot is the dot-directory under outputDir (".claude" or ".cursor").
	OutputRoot string

	// InstructionOutput is the compiled instruction file path relative to
	// outputDir ("CLAUDE.md" or ".cursor/rules/00-keel.mdc").
	InstructionOutput string

	// SharedInstructionOutput is the path for the shared instruction file
	// (e.g., "AGENTS.md"). Empty means this agent IS the shared format (agentsmd).
	SharedInstructionOutput string

	// SharedMCPOutput is the path for the shared MCP file (e.g., ".mcp.json").
	// Empty means no shared MCP output.
	SharedMCPOutput string

	// InstructionShimTemplate is the content template for the agent's
	// instruction shim file (e.g., "@AGENTS.md\n"). Empty means no shim needed
	// (agent's InstructionOutput IS the shared file).
	InstructionShimTemplate string

	// SettingsFile is the settings filename inside OutputRoot
	// ("settings.json" or "cli-config.json").
	SettingsFile string

	// SettingsLockKey is the force-lock key used for the whole settings file
	// ("settings.json" or "cli-config.json").
	SettingsLockKey string

	// MCPOutput is the MCP config file path relative to outputDir
	// (".mcp.json" or ".cursor/mcp.json").
	MCPOutput string

	// HooksInSettings controls whether hooks are merged into the settings
	// file (Claude) or written to a separate hooks.json (Cursor).
	HooksInSettings bool

	// Categories lists all directory-overlay categories for this agent
	// (e.g. commands, agents, skills, and any agent-specific extras like rules).
	Categories []CategoryDef

	// SettingsCandidates lists filenames to look for when resolving settings
	// in native layers, checked in priority order. The first match wins.
	// The tool-native filename (SettingsFile) should be first.
	SettingsCandidates []string

	// InstructionCandidates are filenames to look for in native layers,
	// checked in priority order.
	InstructionCandidates []string

	// HookKeyTransform transforms hook event names. Claude uses identity
	// (PascalCase), Cursor uses pascalToCamel.
	HookKeyTransform func(string) string

	// MCPMergeInto indicates that the MCP output file is shared with other
	// content (e.g., ~/.claude.json contains both MCP servers and OAuth tokens).
	// When true, the writer deep-merges only the mcpServers key into the
	// existing file content, preserving all other keys.
	MCPMergeInto bool

	// MCPNativeLockCategory is the force-lock category for the MCP file
	// in native layers. Empty string means no file-level lock check.
	MCPNativeLockCategory string

	// SandboxOutput is the sandbox config file path relative to outputDir
	// (e.g. ".cursor/sandbox.json"). Empty means sandbox config is embedded
	// in settings (Claude) or not supported.
	SandboxOutput string

	// ContentOutputs lists content blocks that concatenate like instructions.
	// Each block is written to a separate file using its defined separator.
	ContentOutputs []ContentBlockDef

	// ConfigOutputs lists config blocks that deep-merge like settings.
	// Each block is written to a separate JSON file.
	ConfigOutputs []ConfigBlockDef

	// ToolNameTransform maps tool names between agent vocabularies.
	// Claude and Cursor share tool names (identity transform). Future
	// agents with different vocabularies override this.
	ToolNameTransform func(string) string

	// Capabilities declares which DSL features this adapter supports.
	Capabilities Capabilities

	// SettingsDefaults are initial key-value pairs seeded into the settings map
	// before any accumulated data is merged. Used by Cursor for version: 1.
	SettingsDefaults map[string]any

	// SettingsAssembler is called by AssembleAgentOutput to apply agent-specific
	// settings transforms (permissions format, attribution format, sandbox embedding,
	// hooks embedding, MCP policy embedding). It mutates the settings map and may
	// read from the result. If nil, only basic merge is applied.
	SettingsAssembler func(settings map[string]any, result *AccumulatedResult)
}

// Capabilities declares which DSL features an adapter supports.
// The compiler warns when a layer uses a feature the target adapter does not support.
type Capabilities struct {
	Hooks       bool
	MCP         bool
	Skills      bool
	Permissions bool
	Commands    bool
	Agents      bool
	Plugins     bool
	AgentMemory bool
	Rules       bool
	Bin         bool
	Deps        bool

	// InstructionOnly suppresses capability warnings for this adapter.
	// Set for special-purpose output formats (like agentsmd) that intentionally
	// support only instructions — warning about missing hooks/mcp/etc. is noise.
	InstructionOnly bool
}

// BaseCategories returns the common categories shared by Claude and Cursor.
// Lightweight adapters (aider, cline, copilot, agentsmd) have no categories.
func BaseCategories() []CategoryDef {
	return []CategoryDef{
		{DirName: "commands", LockCategory: "command"},
		{DirName: "agents", LockCategory: "agent"},
		{DirName: "skills", LockCategory: "skill"},
	}
}

// DefaultCapabilities returns the default capability set for full-featured
// adapters like Claude and Cursor. Lightweight adapters override to disable
// most features.
func DefaultCapabilities() Capabilities {
	return Capabilities{
		Commands:    true,
		Agents:      true,
		Skills:      true,
		MCP:         true,
		Hooks:       true,
		Permissions: true,
		Bin:         true,
		Deps:        true,
	}
}

// IdentityTransform returns a no-op string transform function.
// Used by adapters that don't need to transform hook keys or tool names.
func IdentityTransform() func(string) string {
	return func(s string) string { return s }
}

// ContentBlockDef defines a content block that concatenates like instructions.
type ContentBlockDef struct {
	// Name is the canonical block name (e.g., "loop", "worktree-include").
	Name string
	// JoinSeparator is used when concatenating blocks (e.g., "\n\n" for loop).
	JoinSeparator string
	// OutputPath is the file path relative to outputDir.
	OutputPath string
}

// ConfigBlockDef defines a config block that deep-merges like settings.
type ConfigBlockDef struct {
	// Name is the canonical block name (e.g., "environment").
	Name string
	// OutputPath is the file path relative to outputDir.
	OutputPath string
}

// CategoryDef defines an overlay category for native layers.
type CategoryDef struct {
	// DirName is the directory name inside the layer and output root.
	DirName string
	// LockCategory is the force-lock category name.
	LockCategory string
}

// Auto-injected variable names — set by the compiler, protected from
// override via CLI --var flags.
const (
	varKeelAgent       = "keel.agent"
	varKeelScope       = "keel.scope"
	varKeelBinProject  = "keel.bin.project"
	varKeelBinGlobal   = "keel.bin.global"
	varKeelOutput      = "keel.output"
	varKeelName        = "keel.name"
	varKeelLayersCount = "keel.layers.count"
	varKeelVersion     = "keel.version"
	varKeelState       = "keel.state"
	varKeelHooksDir    = "keel.hooks.dir"
)

// varKeelDepsPrefix is the namespace prefix for per-dependency path variables
// (e.g. "keel.deps.mytool.path"). Any variable with this prefix is protected.
const varKeelDepsPrefix = "keel.deps."

// keelBuildVersion returns the Keel build version from the KEEL_BUILD env var,
// falling back to "dev" when unset.
func keelBuildVersion() string {
	if v := os.Getenv("KEEL_BUILD"); v != "" {
		return v
	}
	return "dev"
}

// FilterOut returns a new slice excluding items where name(item) == exclude.
// Used by agent profiles to remove scope-inappropriate entries
// (e.g., Cursor user-scope removes "rules" category, "environment" config block).
func FilterOut[T any](items []T, name func(T) string, exclude string) []T {
	var result []T
	for _, item := range items {
		if name(item) != exclude {
			result = append(result, item)
		}
	}
	return result
}
