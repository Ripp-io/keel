package compiler

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
	"gopkg.in/yaml.v3"
)

// === RESOLVE STAGE ===

// Finalize applies final processing after all layers are folded: external
// variables, MCP condition filtering, snippet resolution, conditional
// evaluation, variable substitution, and frontmatter stripping.
func (s *AccumulationState) Finalize(externalVars map[string]string) error {
	if s.phase < phaseGoverned {
		panic(fmt.Sprintf("Finalize called in phase %q — ApplyGovernance must be called first", phaseNames[s.phase]))
	}
	s.phase = phaseFinalized
	// 1. External variables: merge, protecting auto-vars.
	for k, v := range externalVars {
		if isAutoVar(k) {
			slog.Warn("cannot override auto-injected variable", "var", k)
			continue
		}
		s.variables[k] = v
	}

	// 2. Inject keel.name if accumulated from layers.
	if s.name != "" {
		s.variables[varKeelName] = s.name
	}

	// 3. Filter MCP conditions and merge contributions.
	// This happens HERE (after all variables are resolved) so that MCP
	// conditions can use layer-defined and CLI-provided variables, not just
	// auto-vars. Previously this was in AgentBuild, which meant conditions
	// only had access to keel.agent.
	s.FilterAndMergeMCP()

	// 4. Category files: resolve snippets, conditionals, variable substitution,
	//    and optionally strip frontmatter.
	for catName, files := range s.categories {
		for i, file := range files {
			if file.IsDir {
				continue
			}

			content := file.Content

			// Resolve snippet includes.
			if len(s.snippets) > 0 {
				resolved, err := merge.ResolveSnippets(content, s.snippets)
				if err != nil {
					return fmt.Errorf("resolving snippets in %s/%s: %w", catName, file.RelPath, err)
				}
				content = resolved
			}

			// Evaluate conditional blocks (skipped in shared phase so
			// SplitAgentConditionals can partition them first).
			if len(s.variables) > 0 && !s.skipConditionals {
				content = merge.ProcessConditionals(content, s.variables)
			}

			// Substitute variables.
			if len(s.variables) > 0 {
				content = merge.SubstituteVariables(content, s.variables)
			}

			// Parse and strip frontmatter (unless distributable).
			if !s.distributable && merge.IsMarkdownFile(file.RelPath) {
				_, cleaned, err := merge.ParseAndCleanFrontmatter(content)
				if err != nil {
					slog.Warn("frontmatter parse failed, passing content through unchanged", "file", file.RelPath, "category", catName, "error", err)
				} else {
					content = cleaned
				}
			}

			files[i].Content = content
		}
		s.categories[catName] = files
	}

	// 5. Instructions: concatenate with headers, resolve snippets,
	//    conditionals, and substitute variables.
	if len(s.instructions) > 0 {
		// Count distinct layers contributing instructions.
		uniqueLayers := make(map[string]bool)
		for _, block := range s.instructions {
			if block.LayerLabel != "" {
				uniqueLayers[block.LayerLabel] = true
			}
		}
		multiLayer := len(uniqueLayers) > 1

		var instrBuilder strings.Builder
		prevLabel := ""
		for i, block := range s.instructions {
			if i > 0 {
				instrBuilder.WriteString("\n\n")
			}
			// Add section separator when multiple layers contribute.
			// One header per layer (skip consecutive blocks from the same layer).
			if multiLayer && block.LayerLabel != "" && block.LayerLabel != prevLabel {
				instrBuilder.WriteString(fmt.Sprintf("# --- %s ---\n\n", block.LayerLabel))
			}
			prevLabel = block.LayerLabel
			text := strings.TrimRight(string(merge.StripFrontmatter([]byte(block.Content))), "\n\r\t ")
			instrBuilder.WriteString(text)
			instrBuilder.WriteString("\n")
		}
		instrContent := []byte(instrBuilder.String())

		// Resolve snippets.
		if len(s.snippets) > 0 {
			resolved, err := merge.ResolveSnippets(instrContent, s.snippets)
			if err != nil {
				return fmt.Errorf("resolving snippets in instructions: %w", err)
			}
			instrContent = resolved
		}

		// Evaluate conditionals (skipped in shared phase so
		// SplitAgentConditionals can partition them first).
		if len(s.variables) > 0 && !s.skipConditionals {
			instrContent = merge.ProcessConditionals(instrContent, s.variables)
		}

		// Substitute variables.
		if len(s.variables) > 0 {
			instrContent = merge.SubstituteVariables(instrContent, s.variables)
		}

		// Store as the finalized instruction string. Preserve a single trailing newline for clean file output.
		finalInstr := strings.TrimRight(string(instrContent), "\n\r\t ")
		if finalInstr != "" {
			finalInstr += "\n"
		}
		s.instructions = []InstructionBlock{{
			Content: finalInstr,
		}}
	}

	// ContentBlocks (loop, worktree-include) are concatenated raw content.
	// They intentionally skip snippet resolution, conditional evaluation,
	// and variable substitution. If a user puts ${keel:varname} in a
	// content block file, it passes through unchanged. This is by design —
	// content blocks are agent-specific auxiliary content, not governance
	// instructions.

	// 6. Maps: substitute variables in all accumulated JSON structures.
	s.substituteVariablesInMaps()

	return nil
}

