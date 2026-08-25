package compiler

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// capCheck pairs a DSL feature name with functions to test data presence and capability support.
type capCheck struct {
	name    string
	hasData func(*schema.KeelConfig) bool
	hasCap  func(Capabilities) bool
}

// dslConfigEntry pairs a DSL config with its layer header for capability warnings.
type dslConfigEntry struct {
	config *schema.KeelConfig
	layerLabel string
}

// crossFormatLayerEntry pairs cross-format layer IR with its header for portability warnings.
type crossFormatLayerEntry struct {
	ir     *LayerIR
	layerLabel string
}

// HookContribution records one layer's contribution to the hooks map.
type HookContribution struct {
	EventName string
	Entries   any
	LayerLabel string
	FoldIndex int // Fold index when this contribution was collected
}

// MCPContribution records one layer's contribution to the MCP configuration.
type MCPContribution struct {
	Data       map[string]any
	LayerLabel string
}

// SettingsContribution records one layer's contribution to settings, permissions, and agent-specific config.
type SettingsContribution struct {
	Settings      map[string]any
	Permissions   map[string]any
	AgentSpecific map[string]map[string]any
	PermPolicy    *schema.PermissionsPolicy
	LayerLabel    string
	FoldIndex     int
}

// CategoryContribution records one layer's contribution to a category.
type CategoryContribution struct {
	CatName     string
	Files       []CategoryFile
	LayerLabel  string
	SourceAgent string
	FoldIndex   int
}

// FilePolicyContribution records one layer's file policy declarations.
type FilePolicyContribution struct {
	FilePath  string
	Policy    schema.FilePolicy
	LayerLabel string
	FoldIndex int
}

// capChecks is the data-driven table of capability warnings.
var capChecks = []capCheck{
	{"hooks", func(c *schema.KeelConfig) bool { return len(c.Hooks) > 0 }, func(cap Capabilities) bool { return cap.Hooks }},
	{"mcp", func(c *schema.KeelConfig) bool { return c.MCP != nil && len(c.MCP.Servers) > 0 }, func(cap Capabilities) bool { return cap.MCP }},
	{"permissions", func(c *schema.KeelConfig) bool { return c.Permissions != nil && len(c.Permissions.Rules) > 0 }, func(cap Capabilities) bool { return cap.Permissions }},
	{"commands", func(c *schema.KeelConfig) bool { return len(c.Commands) > 0 }, func(cap Capabilities) bool { return cap.Commands }},
	{"agents", func(c *schema.KeelConfig) bool { return len(c.Agents) > 0 }, func(cap Capabilities) bool { return cap.Agents }},
	{"skills", func(c *schema.KeelConfig) bool { return len(c.Skills) > 0 }, func(cap Capabilities) bool { return cap.Skills }},
	{"plugins", func(c *schema.KeelConfig) bool { return len(c.Plugins) > 0 }, func(cap Capabilities) bool { return cap.Plugins }},
	{"agent-memory", func(c *schema.KeelConfig) bool { return len(c.AgentMemory) > 0 }, func(cap Capabilities) bool { return cap.AgentMemory }},
	{"rules", func(c *schema.KeelConfig) bool { return len(c.Rules) > 0 }, func(cap Capabilities) bool { return cap.Rules }},
	{"bin", func(c *schema.KeelConfig) bool { return len(c.Bin) > 0 }, func(cap Capabilities) bool { return cap.Bin }},
	{"deps", func(c *schema.KeelConfig) bool { return len(c.Deps) > 0 }, func(cap Capabilities) bool { return cap.Deps }},
}

// sortedMapKeys returns the keys of a map in sorted order.
func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// === GOVERN STAGE ===

// governanceEntry represents a single governance operation in the sorted processing order.
type governanceEntry struct {
	foldIndex int
	order     int // 0=settings, 1=categories, 2=file-policies (within same fold)
	apply     func()
}

