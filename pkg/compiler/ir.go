package compiler

import (
	"fmt"
	"os"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
	"gopkg.in/yaml.v3"
)

// LayerIR is the agent-agnostic intermediate representation of a single
// configuration layer. It captures everything needed to either serialize
// to a keel.yml layer (keel import) or feed into the compilation pipeline
// for cross-format compilation (transpile).
//
// Lifecycle: produced by NativeReader.Read(), consumed by AccumulationState.Fold().
// Readers populate only the fields relevant to their format — missing data
// produces nil/empty fields, not errors. Fields are immutable after Read()
// returns; Clone() creates a deep copy for cache isolation.
//
// This type is part of Keel's public API for reader/writer adapter authors.
type LayerIR struct {
	// Metadata
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	// Instructions holds ordered instruction content blocks.
	// Each entry is the text from one instruction source file.
	Instructions []InstructionBlock `yaml:"instructions,omitempty"`

	// Permissions in normalized form: {"allow": [...], "deny": [...]}.
	// Uses bare permission names (Claude canonical form); Cursor's
	// trailing (*) is stripped during reading.
	Permissions map[string]any `yaml:"permissions,omitempty"`

	// MCP holds the mcpServers configuration.
	// Structure: {"mcpServers": {"name": {config...}}}
	MCP map[string]any `yaml:"mcp,omitempty"`

	// Settings holds agent-agnostic settings (e.g., model).
	// Agent-specific settings that have no cross-agent equivalent
	// are stored in AgentSpecific instead.
	Settings map[string]any `yaml:"settings,omitempty"`

	// Hooks normalized to PascalCase keys (Claude canonical form).
	// Cursor's camelCase keys are converted during reading.
	Hooks map[string]any `yaml:"hooks,omitempty"`

	// ContentBlocks holds content blocks for agent-specific features that
	// concatenate like instructions (e.g., "loop", "worktree-include").
	// Keys are canonical block names; values are ordered content blocks.
	ContentBlocks map[string][]InstructionBlock `yaml:"content_blocks,omitempty"`

	// ConfigBlocks holds config blocks for agent-specific features that
	// deep-merge like settings (e.g., "environment" for Cursor).
	// Keys are canonical block names; values are JSON config maps.
	ConfigBlocks map[string]map[string]any `yaml:"config_blocks,omitempty"`

	// Categories maps directory-overlay category names to their files.
	// Keys are canonical names: "commands", "agents", "skills",
	// "plugins", "agent-memory", "rules".
	Categories map[string][]CategoryFile `yaml:"categories,omitempty"`

	// ForceLocks records governance metadata from the source layer.
	// Preserved for round-trip fidelity through import/export.
	ForceLocks []ForceLockEntry `yaml:"force_locks,omitempty"`

	// AgentSpecific stores features unique to one agent that have no
	// cross-agent equivalent. Keyed by agent name (e.g., "claude").
	// Preserved as opaque data so nothing is silently dropped.
	AgentSpecific map[string]map[string]any `yaml:"agent_specific,omitempty"`

	// SourceAgent records which agent format this IR was read from.
	SourceAgent string `yaml:"source_agent"`

	// --- Fields added for IR-centric compilation (Phase 0) ---

	// Variables holds template variables defined in this layer.
	// Keys are variable names, values are their string values.
	Variables map[string]string `yaml:"variables,omitempty"`

	// Snippets holds snippet definitions from this layer.
	// Keys are snippet names (filename without .md), values are content bytes.
	Snippets map[string][]byte `yaml:"snippets,omitempty"`

	// Deps holds external binary dependency specifications.
	// Specs only — installation happens post-accumulation.
	Deps map[string]schema.DepSpec `yaml:"deps,omitempty"`

	// Bin holds bin/ file entries from this layer.
	Bin []BinFile `yaml:"bin,omitempty"`

	// Attribution holds portable attribution config.
	// Populated by readers from native or DSL layers.
	Attribution *CanonicalAttribution `yaml:"attribution,omitempty"`

	// Sandbox holds portable sandbox config.
	// Populated by readers from native or DSL layers.
	Sandbox *schema.SandboxConfig `yaml:"sandbox,omitempty"`

	// FilePolicies carries files: section governance from keel.yml.
	// Keys are file paths (e.g., "commands/deploy.md"), values are policies.
	FilePolicies map[string]schema.FilePolicy `yaml:"file_policies,omitempty"`

	// MCPPolicy carries MCP governance (allowed/denied/enableAll) from keel.yml.
	MCPPolicy *schema.MCPPolicy `yaml:"mcp_policy,omitempty"`

	// PermissionsPolicy carries permission merge strategy from keel.yml.
	PermissionsPolicy *schema.PermissionsPolicy `yaml:"permissions_policy,omitempty"`

	// Repos holds repository definitions for workspace init.
	Repos map[string]schema.Repo `yaml:"repos,omitempty"`

	// LayerPath is the absolute path to the layer directory.
	LayerPath string `yaml:"layer_path"`

	// DSLConfig is the raw parsed keel.yml, retained for capability
	// warnings during accumulation. Only set for DSL layers.
	// Not serialized — preserved separately in Clone().
	DSLConfig *schema.KeelConfig `yaml:"-"`
}