// substituteVariablesInMaps applies ${keel:name} variable substitution to all
// accumulated JSON map structures. This is the single entry point for map-based
// variable substitution — when adding a new map field to AccumulationState that
// should support variables, add it here.
//
// Category file content and instructions have their own substitution calls
// within their processing pipelines (which also handle snippets and conditionals).
// ContentBlocks are intentionally excluded — they pass through unchanged.
func (s *AccumulationState) substituteVariablesInMaps() {
	if len(s.variables) == 0 {
		return
	}
	if len(s.settings) > 0 {
		s.settings = merge.SubstituteVariablesInMap(s.settings, s.variables)
	}
	if len(s.permissions) > 0 {
		s.permissions = merge.SubstituteVariablesInMap(s.permissions, s.variables)
	}
	if len(s.hooks) > 0 {
		s.hooks = merge.SubstituteVariablesInMap(s.hooks, s.variables)
	}
	for name, config := range s.configBlocks {
		if len(config) > 0 {
			s.configBlocks[name] = merge.SubstituteVariablesInMap(config, s.variables)
		}
	}
	if len(s.mcp) > 0 {
		s.mcp = merge.SubstituteVariablesInMap(s.mcp, s.variables)
	}
	for agent, data := range s.agentSpecific {
		if len(data) > 0 {
			s.agentSpecific[agent] = merge.SubstituteVariablesInMap(data, s.variables)
		}
	}
}

// Result extracts the final accumulated state as an AccumulatedResult,
// ready for consumption by a Writer.
func (s *AccumulationState) Result() *AccumulatedResult {
	// Build the final instruction string from finalized blocks.
	var instructions string
	if len(s.instructions) > 0 {
		instructions = s.instructions[0].Content
	}

	// Copy variables to avoid sharing the mutable map.
	vars := make(map[string]string, len(s.variables))
	for k, v := range s.variables {
		vars[k] = v
	}

	// Build categories — only include non-empty categories.
	categories := make(map[string][]CategoryFile, len(s.categories))
	for k, v := range s.categories {
		if len(v) > 0 {
			categories[k] = v
		}
	}

	// Nil-out empty maps for cleaner output.
	settings := nilIfEmpty(s.settings)
	permissions := nilIfEmpty(s.permissions)
	hooks := nilIfEmpty(s.hooks)
	mcp := nilIfEmpty(s.mcp)

	// Build ContentBlocks from accumulated blocks using join separators.
	contentBlocks := make(map[string]string)
	for name, blocks := range s.contentBlocks {
		if len(blocks) == 0 {
			continue
		}
		separator := s.contentBlockSeparators[name]
		if separator == "" {
			separator = "\n\n" // fallback
		}
		var parts []string
		for _, block := range blocks {
			parts = append(parts, strings.TrimSpace(block.Content))
		}
		contentBlocks[name] = strings.Join(parts, separator)
	}
	if len(contentBlocks) == 0 {
		contentBlocks = nil
	}

	// Build ConfigBlocks — only include non-empty blocks.
	configBlocks := make(map[string]map[string]any)
	for name, config := range s.configBlocks {
		if len(config) > 0 {
			configBlocks[name] = config
		}
	}
	if len(configBlocks) == 0 {
		configBlocks = nil
	}

	// Agent-specific: nil-out empty inner maps.
	var agentSpecific map[string]map[string]any
	for k, v := range s.agentSpecific {
		if len(v) > 0 {
			if agentSpecific == nil {
				agentSpecific = make(map[string]map[string]any)
			}
			agentSpecific[k] = v
		}
	}

	return &AccumulatedResult{
		Content: CompilationContent{
			Instructions:  instructions,
			Settings:      settings,
			Permissions:   permissions,
			Hooks:         hooks,
			MCP:           mcp,
			ContentBlocks: contentBlocks,
			ConfigBlocks:  configBlocks,
			Categories:    categories,
			Bin:           s.bin,
			HookScripts:   s.hookScripts,
			AgentSpecific: agentSpecific,
			Attribution:   s.attribution,
			Sandbox:       s.sandbox,
		},
		Governance: GovernanceOutput{
			MCPPolicy:                s.mcpPolicy,
			PermissionsPolicy:        s.permPolicy,
			PermPolicySetBy:          s.permPolicySetBy,
			ForceLocked:              s.force.locks(),
			Overlays:                 s.prefixedOverlays(),
			OverrideConflicts:        s.overrideConflicts,
			HooksFilteredByForceLock: s.settingsForceLockedAt >= 0,
		},
		Tracking: CompilationTracking{
			Records:       s.records,
			InstalledDeps: s.installedDeps,
			Variables:     vars,
		},
		Metadata: CompilationMetadata{
			Name:        s.name,
			Description: s.description,
			Repos:       s.repos,
			LayerNames:  s.layerNames,
		},
	}
}