// ApplyGovernance processes all settings, category, and file policy contributions
// collected during the fold loop. Contributions are processed in fold index order,
// with the order within each fold matching the original Fold() sequence:
// settings → categories → file policies.
func (s *AccumulationState) ApplyGovernance() {
	if s.phase > phaseFolding {
		panic(fmt.Sprintf("ApplyGovernance called in phase %q — must be called before TransformForAgent/Finalize", phaseNames[s.phase]))
	}
	s.phase = phaseGoverned

	var entries []governanceEntry

	// 1. Collect settings contributions (order=0)
	for _, c := range s.settingsContributions {
		c := c // capture loop variable
		entries = append(entries, governanceEntry{
			foldIndex: c.FoldIndex,
			order:     0,
			apply:     func() { s.applySettingsContribution(c) },
		})
	}

	// 2. Collect category contributions (order=1)
	for _, c := range s.categoryContributions {
		c := c // capture loop variable
		entries = append(entries, governanceEntry{
			foldIndex: c.FoldIndex,
			order:     1,
			apply:     func() { s.foldCategory(c.CatName, c.Files, c.LayerLabel, c.SourceAgent) },
		})
	}

	// 3. Collect file policy contributions (order=2)
	for _, c := range s.filePolicyContributions {
		c := c // capture loop variable
		entries = append(entries, governanceEntry{
			foldIndex: c.FoldIndex,
			order:     2,
			apply: func() {
				category := categorizeFilePath(c.FilePath, s.foldConfig.Categories)
				key := forceLockKeyForFile(c.FilePath, s.foldConfig.Categories)
				s.force.lock(category, key, c.LayerLabel)
			},
		})
	}

	// Sort by fold index, then by order within fold
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].foldIndex != entries[j].foldIndex {
			return entries[i].foldIndex < entries[j].foldIndex
		}
		return entries[i].order < entries[j].order
	})

	// Single-pass processing
	prevFold := -1
	for _, e := range entries {
		// When transitioning to a new fold, check if settings got locked by the previous fold.
		// The lock takes effect for contributions from the NEXT fold index (the one we're about to process).
		if e.foldIndex != prevFold && prevFold >= 0 {
			settingsLocked, _ := s.force.isCategoryLocked("settings")
			if settingsLocked && s.settingsForceLockedAt < 0 {
				s.settingsForceLockedAt = e.foldIndex
			}
		}
		prevFold = e.foldIndex
		e.apply()
	}

	// Final check after all entries processed
	if prevFold >= 0 {
		settingsLocked, _ := s.force.isCategoryLocked("settings")
		if settingsLocked && s.settingsForceLockedAt < 0 {
			// Settings was locked, but there are no more folds after this.
			// This doesn't matter since there are no more contributions to block.
			// We still set it for consistency with the manifest.
			s.settingsForceLockedAt = prevFold + 1
		}
	}
}

// applySettingsContribution processes a single settings contribution.
func (s *AccumulationState) applySettingsContribution(c SettingsContribution) {
	// A contribution is blocked if it arrived at or after the fold where
	// settings was first force-locked.
	locked := s.settingsForceLockedAt >= 0 && c.FoldIndex >= s.settingsForceLockedAt

	if locked {
		// Record conflicts for blocked contributions
		if len(c.Settings) > 0 {
			s.recordConflict("settings", "settings", c.LayerLabel)
		}
		if len(c.Permissions) > 0 {
			s.recordConflict("permissions", "permissions", c.LayerLabel)
		}
		for agent := range c.AgentSpecific {
			s.recordConflict("agent-specific", agent, c.LayerLabel)
		}
	} else {
		// Merge accepted contributions
		if len(c.Settings) > 0 {
			s.settings = merge.DeepMergeMapsWithPolicies(s.settings, c.Settings, s.settingsPolicies)
		}

		if len(c.Permissions) > 0 {
			var permPolicies map[string]policy.KeelPolicy
			if s.permPolicy != nil && s.permPolicy.Merge == "additive" {
				// When additive, permission arrays (allow, deny) should be unioned.
				permPolicies = map[string]policy.KeelPolicy{
					"allow": {Merge: policy.MergeStrategyAdditive},
					"deny":  {Merge: policy.MergeStrategyAdditive},
				}
			}
			s.permissions = merge.DeepMergeMapsWithPolicies(s.permissions, c.Permissions, permPolicies)
		}

		for agent, data := range c.AgentSpecific {
			if len(data) == 0 {
				continue
			}
			existing, ok := s.agentSpecific[agent]
			if !ok {
				existing = make(map[string]any)
			}
			s.agentSpecific[agent] = merge.DeepMergeMapsWithPolicies(existing, data, nil)
		}
	}

	// PermPolicy tracking — always applies regardless of lock state.
	// (matches current behavior: permPolicy is tracked outside the settingsLocked check)
	if c.PermPolicy != nil {
		s.permPolicy = c.PermPolicy
		s.permPolicySetBy = c.LayerLabel
	}
}

