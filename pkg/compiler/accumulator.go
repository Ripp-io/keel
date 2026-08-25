package compiler

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/globaldeps"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// Pipeline phase constants enforce method call ordering. Each phase-checked
// method verifies the current phase and advances to the next. Calling methods
// out of order panics with a clear message so pipeline violations are caught
// during development, not silently producing wrong output.
const (
	phaseInit        = iota // AccumulationState just created, ready for Fold
	phaseFolding            // At least one Fold() call made, more allowed
	phaseGoverned           // ApplyGovernance() completed
	phaseTransformed        // TransformForAgent() completed
	phaseFinalized          // Finalize() completed
)

// phaseNames maps phase constants to human-readable names for panic messages.
var phaseNames = [...]string{
	phaseInit:        "init",
	phaseFolding:     "folding",
	phaseGoverned:    "governed",
	phaseTransformed: "transformed",
	phaseFinalized:   "finalized",
}

// FoldConfig contains the minimal configuration needed during Fold.
// It is fully agent-neutral — agent identity enters at AgentBuild.
type FoldConfig struct {
	Categories []CategoryDef
}

// AccumulationState holds the running state of the compilation pipeline.
// It deliberately spans multiple pipeline stages (FOLD, GOVERN, TRANSFORM,
// RESOLVE) with phase guards (see phaseInit..phaseFinalized) enforcing
// correct method call ordering. A future refactoring could split this into
// stage-specific types, but the phase guards currently prevent misuse.
//
// It accumulates content from each LayerIR in order, applying force-lock
// governance, deduplication, and merge policies. Variable substitution is
// deferred to Finalize so that late-bound variables (e.g., dep paths) are
// available when substitution runs.
//
// Not thread-safe — assumes single-threaded compilation. If Keel is used
// as a library with concurrent compilations, each must use its own state.
type AccumulationState struct {
	// Accumulated content
	instructions     []InstructionBlock
	settings         map[string]any
	settingsPolicies map[string]policy.KeelPolicy
	permissions      map[string]any
	hooks            map[string]any
	mcp              map[string]any
	mcpPolicies      map[string]policy.KeelPolicy
	contentBlocks    map[string][]InstructionBlock // block name → blocks (concatenated)
	configBlocks     map[string]map[string]any     // block name → config (deep-merged)
	categories       map[string][]CategoryFile     // category → files (last-writer-wins per relpath)
	categoryIndex    map[string]map[string]int     // category → relpath → slice index for O(1) lookup
	agentSpecific    map[string]map[string]any
	attribution      *CanonicalAttribution
	sandbox          *schema.SandboxConfig
	variables        map[string]string
	snippets         map[string][]byte
	bin              []BinFile
	binIndex         map[string]int // relpath → slice index for O(1) lookup
	hookScripts      []BinFile
	hookScriptIndex  map[string]int // relpath → slice index for O(1) lookup
	deps             map[string]schema.DepSpec

	// Governance
	force      *forceRegistry
	mcpPolicy  *schema.MCPPolicy
	permPolicy *schema.PermissionsPolicy

	// Tracking
	overlays                map[string][]string
	records                 []manifest.FileRecord
	installedDeps           []manifest.InstalledDepRecord
	dslConfigs              []dslConfigEntry         // DSL configs for capability warnings
	allLayerEntries         []crossFormatLayerEntry  // All non-keel layer IRs for portability warnings
	hookContributions       []HookContribution       // Hook contributions for agent-specific merge strategy
	mcpContributions        []MCPContribution        // MCP contributions for deferred filtering
	settingsContributions   []SettingsContribution   // Settings contributions for governance pass
	categoryContributions   []CategoryContribution   // Category contributions for governance pass
	filePolicyContributions []FilePolicyContribution // File policy contributions for governance pass
	overrideConflicts       []manifest.OverrideConflict
	permPolicySetBy         string

	// Metadata
	name, description string
	repos             map[string]schema.Repo
	layerNames        map[string]string

	// Pipeline phase
	phase int // current pipeline phase (phaseInit → phaseFolding → phaseGoverned → phaseTransformed → phaseFinalized)

	// Config
	foldConfig             FoldConfig
	skipConditionals       bool   // When true, Finalize skips ProcessConditionals (used by shared phase)
	distributable          bool
	scope                  Scope
	absOutputDir           string
	outputRoot             string            // Agent's output root (e.g., ".claude"), set by AgentBuild
	foldIndex              int               // Current fold index (incremented each Fold call)
	settingsForceLockedAt  int               // Fold index when settings first force-locked, -1 = never locked
	contentBlockSeparators map[string]string // block name → join separator (set by AgentBuild)
}