// prefixedOverlays returns overlay paths with the agent's OutputRoot prefix.
// Category file overlays are stored without prefix during accumulation (since
// the accumulator is agent-agnostic). The prefix is added here so that
// keel clean can match overlay paths to actual output files.
func (s *AccumulationState) prefixedOverlays() map[string][]string {
	if len(s.overlays) == 0 {
		return nil
	}
	result := make(map[string][]string, len(s.overlays))
	for path, layers := range s.overlays {
		// Paths already prefixed (e.g. ".keel/bin/...") pass through.
		if strings.HasPrefix(path, ".keel/") {
			result[path] = layers
			continue
		}
		// Category paths get the agent's output root prefix.
		result[filepath.Join(s.outputRoot, path)] = layers
	}
	return result
}

// nilIfEmpty returns nil if the map is empty, otherwise returns the map.
func nilIfEmpty(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	return m
}

// deepMergeSandbox deep-merges two SandboxConfig values with last-writer-wins
// per field. Uses YAML round-trip to include ALL fields automatically.
//
// Adding new fields to SandboxConfig requires only a yaml tag — no changes here.
func deepMergeSandbox(base, overlay *schema.SandboxConfig) *schema.SandboxConfig {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	// YAML round-trip includes ALL fields (unlike JSON which skips json:"-" fields).
	// SandboxConfig has yaml tags on every field, so this captures both portable
	// and agent-specific fields automatically. Adding new fields to SandboxConfig
	// requires only a yaml tag — no changes here.
	baseMap := sandboxToYAMLMap(base)
	overlayMap := sandboxToYAMLMap(overlay)
	merged := merge.DeepMergeMapsWithPolicies(baseMap, overlayMap, nil)

	data, err := yaml.Marshal(merged)
	if err != nil {
		slog.Warn("sandbox merge: failed to marshal merged config, returning overlay", "error", err)
		return overlay
	}
	var result schema.SandboxConfig
	if err := yaml.Unmarshal(data, &result); err != nil {
		slog.Warn("sandbox merge: failed to unmarshal merged config, returning overlay", "error", err)
		return overlay
	}

	return &result
}

// sandboxToYAMLMap converts a SandboxConfig to a map via YAML marshaling.
// Unlike JSON marshaling, this includes all fields (even those with json:"-")
// because all SandboxConfig fields have yaml tags.
func sandboxToYAMLMap(sb *schema.SandboxConfig) map[string]any {
	data, err := yaml.Marshal(sb)
	if err != nil {
		slog.Warn("sandboxToYAMLMap: failed to marshal sandbox config", "error", err)
		return nil
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		slog.Warn("sandboxToYAMLMap: failed to unmarshal sandbox config", "error", err)
		return nil
	}
	return m
}
