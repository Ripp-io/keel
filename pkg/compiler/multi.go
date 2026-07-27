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

	// Run shared phase for each outputDir group (always, for forward compatibility)
	for outputDir, groupAgents := range outputDirGroups {
		sharedOpts := opts
		sharedOpts.layerCache = cache
		sharedOpts.PreInstalledDeps = preInstalled

		shared, err := WriteSharedOutput(layers, outputDir, agents, sharedOpts)
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
