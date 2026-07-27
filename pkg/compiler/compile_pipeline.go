package compiler

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

// CompileWithProfile runs the IR-centric compilation pipeline:
// FOLD → GOVERN → TRANSFORM → RESOLVE(Finalize) → ASSEMBLE → OUTPUT(Write).
// See section_guide.go for stage definitions.
func CompileWithProfile(p AgentProfile, layers []config.Layer, outputDir string, opts CompileOptions) (*CompileResult, error) {
	// Guard against double-nesting: if outputDir ends with the agent's
	// output root (e.g., "output: claude: .claude"), the writer would join
	// outputDir + OutputRoot and produce .claude/.claude/. This is always
	// a user error — the output path should be the project root, not the
	// agent config directory.
	if p.OutputRoot != "" && filepath.Base(filepath.Clean(outputDir)) == p.OutputRoot {
		return nil, fmt.Errorf("output path %q already ends with the agent directory %q — this would create a nested %s/%s/; use the parent directory instead (e.g., output: %s: .)",
			outputDir, p.OutputRoot, p.OutputRoot, p.OutputRoot, p.Name)
	}

	// Compute absolute output dir for auto-vars.
	absOutputDir := outputDir
	if !filepath.IsAbs(absOutputDir) {
		if abs, err := filepath.Abs(absOutputDir); err == nil {
			absOutputDir = abs
		}
	}

	// Create base output directories.
	agentDir := filepath.Join(outputDir, p.OutputRoot)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory %q: %w", agentDir, err)
	}

	// Load old manifest for conflict detection.
	oldManifest, _ := manifest.LoadKeelState(agentDir)

	// Check if implicit layer exists and prepend it to layer list.
	implicitLayerDir := filepath.Join(outputDir, ".keel", p.OutputRoot)
	if info, err := os.Stat(implicitLayerDir); err == nil && info.IsDir() {
		implicitLayer := config.Layer{
			Path:   implicitLayerDir,
			Format: config.LayerFormat(p.Name),
			Type:   "implicit",
		}
		// Prepend — implicit layer has lowest priority
		layers = append([]config.Layer{implicitLayer}, layers...)
	}

	// Build fold config from profile.
	foldConfig := FoldConfig{
		Categories: p.Categories,
	}

	// Initialize accumulation state.
	state := NewAccumulationState(foldConfig, opts, absOutputDir, len(layers))

	// Read → Fold each layer.
	//
	// Error semantics for layer reads:
	//   - selectReader error: "no reader knows this format" — layer is skipped
	//     with a warning because it might be a non-Keel directory in the layer list.
	//   - reader.Read error: "reader matched the format but failed to parse" —
	//     hard error because the layer is malformed and should not be silently ignored.
	//   - cache read errors inherit these semantics — hard errors propagate.
	myFormat := config.LayerFormat(p.Name)
	for _, layer := range layers {
		var ir *LayerIR
		var layerLabel string
		var err error

		// Use cache when available (CompileMulti provides one), otherwise read directly.
		if opts.layerCache != nil {
			ir, layerLabel, err = opts.layerCache.ReadOrCache(layer, p, myFormat, opts.Offline)
		} else {
			layerLabel = filepath.Base(layer.Path)
			reader, readErr := selectReader(layer, p, myFormat, opts.Offline)
			if readErr != nil {
				slog.Warn("skipping layer", "path", layer.Path, "error", readErr)
				continue
			}
			ir, err = reader.Read(layer.Path)
		}

		if err != nil {
			return nil, fmt.Errorf("reading layer %q: %w", layer.Path, err)
		}

		if err := state.Fold(ir, layerLabel); err != nil {
			return nil, fmt.Errorf("merging layer %q: %w", layer.Path, err)
		}
	}

	// Run GOVERN → TRANSFORM → RESOLVE → RESULT.
	accumulated, err := runPostFoldPipeline(state, &p, outputDir, opts)
	if err != nil {
		return nil, err
	}

	// --- ASSEMBLE stage ---
	// Post-pipeline operations that bridge RESOLVE and OUTPUT. These run
	// on the AccumulatedResult (not AccumulationState) and prepare the
	// final data shape for the writer:
	//   1. AssembleAgentOutput: merge settings + agent-specific + hooks into final map
	//   2. detectInstructionConflicts: guard — skip user-edited instruction files
	//   3. evaluateSettingsSkip: guard — suppress empty settings output
	//   4. detectFirstCompileConflicts: guard — prevent overwriting pre-existing files

	// Assemble: agent-specific output transforms and conflict detection.
	if err := runAssemblyStage(accumulated, p, outputDir, opts, oldManifest); err != nil {
		return nil, err
	}

	// When SharedResult is present, replace instruction content with shim.
	if opts.SharedResult != nil {
		shimContent := buildInstructionShim(p, opts.SharedResult, accumulated.Tracking.Variables)
		accumulated.Content.Instructions = shimContent
	}

	// Write output using the agent-specific writer.
	writer, err := GetWriter(p.Name)
	if err != nil {
		return nil, fmt.Errorf("getting writer for %s: %w", p.Name, err)
	}
	writerRecords, err := writer.Write(accumulated, p, outputDir, opts)
	if err != nil {
		return nil, fmt.Errorf("writing output for %s: %w", p.Name, err)
	}

	// Include shared file records in the result when SharedResult is present.
	// Per-agent writer records take precedence over shared records for the
	// same output path (e.g., .mcp.json written by both shared and per-agent).
	var allRecords []manifest.FileRecord
	if opts.SharedResult != nil {
		writerPaths := make(map[string]bool, len(writerRecords))
		for _, r := range writerRecords {
			writerPaths[r.Output] = true
		}
		for _, r := range opts.SharedResult.FileRecords {
			if !writerPaths[r.Output] {
				allRecords = append(allRecords, r)
			}
		}
	}
	allRecords = append(allRecords, writerRecords...)

	return buildCompileResult(accumulated, p, allRecords), nil
}