// NewAccumulationState creates a fresh AccumulationState with auto-injected
// variables. The layerCount is needed for the keel.layers.count auto-var.
// The keel.agent variable is set later by AgentBuild.SetAgentVar().
func NewAccumulationState(foldConfig FoldConfig, opts CompileOptions, absOutputDir string, layerCount int) *AccumulationState {
	scope := string(opts.Scope)
	if scope == "" {
		scope = "project"
	}

	return &AccumulationState{
		settings:               make(map[string]any),
		settingsPolicies:       make(map[string]policy.KeelPolicy),
		permissions:            make(map[string]any),
		hooks:                  make(map[string]any),
		mcp:                    make(map[string]any),
		mcpPolicies:            make(map[string]policy.KeelPolicy),
		contentBlocks:          make(map[string][]InstructionBlock),
		configBlocks:           make(map[string]map[string]any),
		categories:             make(map[string][]CategoryFile),
		categoryIndex:          make(map[string]map[string]int),
		agentSpecific:          make(map[string]map[string]any),
		snippets:               make(map[string][]byte),
		binIndex:               make(map[string]int),
		deps:                   make(map[string]schema.DepSpec),
		overlays:               make(map[string][]string),
		repos:                  make(map[string]schema.Repo),
		layerNames:             make(map[string]string),
		force:                  newForceRegistry(),
		hookScriptIndex:        make(map[string]int),
		contentBlockSeparators: make(map[string]string),
		variables: map[string]string{
			// keel.agent is set later by AgentBuild.SetAgentVar()
			varKeelScope:       scope,
			varKeelBinProject:  filepath.Join(absOutputDir, ".keel", "bin"),
			varKeelBinGlobal:   globaldeps.BinDir(),
			varKeelOutput:      absOutputDir,
			varKeelLayersCount: fmt.Sprintf("%d", layerCount),
			varKeelVersion:     keelBuildVersion(),
			varKeelState:       filepath.Join(absOutputDir, ".keel", "state"),
			varKeelHooksDir:    filepath.Join(absOutputDir, ".keel", "hooks"),
		},
		foldConfig:            foldConfig,
		distributable:         opts.Distributable,
		scope:                 opts.Scope,
		absOutputDir:          absOutputDir,
		settingsForceLockedAt: -1, // Initialize to -1 (never locked)
	}
}

// isAutoVar returns true if the variable name is an auto-injected keel.*
// variable that should be protected from override.
func isAutoVar(k string) bool {
	switch k {
	case varKeelAgent, varKeelScope, varKeelBinProject, varKeelBinGlobal,
		varKeelOutput, varKeelName, varKeelLayersCount, varKeelVersion, varKeelState,
		varKeelHooksDir:
		return true
	}
	return strings.HasPrefix(k, varKeelDepsPrefix)
}

// recordConflict records when a layer attempts to modify a force-locked item.
func (s *AccumulationState) recordConflict(category, key, attemptedBy string) {
	lockedBy := ""
	if _, layer := s.force.isLocked(category, key); layer != "" {
		lockedBy = layer
	}
	if lockedBy == "" {
		if _, info := s.force.isCategoryLocked(category); info != "" {
			lockedBy = info
		}
	}
	s.overrideConflicts = append(s.overrideConflicts, manifest.OverrideConflict{
		Category:    category,
		Key:         key,
		AttemptedBy: attemptedBy,
		LockedBy:    lockedBy,
	})
}

// === FOLD STAGE ===

