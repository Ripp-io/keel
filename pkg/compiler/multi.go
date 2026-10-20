package compiler

import (
	"fmt"
	"log/slog"
	"sort"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// CompileMulti compiles for multiple agents, sharing layer reads across agents.
// DSL layers (FormatKeel with keel.yml) are read from disk only once and cached
// for reuse. Native-format and convention layers are read per-agent since they
// may produce different IRs.
//
// Uses a two-phase approach: shared phase produces AGENTS.md and .mcp.json once,
// then per-agent compilation writes shims (e.g., CLAUDE.md with @AGENTS.md) and
// agent-specific config.
//
// Returns results keyed by agent name.
func CompileMulti(agents map[string]string, layers []config.Layer, opts CompileOptions) (map[string]*CompileResult, error) {
	cache := NewLayerCache()
	results := make(map[string]*CompileResult, len(agents))

	// Track installed deps across agents. Same dep should only be installed once
	// since all agents share .keel/bin/ (or global bin dir for global deps).
	preInstalled := make(map[string]string)

	// Sort agent names for deterministic output order
	agentNames := make([]string, 0, len(agents))
	for name := range agents {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)

	// Group agents by outputDir for shared phase
	outputDirGroups := make(map[string][]string)
	for _, agentName := range agentNames {
		outputDir := agents[agentName]
		outputDirGroups[outputDir] = append(outputDirGroups[outputDir], agentName)
	}

	// Track SharedResult per outputDir
	sharedResults := make(map[string]*SharedResult)

	// Run shared phase for each outputDir group, but only when at least one
	// agent in the group uses shared output (SharedInstructionOutput or
	// SharedMCPOutput). Packaging-only targets like agentplugins don't need
	// the shared phase — skipping it avoids writing an unused AGENTS.md.
	for outputDir, groupAgents := range outputDirGroups {
		if !groupNeedsSharedPhase(groupAgents, opts.Scope) {
			slog.Debug("skipping shared phase — no agent needs shared output", "outputDir", outputDir)
			continue
		}

		sharedOpts := opts
		sharedOpts.layerCache = cache
		sharedOpts.PreInstalledDeps = preInstalled

		// Resolve shared instruction path from agent profiles for this output group.
		// Agents that actively reference the shared file (non-empty InstructionShimTemplate)
		// determine the path. Falls back to "AGENTS.md" if no profile provides one.
		sharedInstrPath := resolveSharedInstructionPath(groupAgents, opts.Scope)

		shared, err := WriteSharedOutput(layers, outputDir, sharedInstrPath, agents, sharedOpts)
		if err != nil {
			return nil, fmt.Errorf("writing shared output to %s: %w", outputDir, err)
		}
		sharedResults[outputDir] = shared

		slog.Debug("shared phase complete", "outputDir", outputDir, "agents", len(groupAgents))
	}

	// Per-agent compilation with SharedResult
	for _, agentName := range agentNames {
		outputDir := agents[agentName]

		// Create agent-specific options with shared cache, pre-installed deps, and SharedResult
		agentOpts := opts
		agentOpts.PreInstalledDeps = preInstalled
		agentOpts.layerCache = cache
		if shared, ok := sharedResults[outputDir]; ok {
			agentOpts.SharedResult = shared
		}

		slog.Debug("compiling with cache", "agent", agentName, "output", outputDir, "layers", len(layers), "hasShared", agentOpts.SharedResult != nil)

		// Use the normal Compile function, which will use the cache via opts.layerCache
		result, err := Compile(agentName, layers, outputDir, agentOpts)
		if err != nil {
			return nil, fmt.Errorf("compiling for %s: %w", agentName, err)
		}

		// Track all deps processed by this agent for subsequent agents
		for _, dep := range result.Dependencies.InstalledDeps {
			preInstalled[dep.Name] = dep.Method
		}

		results[agentName] = result
	}

	return results, nil
}

// groupNeedsSharedPhase returns true if any agent in the group uses shared
// output files (SharedInstructionOutput or SharedMCPOutput). When all agents
// produce standalone output (like agentplugins), the shared phase is skipped.
func groupNeedsSharedPhase(agents []string, scope Scope) bool {
	for _, agent := range agents {
		if p, ok := GetProfile(agent, scope); ok {
			if p.SharedInstructionOutput != "" || p.SharedMCPOutput != "" {
				return true
			}
		}
	}
	return false
}

// resolveSharedInstructionPath determines where AGENTS.md should be written
// by inspecting agent profiles. Agents with a non-empty InstructionShimTemplate
// actively reference the shared file, so their SharedInstructionOutput is used.
func resolveSharedInstructionPath(agents []string, scope Scope) string {
	for _, agent := range agents {
		if p, ok := GetProfile(agent, scope); ok {
			// Only consider agents that actually reference the shared file via a shim
			if p.InstructionShimTemplate != "" && p.SharedInstructionOutput != "" {
				return p.SharedInstructionOutput
			}
		}
	}
	// Fallback: check any agent's SharedInstructionOutput
	for _, agent := range agents {
		if p, ok := GetProfile(agent, scope); ok && p.SharedInstructionOutput != "" {
			return p.SharedInstructionOutput
		}
	}
	return "AGENTS.md"
}