// Clone creates a deep copy of the LayerIR to prevent cache mutations.
// Returns a new LayerIR with copies of all mutable fields (maps and slices).
// Uses YAML round-trip for deep copying — all fields have yaml tags.
func (ir *LayerIR) Clone() (*LayerIR, error) {
	if ir == nil {
		return nil, nil
	}
	data, err := yaml.Marshal(ir)
	if err != nil {
		return nil, fmt.Errorf("LayerIR.Clone: marshal failed: %w", err)
	}
	var cloned LayerIR
	if err := yaml.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("LayerIR.Clone: unmarshal failed: %w", err)
	}
	// Preserve fields that don't round-trip through YAML
	cloned.DSLConfig = ir.DSLConfig // read-only during compilation
	return &cloned, nil
}

// CategoryDotDir is the pseudo-category name for arbitrary files in an
// agent's dot-directory that don't belong to a named category.
const CategoryDotDir = "_dotdir"

// InstructionBlock holds one instruction file's content with its source filename.
type InstructionBlock struct {
	Filename   string `yaml:"filename"`
	Content    string `yaml:"content"`
	LayerLabel string `yaml:"layer_label"` // layer label for section separator (e.g., "base", "company")
}

// CategoryFile represents a single file within a category directory.
type CategoryFile struct {
	// RelPath is the path relative to the category root
	// (e.g., "deploy.md" or "sub/review.md").
	RelPath string `yaml:"rel_path"`
	Content []byte `yaml:"content"`
	IsDir   bool   `yaml:"is_dir"`
	// SourceAgent records which agent format this file came from (for frontmatter translation).
	// Empty or "keel" means native/DSL format (no translation needed).
	SourceAgent string `yaml:"source_agent"`
}

// ForceLockEntry records a force-lock from the source configuration.
type ForceLockEntry struct {
	Category string `yaml:"category"` // e.g., "command", "skill", "hook"
	Key      string `yaml:"key"`      // item identifier within the category
}

// BinFile represents a single file in the .keel/bin/ overlay directory.
type BinFile struct {
	RelPath string      `yaml:"rel_path"`
	Content []byte      `yaml:"content"`
	Mode    os.FileMode `yaml:"mode"`
}

// CanonicalAttribution represents attribution config in a portable format.
// Claude uses template strings; Cursor uses boolean flags.
// The canonical form captures both: the enabled flag (portable) and the
// template (Claude-specific detail, preserved for round-trip fidelity).
type CanonicalAttribution struct {
	CommitEnabled  bool   `yaml:"commit_enabled"`
	CommitTemplate string `yaml:"commit_template"` // Claude-specific; empty = use agent default
	PREnabled      bool   `yaml:"pr_enabled"`
	PRTemplate     string `yaml:"pr_template"` // Claude-specific; empty = use agent default
}