// runAssemblyStage applies agent-specific output assembly and conflict
// detection. This is the ASSEMBLE stage of the pipeline — it transforms the
// accumulated result into agent-native shape and guards against overwriting
// user-edited files.
func runAssemblyStage(accumulated *AccumulatedResult, p AgentProfile, outputDir string, opts CompileOptions, oldManifest *manifest.CompiledManifest) error {
	// Assemble agent-specific output (settings, hooks key transform).
	AssembleAgentOutput(accumulated, p)

	// Detect instruction conflicts (user-edited files).
	detectInstructionConflicts(accumulated, oldManifest, p, outputDir, opts.ForceInstructions)

	// Evaluate whether to skip settings output for agents with separate settings files.
	evaluateSettingsSkip(accumulated, p)

	// On first compile, guard against overwriting pre-existing files.
	if oldManifest == nil && !opts.ForceInstructions {
		if err := detectFirstCompileConflicts(accumulated, p, outputDir, opts); err != nil {
			return err
		}
	}

	return nil
}

// buildCompileResult constructs the final CompileResult from the accumulated
// output, profile, and writer records. Pure data mapping — no side effects.
func buildCompileResult(accumulated *AccumulatedResult, p AgentProfile, writerRecords []manifest.FileRecord) *CompileResult {
	result := &CompileResult{
		Output: CompileOutput{
			Agent:      p.Name,
			OutputRoot: p.OutputRoot,
			Files:      append(accumulated.Tracking.Records, writerRecords...),
			Overlays:   accumulated.Governance.Overlays,
		},
		Governance: CompileGovernance{
			ForceLocked: accumulated.Governance.ForceLocked,
		},
		Dependencies: CompileDependencies{
			InstalledDeps: accumulated.Tracking.InstalledDeps,
		},
		Tracking: CompileTracking{
			Variables:   accumulated.Tracking.Variables,
			Name:        accumulated.Metadata.Name,
			Description: accumulated.Metadata.Description,
			Repos:       accumulated.Metadata.Repos,
			LayerNames:  accumulated.Metadata.LayerNames,
		},
	}

	// Build governance record from accumulated data.
	result.Governance.Record = buildGovernanceRecord(accumulated)

	// Build effective state from accumulated result.
	result.State.EffectiveState = BuildEffectiveState(accumulated)

	// Compute stats using the already-built effective state.
	result.State.Stats = computeStats(accumulated, result.State.EffectiveState)

	// Track instruction conflicts for CLI reporting.
	if accumulated.Content.SkipInstructions {
		result.Governance.InstructionConflicts = append(result.Governance.InstructionConflicts, p.InstructionOutput)
	}

	// Track managed MCP server names for merge-into files.
	result.Dependencies.ManagedMCPServers = trackManagedMCPServers(accumulated, p)

	return result
}

// buildGovernanceRecord constructs a GovernanceRecord from accumulated data
// when any governance decisions were made (permissions policy, MCP policy,
// override conflicts, or hook lock strategy). Returns nil if no governance
// occurred.
func buildGovernanceRecord(accumulated *AccumulatedResult) *manifest.GovernanceRecord {
	hasGov := accumulated.Governance.PermissionsPolicy != nil ||
		accumulated.Governance.MCPPolicy != nil ||
		len(accumulated.Governance.OverrideConflicts) > 0 ||
		accumulated.Governance.HooksFilteredByForceLock

	if !hasGov {
		return nil
	}

	gov := &manifest.GovernanceRecord{
		OverrideConflicts: accumulated.Governance.OverrideConflicts,
	}
	if accumulated.Governance.PermissionsPolicy != nil {
		gov.PermissionsPolicy = &manifest.PermissionsPolicyRecord{
			Merge: accumulated.Governance.PermissionsPolicy.Merge,
			SetBy: accumulated.Governance.PermPolicySetBy,
		}
	}
	if accumulated.Governance.MCPPolicy != nil {
		gov.MCPPolicy = accumulated.Governance.MCPPolicy
	}
	if accumulated.Governance.HooksFilteredByForceLock {
		gov.HookLockStrategy = "settings-locked"
	}

	return gov
}