// ApplyAgentGovernance applies agent-specific governance rules.
// Call after Fold loop, before TransformForAgent.
func (s *AccumulationState) ApplyAgentGovernance(p AgentProfile) {
	s.ApplyHooksSettingsLock(p.HooksInSettings)
}

// ApplyHooksSettingsLock filters out hook contributions that arrived after
// the settings file was force-locked. This is only relevant for agents with
// HooksInSettings=true (Claude), where hooks live inside settings.json.
func (s *AccumulationState) ApplyHooksSettingsLock(hooksInSettings bool) {
	if !hooksInSettings || s.settingsForceLockedAt < 0 {
		return // Not applicable or settings never force-locked
	}

	// Keep only contributions from before settings was force-locked
	var kept []HookContribution
	for _, c := range s.hookContributions {
		if c.FoldIndex < s.settingsForceLockedAt {
			kept = append(kept, c)
		}
	}
	s.hookContributions = kept
}

// === TRANSFORM STAGE ===

// TransformForAgent applies agent-specific structural transforms.
// Call after ApplyAgentGovernance, before EmitAgentWarnings.
func (s *AccumulationState) TransformForAgent(p AgentProfile) {
	if s.phase < phaseGoverned {
		panic(fmt.Sprintf("TransformForAgent called in phase %q — ApplyGovernance must be called first", phaseNames[s.phase]))
	}
	if s.phase > phaseGoverned {
		panic(fmt.Sprintf("TransformForAgent called in phase %q — pipeline has already advanced past transformation", phaseNames[s.phase]))
	}
	s.phase = phaseTransformed
	s.outputRoot = p.OutputRoot
	s.SetAgentVar(p.Name)
	s.ApplyHooksStrategy(p.HooksInSettings)
	s.TranslateFrontmatter(p.Name)
	s.PruneCategories(p)
	s.PopulateContentBlockMeta(p)
}

// SetAgentVar sets the keel.agent variable to the given agent name.
// This must be called before Finalize() since variable substitution relies on it.
func (s *AccumulationState) SetAgentVar(agentName string) {
	s.variables[varKeelAgent] = agentName
}

// ApplyHooksStrategy applies the agent-specific merge strategy to hook contributions.
// If hooksInSettings is true (Claude), uses last-writer-wins.
// If hooksInSettings is false (Cursor), uses additive merge.
func (s *AccumulationState) ApplyHooksStrategy(hooksInSettings bool) {
	if len(s.hookContributions) == 0 {
		return
	}

	if hooksInSettings {
		// Claude: last-writer-wins per event.
		for _, contrib := range s.hookContributions {
			s.hooks[contrib.EventName] = contrib.Entries
		}
	} else {
		// Cursor: additive merge (union with deduplication).
		additivePolicies := map[string]policy.KeelPolicy{
			"": {Merge: policy.MergeStrategyAdditive},
		}
		for _, contrib := range s.hookContributions {
			layerHooks := map[string]any{
				contrib.EventName: contrib.Entries,
			}
			s.hooks = merge.DeepMergeMapsWithPolicies(s.hooks, layerHooks, additivePolicies)
		}
	}
}

// TranslateFrontmatter renames frontmatter fields in accumulated category files
// for cross-format compatibility. Only translates files where SourceAgent != targetAgent.
func (s *AccumulationState) TranslateFrontmatter(targetAgent string) {
	for catName, files := range s.categories {
		// Only translate fields in categories that have portable equivalents.
		// Currently only agent category uses frontmatter translation (background <-> is_background).
		portCat := categoryToPortabilityKey(catName)
		if portCat != "agent" {
			continue
		}

		for i, file := range files {
			// Skip if not a cross-format file or not a markdown file.
			if file.IsDir || !merge.IsMarkdownFile(file.RelPath) || len(file.Content) == 0 {
				continue
			}
			if file.SourceAgent == "" || file.SourceAgent == "keel" || file.SourceAgent == targetAgent {
				continue
			}

			// Get translation map for this source→target pair.
			sourceTranslations, ok := frontmatterTranslations[file.SourceAgent]
			if !ok {
				continue
			}
			fieldMap, ok := sourceTranslations[targetAgent]
			if !ok || len(fieldMap) == 0 {
				continue
			}

			// Translate frontmatter fields.
			translated, changed := renameFrontmatterFields(file.Content, fieldMap)
			if changed {
				files[i].Content = translated
			}
		}
	}
}

