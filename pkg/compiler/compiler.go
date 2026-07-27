// Package compiler compiles config layers into tool-native output.
package compiler

import (
	"fmt"
	"log/slog"
	"os"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// Scope controls whether compilation targets project-level or user-level output.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

// CompileOptions control compilation behavior. Passed to Compile() and
// CompileWithProfile(). User-facing flags (Distributable, DryRun, etc.)
// are set by the CLI; internal fields (layerCache, PreInstalledDeps) are
// set by the compiler for multi-agent compilation.
type CompileOptions struct {
	// Distributable preserves _keel policies and frontmatter force: fields so
	// the output can be consumed as an input layer by a downstream compile.
	Distributable bool

	// Name overrides the project name (set via --distributable "name").
	Name string

	// DryRun runs the full compilation pipeline but the caller discards the
	// output directory. The returned CompileResult is used to display a plan
	// summary instead of writing final output.
	DryRun bool

	// Scope selects project-level (default) or user-level output paths.
	Scope Scope

	// Offline prevents network access for remote FileRef resolution.
	// When true, only cached remote files are used; missing files cause errors.
	Offline bool

	// NoPrune skips automatic removal of stale files from previous compilations.
	NoPrune bool

	// AllowInstall permits dep install/download commands to execute during
	// compilation. Without this flag, deps are recorded but not installed.
	AllowInstall bool

	// RequireChecksums makes missing SHA256 checksums on downloads a hard error.
	// When false (default), missing checksums emit a warning.
	RequireChecksums bool

	// MaxRemoteLayers limits the number of remote layers (git://, https://)
	// that can be resolved during compilation. Default: 50.
	// Set to 0 to use the default. Set to -1 for unlimited (not recommended).
	MaxRemoteLayers int

	// Vars provides external template variables from CLI flags, config files,
	// and var-files. Applied after layer processing with higher precedence
	// than layer-defined variables. Auto-injected vars (keel.agent, keel.scope)
	// cannot be overridden.
	Vars map[string]string

	// PreInstalledDeps tracks deps already processed by a previous adapter
	// pass. Maps dep name to the method recorded (e.g. "install", "download",
	// "skipped"). Prevents duplicate installs and warnings when compiling
	// for multiple targets.
	PreInstalledDeps map[string]string

	// SharedResult holds cross-functional output data from a shared compilation
	// phase. Populated by shared-phase writers, consumed by per-agent compilation.
	SharedResult *SharedResult

	// ForceInstructions overwrites user-edited instruction files without warning.
	// Set by --force on the CLI.
	ForceInstructions bool

	// layerCache is an internal field used by CompileMulti to share
	// layer reads across agents. Not exposed to external callers.
	layerCache *LayerCache
}

// CompileFunc is the signature for agent-specific compilation functions.
type CompileFunc func(layers []config.Layer, outputDir string, opts CompileOptions) (*CompileResult, error)

// CompileOutput holds generated files and overlays.
type CompileOutput struct {
	Agent string
	Files []manifest.FileRecord

	// OutputRoot is the agent's config directory relative to outputDir
	// (e.g. ".claude" or ".cursor"). Set by each adapter so the compiler
	// knows where to write .keel.json.
	OutputRoot string

	// Overlays maps output file paths to the ordered list of layers that wrote them.
	// Only populated for overlay-category files (commands, skills, agents, etc.).
	Overlays map[string][]string
}

// CompileGovernance holds policy decisions made during compilation.
type CompileGovernance struct {
	ForceLocked []manifest.ForceLockRecord

	// InstructionConflicts lists instruction output paths that were skipped
	// because the existing file was user-edited. Empty when --force is used
	// or when no conflicts exist.
	InstructionConflicts []string

	// Record captures compile-time policy decisions for compliance reporting.
	Record *manifest.GovernanceRecord
}

// CompileDependencies holds resolved dependencies.
type CompileDependencies struct {
	// InstalledDeps records resolved external binary dependencies.
	InstalledDeps []manifest.InstalledDepRecord

	// ManagedMCPServers lists server names that Keel manages in the MCP output.
	// Populated when the profile uses MCPMergeInto mode.
	ManagedMCPServers []string
}

// CompileState holds compilation state and metrics.
type CompileState struct {
	// Pruned holds the results of stale file cleanup from the previous compilation.
	// Nil when pruning was skipped (dry-run, first compile, --no-prune).
	Pruned *manifest.PruneResult

	// EffectiveState captures the effective configuration state at compile time.
	EffectiveState *manifest.EffectiveState

	// Stats holds compiled content metrics (token estimates, counts, sizes).
	// Populated when stats are requested. Nil when not computed.
	Stats *CompileStats
}

// CompileTracking holds metadata and tracking records.
type CompileTracking struct {
	// Variables holds the final resolved variable map from compilation.
	// Includes auto-injected variables (keel.agent, keel.scope, etc.),
	// layer-defined variables, and CLI overrides.
	Variables map[string]string

	// Metadata accumulated from keel.yml layers during compilation.
	Name        string
	Description string
	Repos       map[string]schema.Repo
	LayerNames  map[string]string // layer path -> name from keel.yml
}

// CompileResult is the final output of a compilation, returned by Compile()
// and CompileWithProfile(). Named fields group results by concern:
// Output (files written), Governance (policy decisions), Dependencies
// (installed deps), State (prune results, effective config, stats),
// and Tracking (variables, metadata, manifest records).
//
// This type is part of Keel's public API.
type CompileResult struct {
	Output       CompileOutput
	Governance   CompileGovernance
	Dependencies CompileDependencies
	State        CompileState
	Tracking     CompileTracking
}

// Compile resolves the agent, delegates compilation, and writes .keel.json.
func Compile(agent string, layers []config.Layer, outputDir string, opts CompileOptions) (*CompileResult, error) {
	compiler, err := getCompiler(agent)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory %q: %w", outputDir, err)
	}

	result, err := compiler(layers, outputDir, opts)
	if err != nil {
		return nil, fmt.Errorf("compiling for %s: %w", agent, err)
	}

	if !opts.NoPrune {
		pruned, pruneErr := manifest.PruneStaleOutput(outputDir, result.Output.OutputRoot, result.Output.Files, result.Output.Overlays)
		if pruneErr != nil {
			slog.Warn("pruning stale output failed", "error", pruneErr)
		} else if pruned != nil {
			result.State.Pruned = pruned
			for _, f := range pruned.Removed {
				slog.Info("pruned stale file", "path", f)
			}
		}
	}

	if err := writeKeelState(result, layers, outputDir, opts); err != nil {
		return nil, fmt.Errorf("writing .keel.json: %w", err)
	}

	return result, nil
}

var compilers = make(map[string]CompileFunc)

// RegisterCompiler registers an adapter compiler function for the named agent.
// Call this from init() in each adapter file.
func RegisterCompiler(name string, fn CompileFunc) {
	compilers[name] = fn
}

func getCompiler(agent string) (CompileFunc, error) {
	if fn, ok := compilers[agent]; ok {
		return fn, nil
	}
	return nil, fmt.Errorf("unknown agent %q (supported: %v)", agent, config.AgentNames())
}