// CompilationContent holds compiled content ready for serialization by writers.
// All merge logic, variable substitution, and governance have been applied.
// Writers transform this into agent-native file formats.
type CompilationContent struct {
	Instructions  string                    // concatenated, post-substitution
	Settings      map[string]any            // deep-merged across all layers
	Permissions   map[string]any            // merged allow/deny lists
	Hooks         map[string]any            // accumulated (PascalCase canonical keys)
	MCP           map[string]any            // deep-merged MCP config
	ContentBlocks map[string]string         // concatenated content blocks (loop, worktree-include)
	ConfigBlocks  map[string]map[string]any // deep-merged config blocks (environment)
	Categories    map[string][]CategoryFile // per-category files (last-writer-wins per relpath)
	Bin           []BinFile                 // bin/ overlay files
	AgentSpecific map[string]map[string]any // agent-only settings, keyed by agent name
	Attribution   *CanonicalAttribution     // portable attribution config (last-writer-wins)
	Sandbox       *schema.SandboxConfig     // portable sandbox config (last-writer-wins)

	// SkipSettings signals that the writer should not produce a settings file.
	SkipSettings bool
	// SkipInstructions signals that the writer should not produce an instruction file.
	SkipInstructions bool
}

// SharedResult holds cross-functional output data produced by the shared
// compilation phase and consumed by per-agent compilation.
type SharedResult struct {
	UniversalInstructions string            // Compiled shared instruction content
	UniversalMCP          map[string]any    // Shared MCP config
	AgentAddenda          map[string]string // agent name → instruction delta
	FileRecords           []manifest.FileRecord
}

// GovernanceOutput holds governance decisions made during compilation.
type GovernanceOutput struct {
	MCPPolicy                *schema.MCPPolicy           // accumulated MCP governance policy
	PermissionsPolicy        *schema.PermissionsPolicy   // accumulated permissions merge policy
	PermPolicySetBy          string                      // layer that set the permissions policy
	ForceLocked              []manifest.ForceLockRecord  // all registered force-locks
	Overlays                 map[string][]string         // output path → layer provenance
	OverrideConflicts        []manifest.OverrideConflict // override conflicts tracked during compilation
	HooksFilteredByForceLock bool                        // true if hooks were filtered by settings force-lock
}

// CompilationTracking holds records for manifest and dependency tracking.
type CompilationTracking struct {
	Records       []manifest.FileRecord         // manifest file records
	InstalledDeps []manifest.InstalledDepRecord // resolved deps
	Variables     map[string]string             // final variable map
}

// CompilationMetadata holds layer and project metadata.
type CompilationMetadata struct {
	Name        string
	Description string
	Repos       map[string]schema.Repo
	LayerNames  map[string]string
}

// AccumulatedResult is the final output of the accumulation pipeline, consumed
// by AssembleAgentOutput and then by Writers. It contains all content fully
// merged, variables substituted, and governance applied.
//
// Lifecycle: produced by AccumulationState.Result() after Finalize(),
// consumed by AssembleAgentOutput() and Writer.Write(). Named fields
// group data by concern: Content (what to write), Governance (policy
// decisions), Tracking (manifest records), Metadata (project info).
//
// This type is part of Keel's public API for writer adapter authors.
type AccumulatedResult struct {
	Content    CompilationContent
	Governance GovernanceOutput
	Tracking   CompilationTracking
	Metadata   CompilationMetadata
}

// Writer serializes an AccumulatedResult to disk in a specific agent's
// native format. Writers handle format-specific transforms (hook key casing,
// permission suffixes, instruction wrapping, output paths) but contain no
// merging, force-lock, or variable substitution logic.
type Writer interface {
	// Write serializes the accumulated result to outputDir.
	// The profile provides scope-specific output paths.
	// Returns file records for the manifest.
	Write(result *AccumulatedResult, profile AgentProfile, outputDir string, opts CompileOptions) ([]manifest.FileRecord, error)
}

// NativeReader reads a native agent configuration directory and produces
// a normalized LayerIR. Implementations handle agent-specific file layouts,
// naming conventions, and format differences.
type NativeReader interface {
	// AgentName returns the agent format this reader handles (e.g., "claude").
	AgentName() string

	// Read reads the native configuration at layerPath and returns a
	// normalized IR. Missing files produce nil/empty fields, not errors.
	Read(layerPath string) (*LayerIR, error)
}