// Fold merges one layer's IR into the accumulated state. Layers must be
// folded in trust order (lowest first, highest last) so that last-writer-wins
// and force-lock semantics produce the correct result.
func (s *AccumulationState) Fold(ir *LayerIR, layerLabel string) error {
	if s.phase > phaseFolding {
		panic(fmt.Sprintf("Fold called in phase %q — pipeline has already advanced past folding", phaseNames[s.phase]))
	}
	s.phase = phaseFolding

	// Track the current fold index for this layer
	currentFoldIndex := s.foldIndex
	s.foldIndex++

	// 1. Metadata: first-writer-wins for name/description (project identity
	// comes from the first layer, included layers contribute content, not
	// identity). Layer names are always tracked for the manifest BOM.
	if ir.Name != "" {
		s.layerNames[ir.LayerPath] = ir.Name
		if s.name == "" {
			s.name = ir.Name
		}
	}
	if ir.Description != "" && s.description == "" {
		s.description = ir.Description
	}
	for k, v := range ir.Repos {
		s.repos[k] = v
	}

	// 2. Variables: merge, protecting auto-injected keel.* vars.
	for k, v := range ir.Variables {
		if isAutoVar(k) {
			slog.Warn("cannot override auto-injected variable from layer", "var", k, "layer", layerLabel)
			continue
		}
		s.variables[k] = v
	}

	// 3. Snippets: last-writer-wins per key.
	for k, v := range ir.Snippets {
		s.snippets[k] = v
	}

	// 4. File policies: deferred to after categories (step 12b) so the
	// current layer's own files are accumulated before being force-locked
	// for subsequent layers.

	// 5. Collect DSL configs for capability warnings (emitted post-accumulation)
	if ir.DSLConfig != nil {
		s.dslConfigs = append(s.dslConfigs, dslConfigEntry{
			config: ir.DSLConfig,
			layerLabel: layerLabel,
		})
	}

	// 6. Instructions: append blocks in order, tagging each with the layer header.
	// Prefer the layer's name from keel.yml (e.g., "Agent Ripp") over the
	// directory basename (e.g., "keel") for more meaningful section separators.
	instrLabel := layerLabel
	if ir.Name != "" {
		instrLabel = ir.Name
	}
	for _, block := range ir.Instructions {
		block.LayerLabel = instrLabel
		s.instructions = append(s.instructions, block)
	}

	// 6b. ContentBlocks: append blocks in order, tagging each with the layer header.
	for name, blocks := range ir.ContentBlocks {
		for _, block := range blocks {
			block.LayerLabel = layerLabel
			s.contentBlocks[name] = append(s.contentBlocks[name], block)
		}
	}

	// 6c. ConfigBlocks: deep-merge config blocks.
	for name, config := range ir.ConfigBlocks {
		if s.configBlocks[name] == nil {
			s.configBlocks[name] = make(map[string]any)
		}
		s.configBlocks[name] = merge.DeepMergeMapsWithPolicies(s.configBlocks[name], config, nil)
	}

	// 7. MCP: filter conditions, strip force-locks, deep-merge.
	if len(ir.MCP) > 0 {
		if err := s.foldMCP(ir.MCP, layerLabel); err != nil {
			return fmt.Errorf("merging MCP from %s: %w", layerLabel, err)
		}
	}
	if ir.MCPPolicy != nil {
		if s.mcpPolicy == nil {
			s.mcpPolicy = &schema.MCPPolicy{}
		}
		if len(ir.MCPPolicy.Allowed) > 0 {
			s.mcpPolicy.Allowed = append(s.mcpPolicy.Allowed, ir.MCPPolicy.Allowed...)
		}
		if len(ir.MCPPolicy.Denied) > 0 {
			s.mcpPolicy.Denied = append(s.mcpPolicy.Denied, ir.MCPPolicy.Denied...)
		}
		if ir.MCPPolicy.EnableAll != nil {
			s.mcpPolicy.EnableAll = ir.MCPPolicy.EnableAll
		}
	}

	// 8, 9, 10. Settings/Permissions/AgentSpecific: collect contributions for governance pass.
	// Governance (lock checks, conflicts, merging) happens in ApplyGovernance.
	if len(ir.Settings) > 0 || len(ir.Permissions) > 0 || len(ir.AgentSpecific) > 0 || ir.PermissionsPolicy != nil {
		s.settingsContributions = append(s.settingsContributions, SettingsContribution{
			Settings:      ir.Settings,
			Permissions:   ir.Permissions,
			AgentSpecific: ir.AgentSpecific,
			PermPolicy:    ir.PermissionsPolicy,
			LayerLabel:    layerLabel,
			FoldIndex:     currentFoldIndex,
		})
	}

	// 10b. Portable attribution (last-writer-wins) and sandbox (deep-merge).
	if ir.Attribution != nil {
		s.attribution = ir.Attribution
	}
	if ir.Sandbox != nil {
		s.sandbox = deepMergeSandbox(s.sandbox, ir.Sandbox)
	}

	// 11. Hooks: check force-lock, strip force fields, collect contributions.
	//     All contributions are collected. Filtering based on settings force-lock
	//     happens post-accumulation in ApplyHooksSettingsLock.
	//     Hook scripts are accumulated for deployment to .keel/hooks/.
	for _, hs := range ir.HookScripts {
		s.foldHookScript(hs)
	}
	for eventName, entries := range ir.Hooks {
		if locked, _ := s.force.isLocked("hook", eventName); locked {
			s.recordConflict("hook", eventName, layerLabel)
			continue
		}
		if hasForceEntry(entries) {
			s.force.lock("hook", eventName, layerLabel)
		}
		// Hook contributions are collected during Fold but NOT filtered here.
		// Filtering happens post-accumulation in AgentBuild.ApplyHooksSettingsLock()
		// because settings force-locks apply retroactively — if a later layer
		// force-locks settings, ALL hook contributions (including from earlier
		// layers) must be filtered. This requires seeing the complete force-lock
		// state before deciding which hooks survive.
		s.hookContributions = append(s.hookContributions, HookContribution{
			EventName: eventName,
			Entries:   stripForceFromEntries(entries),
			LayerLabel: layerLabel,
			FoldIndex: currentFoldIndex,
		})
	}

	// 11b. Frontmatter translation moved to post-accumulation (TranslateFrontmatter method).
	// Files are now tagged with their SourceAgent and translation happens after all layers are folded.

	// 12. Categories: collect contributions for governance pass.
	for catName, files := range ir.Categories {
		s.categoryContributions = append(s.categoryContributions, CategoryContribution{
			CatName:     catName,
			Files:       files,
			LayerLabel:  layerLabel,
			SourceAgent: ir.SourceAgent,
			FoldIndex:   currentFoldIndex,
		})
	}

	// 12b. File policies: collect for governance pass.
	for filePath, fp := range ir.FilePolicies {
		if !fp.Force {
			continue
		}
		s.filePolicyContributions = append(s.filePolicyContributions, FilePolicyContribution{
			FilePath:  filePath,
			Policy:    fp,
			LayerLabel: layerLabel,
			FoldIndex: currentFoldIndex,
		})
	}

	// 13. Bin: last-writer-wins per RelPath.
	for _, bf := range ir.Bin {
		s.foldBin(bf, layerLabel)
	}

	// 14. Deps: first-occurrence-wins.
	for name, dep := range ir.Deps {
		if _, exists := s.deps[name]; exists {
			continue
		}
		s.deps[name] = dep
	}

	// 15. Collect all non-keel layers for portability warnings (emitted post-accumulation).
	//     Filtering by target agent happens in EmitPortabilityWarnings.
	if ir.SourceAgent != "" && ir.SourceAgent != "keel" {
		s.allLayerEntries = append(s.allLayerEntries, crossFormatLayerEntry{
			ir:     ir,
			layerLabel: layerLabel,
		})
	}

	return nil
}