// trackManagedMCPServers extracts MCP server names from accumulated MCP data
// for merge-into files (user-scope Claude where mcpServers are merged into
// an existing file). Returns a sorted list of server names.
func trackManagedMCPServers(accumulated *AccumulatedResult, profile AgentProfile) []string {
	if !profile.MCPMergeInto || accumulated.Content.MCP == nil {
		return nil
	}

	servers, ok := accumulated.Content.MCP["mcpServers"].(map[string]any)
	if !ok {
		return nil
	}

	var names []string
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// buildInstructionShim constructs the agent's instruction shim file content
// when SharedResult is present. The shim combines the profile's template
// (e.g., "@AGENTS.md\n") with the agent-specific addenda from SharedResult.
//
// Agent addenda still contain conditional markers from the shared phase (which
// skips conditional evaluation). These are evaluated here with the per-agent
// variable map (which includes keel.agent).
func buildInstructionShim(p AgentProfile, shared *SharedResult, vars map[string]string) string {
	// Start with the shim template (e.g., "@AGENTS.md\n" for Claude)
	content := p.InstructionShimTemplate

	// Append agent-specific addenda if present
	if addendum, ok := shared.AgentAddenda[p.Name]; ok && addendum != "" {
		// Evaluate conditionals in addenda (e.g., compound conditions like
		// "keel.agent == 'claude' && keel.scope == 'project'")
		evaluated := string(merge.ProcessConditionals([]byte(addendum), vars))
		evaluated = strings.TrimRight(evaluated, "\n\r\t ")
		if evaluated != "" {
			if content != "" {
				content += "\n"
			}
			content += evaluated + "\n"
		}
	}

	return content
}

// selectReader chooses the appropriate NativeReader for a layer based on
// its format and the target agent.
func selectReader(layer config.Layer, p AgentProfile, myFormat config.LayerFormat, offline bool) (NativeReader, error) {
	switch {
	case layer.Format == config.FormatKeel:
		// DSL reader for all keel-format layers. When no keel.yml exists,
		// processDSLLayer returns compiled defaults (instructions/, commands/,
		// skills/, etc.) so convention layers work without a manifest.
		return &KeelDSLReader{Offline: offline}, nil

	case layer.Format == myFormat:
		// Native layer for this agent. Native readers check both the
		// dot-directory and root for categories. AgentBuild prunes
		// categories not supported by the target profile.
		reader, err := GetReader(string(myFormat))
		if err != nil {
			return nil, fmt.Errorf("no reader for agent %q: %w", p.Name, err)
		}
		return reader, nil

	case config.IsAgentFormat(layer.Format):
		// Foreign agent format — use foreign reader for transpilation.
		// Reader produces a complete IR; AgentBuild prunes categories
		// not supported by the target profile.
		reader, err := GetReader(string(layer.Format))
		if err != nil {
			return nil, fmt.Errorf("no reader for foreign agent %q: %w", layer.Format, err)
		}
		return reader, nil

	default:
		return nil, fmt.Errorf("unknown layer format %q", layer.Format)
	}
}

// runPostFoldPipeline runs the GOVERN → TRANSFORM → RESOLVE pipeline stages
// on a folded AccumulationState. This is the shared core that both
// CompileWithProfile and WriteSharedOutput call after their respective
// Fold loops complete.
//
// When profile is non-nil, agent-specific governance and transforms are
// applied (per-agent compilation). When profile is nil, only universal
// governance runs (shared compilation phase).
func runPostFoldPipeline(state *AccumulationState, profile *AgentProfile, outputDir string, opts CompileOptions) (*AccumulatedResult, error) {
	// GOVERN — universal governance always runs.
	state.ApplyGovernance()
	if profile != nil {
		state.ApplyAgentGovernance(*profile)
	}

	// TRANSFORM — agent-specific transforms only when compiling for a specific agent.
	if profile != nil {
		state.TransformForAgent(*profile)
		state.EmitAgentWarnings(*profile)
	}

	// Process deps (side effect: install/download binaries).
	if err := state.ProcessDeps(outputDir, opts.AllowInstall, opts.RequireChecksums, opts.PreInstalledDeps); err != nil {
		return nil, fmt.Errorf("processing deps: %w", err)
	}

	// RESOLVE — variable substitution, snippet resolution, conditional evaluation.
	if err := state.Finalize(opts.Vars); err != nil {
		return nil, fmt.Errorf("finalizing compilation: %w", err)
	}

	return state.Result(), nil
}