// PruneCategories removes accumulated categories that are not in the target
// agent's profile. Readers produce a complete IR (including root-level
// categories); filtering for the target agent happens here.
func (s *AccumulationState) PruneCategories(p AgentProfile) {
	supported := make(map[string]bool, len(p.Categories))
	for _, cat := range p.Categories {
		supported[cat.DirName] = true
	}
	// _dotdir is a passthrough category for arbitrary dot-directory files.
	supported[CategoryDotDir] = true

	for name := range s.categories {
		if !supported[name] {
			delete(s.categories, name)
		}
	}
}

// PopulateContentBlockMeta populates contentBlockSeparators from the profile.
func (s *AccumulationState) PopulateContentBlockMeta(p AgentProfile) {
	for _, cb := range p.ContentOutputs {
		s.contentBlockSeparators[cb.Name] = cb.JoinSeparator
	}
}

// EmitAgentWarnings emits diagnostic warnings for agent compatibility.
// Call after TransformForAgent.
func (s *AccumulationState) EmitAgentWarnings(p AgentProfile) {
	s.EmitCapabilityWarnings(p)
	s.EmitPortabilityWarnings(p)
}

// EmitCapabilityWarnings checks accumulated DSL configs against the target
// agent's capabilities and emits warnings for unsupported features.
// Warnings are emitted once per feature (not per layer) to avoid noise.
// Instruction-only adapters (like agentsmd) suppress all capability warnings.
func (s *AccumulationState) EmitCapabilityWarnings(p AgentProfile) {
	if p.Capabilities.InstructionOnly {
		return
	}

	// Build supported ContentBlocks and ConfigBlocks maps from profile
	supportedContentBlocks := make(map[string]bool)
	for _, cb := range p.ContentOutputs {
		supportedContentBlocks[cb.Name] = true
	}
	supportedConfigBlocks := make(map[string]bool)
	for _, cb := range p.ConfigOutputs {
		supportedConfigBlocks[cb.Name] = true
	}

	// Track which features have already been warned about (dedup across layers).
	warned := make(map[string]bool)

	for _, entry := range s.dslConfigs {
		for _, check := range capChecks {
			if !warned[check.name] && check.hasData(entry.config) && !check.hasCap(p.Capabilities) {
				slog.Warn("layer uses "+check.name+" but target adapter does not support them",
					"layer", entry.layerLabel, "adapter", p.Name)
				warned[check.name] = true
			}
		}

		// Check ContentBlocks (loop, worktree-include)
		if !warned["loop"] && len(entry.config.Loop) > 0 && !supportedContentBlocks["loop"] {
			slog.Warn("layer uses loop but target adapter does not support them",
				"layer", entry.layerLabel, "adapter", p.Name)
			warned["loop"] = true
		}
		if !warned["worktree-include"] && len(entry.config.WorktreeInclude) > 0 && !supportedContentBlocks["worktree-include"] {
			slog.Warn("layer uses worktree-include but target adapter does not support them",
				"layer", entry.layerLabel, "adapter", p.Name)
			warned["worktree-include"] = true
		}

		// Check ConfigBlocks (environment)
		if !warned["environment"] && len(entry.config.Environment) > 0 && !supportedConfigBlocks["environment"] {
			slog.Warn("layer uses environment but target adapter does not support them",
				"layer", entry.layerLabel, "adapter", p.Name)
			warned["environment"] = true
		}
	}
}

// EmitPortabilityWarnings checks cross-format layers for lossy content and
// non-portable frontmatter fields. Only emits warnings for layers where
// SourceAgent != target agent.
func (s *AccumulationState) EmitPortabilityWarnings(p AgentProfile) {
	for _, entry := range s.allLayerEntries {
		// Only warn for cross-format layers (source != target)
		if entry.ir.SourceAgent != p.Name {
			emitLossyWarnings(entry.ir, &p, entry.layerLabel)
			emitFrontmatterWarnings(entry.ir, p.Name, entry.layerLabel)
		}
	}
}