// foldMCP collects raw MCP contributions for later filtering and merging.
// Conditional filtering and force-lock stripping are deferred to AgentBuild
// so that filtering happens after keel.agent is set.
func (s *AccumulationState) foldMCP(srcMap map[string]any, layerLabel string) error {
	// Store the raw data with _keel metadata intact for later filtering
	s.mcpContributions = append(s.mcpContributions, MCPContribution{
		Data:   srcMap,
		LayerLabel: layerLabel,
	})
	return nil
}

// foldCategory accumulates category files with force-lock enforcement and
// last-writer-wins semantics per relpath.
func (s *AccumulationState) foldCategory(catName string, files []CategoryFile, layerLabel string, sourceAgent string) {
	// Determine the lock category from the fold config's category definitions.
	lockCategory := catName // default fallback
	for _, cat := range s.foldConfig.Categories {
		if cat.DirName == catName {
			lockCategory = cat.LockCategory
			break
		}
	}

	existing := s.categories[catName]
	idx := s.categoryIndex[catName]
	if idx == nil {
		idx = make(map[string]int, len(existing))
		// Build index from any pre-existing entries.
		for i, f := range existing {
			if f.IsDir {
				idx["\x00dir:"+f.RelPath] = i
			} else {
				idx[f.RelPath] = i
			}
		}
		s.categoryIndex[catName] = idx
	}

	for _, file := range files {
		// Propagate source agent to the file for later frontmatter translation.
		if file.SourceAgent == "" {
			file.SourceAgent = sourceAgent
		}

		if file.IsDir {
			// Ensure directory entries exist. Use a dir-specific key prefix
			// in the index to avoid collisions with file entries.
			dirKey := "\x00dir:" + file.RelPath
			if _, ok := idx[dirKey]; !ok {
				idx[dirKey] = len(existing)
				existing = append(existing, file)
			}
			continue
		}

		// Check force-lock.
		if locked, _ := s.force.isLocked(lockCategory, file.RelPath); locked {
			s.recordConflict(lockCategory, file.RelPath, layerLabel)
			continue
		}

		// Check frontmatter for force: true (parse but don't strip yet —
		// stripping happens at Finalize).
		if merge.IsMarkdownFile(file.RelPath) && len(file.Content) > 0 {
			fm, _, fmErr := merge.ParseAndCleanFrontmatter(file.Content)
			if fmErr == nil && fm != nil && fm.Force != nil && *fm.Force {
				s.force.lock(lockCategory, file.RelPath, layerLabel)
			}
		}

		// Last-writer-wins: O(1) lookup via index map.
		if i, ok := idx[file.RelPath]; ok {
			existing[i] = file
		} else {
			idx[file.RelPath] = len(existing)
			existing = append(existing, file)
		}

		// Record overlay provenance.
		outputPath := filepath.Join(catName, file.RelPath)
		s.overlays[outputPath] = append(s.overlays[outputPath], layerLabel)
	}

	s.categories[catName] = existing
}