// emitLossyWarnings checks for IR content that the target agent profile does
// not support and emits structured warnings.
func emitLossyWarnings(ir *LayerIR, p *AgentProfile, layerLabel string) {
	for agent, data := range ir.AgentSpecific {
		if agent == p.Name || len(data) == 0 {
			continue
		}
		keys := sortedMapKeys(data)
		slog.Warn("agent-specific settings will not be included in output",
			"source", agent, "target", p.Name,
			"keys", strings.Join(keys, ", "),
			"layer", layerLabel)
	}

	targetCats := make(map[string]bool, len(p.Categories))
	for _, cat := range p.Categories {
		targetCats[cat.DirName] = true
	}
	for catName, files := range ir.Categories {
		if targetCats[catName] || len(files) == 0 {
			continue
		}
		slog.Warn("category has no equivalent in target agent, skipping",
			"category", catName, "target", p.Name, "layer", layerLabel)
	}

	// Check ContentBlocks and ConfigBlocks against profile's outputs
	targetContentBlocks := make(map[string]bool, len(p.ContentOutputs))
	for _, cb := range p.ContentOutputs {
		targetContentBlocks[cb.Name] = true
	}
	for blockName := range ir.ContentBlocks {
		if !targetContentBlocks[blockName] {
			slog.Warn("content block has no equivalent in target agent, skipping",
				"block", blockName, "target", p.Name, "layer", layerLabel)
		}
	}

	targetConfigBlocks := make(map[string]bool, len(p.ConfigOutputs))
	for _, cb := range p.ConfigOutputs {
		targetConfigBlocks[cb.Name] = true
	}
	for blockName := range ir.ConfigBlocks {
		if !targetConfigBlocks[blockName] {
			slog.Warn("config block has no equivalent in target agent, skipping",
				"block", blockName, "target", p.Name, "layer", layerLabel)
		}
	}

	if len(ir.Hooks) > 0 {
		for eventName, entries := range ir.Hooks {
			if !isPortableHookEvent(eventName) {
				slog.Warn("hook event may not be supported by target agent",
					"event", eventName, "target", p.Name, "layer", layerLabel)
			}
			nonPortable := extractNonPortableHandlerTypes(entries)
			for _, handlerType := range nonPortable {
				slog.Warn("hook handler type not supported by target agent",
					"type", handlerType, "event", eventName,
					"target", p.Name, "layer", layerLabel)
			}
		}
	}
}

// === RESOLVE STAGE ===

// FilterAndMergeMCP filters conditional MCP content and merges all contributions
// into the final MCP map. This is called from Finalize after all variables (auto,
// layer-defined, and CLI-provided) are available, so MCP conditions can reference
// any variable, not just keel.agent.
func (s *AccumulationState) FilterAndMergeMCP() {
	for _, contrib := range s.mcpContributions {
		data := contrib.Data

		// Step 1: Filter conditions using current variables (keel.agent is now set)
		if len(s.variables) > 0 && len(data) > 0 {
			filtered, err := merge.FilterConditionJSON(data, s.variables)
			if err != nil {
				slog.Warn("filtering MCP conditions", "layer", contrib.LayerLabel, "error", err)
				continue
			}
			data = filtered
		}

		// Step 2: Strip force-lock policies and remove force-locked entries
		cleanedSrc, policies, err := cleanContainerForceLocked(data, "mcpServers", "mcp", s.force)
		if err != nil {
			slog.Warn("cleaning MCP force-locks", "layer", contrib.LayerLabel, "error", err)
			continue
		}

		// Step 3: Register force-locks from this contribution
		registerMCPPolicies(policies, s.force, contrib.LayerLabel)

		// Step 4: Merge into accumulated MCP state
		if len(cleanedSrc) > 0 {
			s.mcp = merge.DeepMergeMapsWithPolicies(s.mcp, cleanedSrc, s.mcpPolicies)
			// Merge per-contribution policies into accumulated policies
			for k, v := range policies {
				s.mcpPolicies[k] = v
			}
		}
	}
}