// foldBin accumulates bin files with last-writer-wins per RelPath
// and records overlay provenance.
func (s *AccumulationState) foldBin(bf BinFile, layerLabel string) {
	overlayKey := filepath.Join(".keel", "bin", bf.RelPath)
	if i, ok := s.binIndex[bf.RelPath]; ok {
		s.bin[i] = bf
	} else {
		s.binIndex[bf.RelPath] = len(s.bin)
		s.bin = append(s.bin, bf)
	}
	s.overlays[overlayKey] = append(s.overlays[overlayKey], layerLabel)
}

// foldHookScript accumulates hook scripts with last-writer-wins per RelPath.
func (s *AccumulationState) foldHookScript(hs BinFile) {
	if i, ok := s.hookScriptIndex[hs.RelPath]; ok {
		s.hookScripts[i] = hs
	} else {
		s.hookScriptIndex[hs.RelPath] = len(s.hookScripts)
		s.hookScripts = append(s.hookScripts, hs)
	}
}

// ProcessDeps installs or downloads dependencies. This is the only
// accumulator method that performs disk I/O.
func (s *AccumulationState) ProcessDeps(outputDir string, allowInstall bool, requireChecksums bool, preInstalled map[string]string) error {
	if len(s.deps) == 0 {
		return nil
	}

	records, err := processDeps(s.deps, outputDir, allowInstall, requireChecksums, s.variables, preInstalled)
	if err != nil {
		return err
	}
	s.installedDeps = append(s.installedDeps, records...)

	return nil
}
