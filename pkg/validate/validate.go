// Package validate provides validation logic for Keel configuration layers.
// It checks layer structure, schema validity, file references, agent compatibility,
// and Agent Skills format compliance.
package validate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/skills"
)

// Result collects all findings from a validation run.
type Result struct {
	Path       string
	Format     config.LayerFormat
	Version    string
	Name       string
	Items      []Item
	ErrorCount int
	WarnCount  int
}

// Item is a single pass/fail/warn finding.
type Item struct {
	OK      bool // true = pass, false = error
	Warn    bool // true = warning (non-fatal), only when OK is false
	Message string
}

func (r *Result) addPass(msg string) {
	r.Items = append(r.Items, Item{OK: true, Message: msg})
}

func (r *Result) addError(msg string) {
	r.Items = append(r.Items, Item{OK: false, Message: msg})
	r.ErrorCount++
}

func (r *Result) addWarn(msg string) {
	r.Items = append(r.Items, Item{OK: false, Warn: true, Message: msg})
	r.WarnCount++
}

// Options configures which validation checks to run.
type Options struct {
	// Agent is the target agent for compatibility checks (e.g. "claude", "cursor").
	// Empty string skips agent compatibility checks.
	Agent string

	// AgentSkillsCheck controls whether Agent Skills format validation runs.
	AgentSkillsCheck bool

	// Lint controls whether body content linting runs. When true, markdown
	// files are scanned for non-portable agent-specific patterns. If Agent
	// is set, warnings for that agent's own patterns are suppressed (they
	// are valid for the target agent).
	Lint bool

	// TokenBudget controls whether token budget estimation runs.
	TokenBudget bool

	// AgentToolsCheck controls whether agent tool count validation runs.
	AgentToolsCheck bool
}

// Run performs all validation checks on a layer directory.
func Run(layerPath string, opts Options) Result {
	result := Result{Path: layerPath}

	// 1. Check path exists and is a directory.
	info, err := os.Stat(layerPath)
	if err != nil {
		result.addError(fmt.Sprintf("path does not exist: %s", layerPath))
		return result
	}
	if !info.IsDir() {
		result.addError(fmt.Sprintf("path is not a directory: %s", layerPath))
		return result
	}

	// 2. Detect layer format.
	format, version := config.DetectLayerFormat(layerPath)
	result.Format = format
	result.Version = version

	// 3. If keel.yml exists: parse and validate schema.
	keelPath := filepath.Join(layerPath, "keel.yml")
	keelExists := config.RegularFileExists(keelPath)
	var cfg *schema.KeelConfig

	if keelExists {
		parsed, parseErr := schema.Parse(keelPath)
		if parseErr != nil {
			result.addError(fmt.Sprintf("keel.yml: %s", parseErr))
			// Cannot continue checking references if parse failed,
			// but still run agent and skills checks (skills falls back
			// to native layer discovery when cfg is nil).
			addAgentCheck(&result, format, opts.Agent)
			if opts.AgentSkillsCheck {
				validateSkills(&result, layerPath, false, nil)
			}
			return result
		}
		cfg = parsed
		merged := schema.MergeWithDefaults(parsed)

		result.addPass("keel.yml schema valid")
		result.Name = cfg.Name

		// 4. Check referenced files exist.
		// Use the merged config (with defaults) so convention directories
		// are enumerated. Explicitly declared refs that are missing are
		// errors; default-provided refs that are missing are silently
		// skipped (matching the compiler's behavior).
		checkFileRefs(&result, layerPath, "instructions", cfg.Instructions, true)
		checkFileRefs(&result, layerPath, "instructions", defaultOnly(merged.Instructions, cfg.Instructions), false)
		checkFileRefs(&result, layerPath, "commands", cfg.Commands, true)
		checkFileRefs(&result, layerPath, "commands", defaultOnly(merged.Commands, cfg.Commands), false)
		checkFileRefs(&result, layerPath, "agents", cfg.Agents, true)
		checkFileRefs(&result, layerPath, "agents", defaultOnly(merged.Agents, cfg.Agents), false)
		checkFileRefs(&result, layerPath, "skills", cfg.Skills, true)
		checkFileRefs(&result, layerPath, "skills", defaultOnly(merged.Skills, cfg.Skills), false)
		if cfg.MCP != nil {
			checkFileRefs(&result, layerPath, "mcp", cfg.MCP.Servers, true)
		}
		checkFileRefs(&result, layerPath, "hooks", cfg.Hooks, true)
		checkFileRefs(&result, layerPath, "hooks", defaultOnly(merged.Hooks, cfg.Hooks), false)
		if cfg.Permissions != nil {
			checkFileRefs(&result, layerPath, "permissions", cfg.Permissions.Rules, true)
		}
		checkFileRefs(&result, layerPath, "init", cfg.Init, true)
		checkFileRefs(&result, layerPath, "init", defaultOnly(merged.Init, cfg.Init), false)
		checkFileRefs(&result, layerPath, "bin", cfg.Bin, true)
		checkFileRefs(&result, layerPath, "snippets", defaultOnly(merged.Snippets, cfg.Snippets), false)

		// Snippet placeholder and inventory checks.
		snippetDirs := resolveSnippetDirs(layerPath, merged.Snippets)
		checkSnippetPlaceholders(&result, layerPath, snippetDirs, cfg)
		checkSnippetInventory(&result, layerPath, snippetDirs, cfg)

		// Check deps recommendations.
		if len(cfg.Deps) > 0 {
			for name, dep := range cfg.Deps {
				if dep.Download != "" && dep.SHA256 == "" {
					result.addWarn(fmt.Sprintf("deps.%s: download without sha256 — binary integrity is not verified", name))
				}
			}
			result.addPass(fmt.Sprintf("%d dep(s) defined", len(cfg.Deps)))
		}

		// Report bin count.
		if len(cfg.Bin) > 0 {
			result.addPass(fmt.Sprintf("%d bin file(s) defined", len(cfg.Bin)))
		}

		// Check settings file references.
		if cfg.Settings != nil {
			if cfg.Settings.Claude != "" {
				checkSingleFile(&result, layerPath, "settings", cfg.Settings.Claude)
			}
			if cfg.Settings.Cursor != "" {
				checkSingleFile(&result, layerPath, "settings", cfg.Settings.Cursor)
			}
		}

		// Report repo count.
		if len(cfg.Repos) > 0 {
			result.addPass(fmt.Sprintf("%d repo(s) defined", len(cfg.Repos)))
		}
	} else if format == config.FormatKeel {
		// No keel.yml and no native markers — likely misconfigured.
		result.addWarn("no keel.yml found and no native agent files detected")
	} else {
		// Native layer — validate native files exist.
		result.addPass(fmt.Sprintf("native %s layer (no keel.yml)", format))
		validateNativeLayer(&result, layerPath, format)
	}

	// 5. Agent compatibility check.
	addAgentCheck(&result, format, opts.Agent)

	// 6. Agent Skills format check.
	// CLI flag (explicit false) takes priority over keel.yml config.
	runSkillsCheck := opts.AgentSkillsCheck
	if runSkillsCheck && cfg != nil && cfg.ValidateOpts != nil && cfg.ValidateOpts.AgentSkills != nil {
		runSkillsCheck = *cfg.ValidateOpts.AgentSkills
	}
	if runSkillsCheck {
		validateSkills(&result, layerPath, keelExists, cfg)
	}

	// 7. Token budget estimation.
	if opts.TokenBudget {
		checkTokenBudget(&result, layerPath, keelExists, cfg)
	}

	// 8. Agent tool count check.
	if opts.AgentToolsCheck {
		checkAgentTools(&result, layerPath, keelExists, cfg)
	}

	// 9. Body content linting.
	if opts.Lint {
		lintMarkdownBodies(&result, layerPath, opts.Agent)
	}

	return result
}

// checkFileRefs validates that each source/path reference in a FileRef slice
// points to an existing file or directory. When required is true, missing refs
// are reported as errors; when false, they are silently skipped (convention
// directories are optional). For directory references (path:), enumerates
// the discovered files so the user can verify the right content is being
// included.
func checkFileRefs(result *Result, layerPath, category string, refs []schema.FileRef, required bool) {
	for _, ref := range refs {
		target := ref.Source
		if target == "" {
			continue
		}

		fullPath := filepath.Join(layerPath, target)
		info, err := os.Stat(fullPath)
		if err != nil {
			if required {
				result.addError(fmt.Sprintf("%s: %s referenced but not found", category, target))
			}
			continue
		}

		if !info.IsDir() {
			result.addPass(fmt.Sprintf("%s: %s exists", category, target))
			continue
		}

		// Directory reference — enumerate contents so the user can verify
		// which files will be included.
		enumerateDir(result, fullPath, category, target)
	}
}

// enumerateDir lists files inside a directory for validate output.
// Uses filepath.Join for correct path separators. For "hooks" category,
// only .json files are listed (hook directories may contain .sh scripts
// referenced by the JSON, which are not hook definitions themselves).
func enumerateDir(result *Result, fullPath, category, target string) {
	entries, walkErr := os.ReadDir(fullPath)
	if walkErr != nil {
		result.addPass(fmt.Sprintf("%s: %s exists (directory, could not enumerate)", category, target))
		return
	}

	fileCount := 0
	for _, e := range entries {
		// For hooks, only enumerate .json files — .sh scripts in the hooks
		// directory are referenced by the JSON but are not hook definitions.
		if category == "hooks" && !e.IsDir() && !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		rel := filepath.Join(target, e.Name())
		if e.IsDir() {
			result.addPass(fmt.Sprintf("%s: %s/", category, rel))
		} else {
			result.addPass(fmt.Sprintf("%s: %s", category, rel))
		}
		fileCount++
	}
	if fileCount == 0 {
		result.addWarn(fmt.Sprintf("%s: %s is an empty directory", category, target))
	}
}

// defaultOnly returns the FileRefs from merged that are not present in user.
// When the user declared the category, MergeWithDefaults replaces entirely,
// so defaultOnly returns nil. When the user didn't declare it, merged
// contains only the defaults.
func defaultOnly(merged, user []schema.FileRef) []schema.FileRef {
	if len(user) > 0 {
		return nil // user declared this category, defaults were replaced
	}
	return merged
}

// checkSingleFile validates a single file reference exists.
func checkSingleFile(result *Result, layerPath, category, file string) {
	fullPath := filepath.Join(layerPath, file)
	if _, err := os.Stat(fullPath); err != nil {
		result.addError(fmt.Sprintf("%s: %s referenced but not found", category, file))
	} else {
		result.addPass(fmt.Sprintf("%s: %s exists", category, file))
	}
}

// validateNativeLayer checks for expected native agent files.
func validateNativeLayer(result *Result, layerPath string, format config.LayerFormat) {
	for _, a := range config.AgentList() {
		if a.Format != format {
			continue
		}
		for _, f := range a.Files {
			fp := filepath.Join(layerPath, f)
			if config.RegularFileExists(fp) {
				result.addPass(fmt.Sprintf("%s exists", f))
			}
		}
		for _, d := range a.Dirs {
			dp := filepath.Join(layerPath, d)
			if info, err := os.Stat(dp); err == nil && info.IsDir() {
				result.addPass(fmt.Sprintf("%s/ exists", d))
			}
		}
	}
}

// addAgentCheck adds a warning if the layer format is incompatible with
// the specified target agent.
func addAgentCheck(result *Result, format config.LayerFormat, agent string) {
	if agent == "" {
		return
	}

	// Keel DSL layers are compatible with any agent.
	if format == config.FormatKeel {
		result.addPass(fmt.Sprintf("compatible with agent %q (keel DSL)", agent))
		return
	}

	// Native layer targeting a different agent.
	if string(format) != agent {
		result.addWarn(fmt.Sprintf("layer format %q targets a different agent than %q", format, agent))
	} else {
		result.addPass(fmt.Sprintf("compatible with agent %q", agent))
	}
}

// lintMarkdownBodies walks the layer directory for markdown files and scans
// each for non-portable agent-specific body patterns. All findings are
// emitted as warnings (non-blocking). When targetAgent is set, warnings
// for that agent's own patterns are suppressed.
func lintMarkdownBodies(result *Result, layerPath, targetAgent string) {
	var mdFiles []string

	err := filepath.Walk(layerPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // best effort — skip inaccessible paths
		}
		if info.IsDir() {
			// Skip snippet directories — they contain compile-time templates
			// with {{param}} placeholders that are Keel syntax, not Cursor
			// Maestro variables.
			if info.Name() == "snippets" {
				return filepath.SkipDir
			}
			return nil
		}
		lower := strings.ToLower(info.Name())
		if strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdc") {
			mdFiles = append(mdFiles, path)
		}
		return nil
	})
	if err != nil {
		return // best effort
	}

	lintCount := 0
	for _, path := range mdFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		rel, _ := filepath.Rel(layerPath, path)
		if rel == "" {
			rel = path
		}

		warnings := lintBodyPatterns(content, rel)
		for _, w := range warnings {
			// If targeting a specific agent, skip warnings about patterns
			// native to that agent — they will work fine.
			if targetAgent != "" && w.Agent == targetAgent {
				continue
			}
			result.addWarn(fmt.Sprintf("lint: %s:%d: %s (%s)", w.File, w.Line, w.Message, w.Pattern))
			lintCount++
		}
	}

	if len(mdFiles) > 0 && lintCount == 0 {
		result.addPass("lint: no non-portable body patterns found")
	}
}

// discoverSkillPaths discovers skill directories from both DSL and native layers.
// Returns two slices: parent directories (containing skill subdirs) and individual
// skill directories (containing SKILL.md directly).
func discoverSkillPaths(layerPath string, keelExists bool, cfg *schema.KeelConfig) (skillDirs, singleSkills []string) {
	if keelExists && cfg != nil {
		// DSL layer: resolve skill FileRef entries to directories.
		// A ref may point to a skills parent directory (e.g. "skills/")
		// or a single skill directory (e.g. "skills/security-review").
		for _, ref := range cfg.Skills {
			target := ref.Source
			if target == "" {
				continue
			}
			fullPath := filepath.Join(layerPath, target)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue // missing ref already reported by checkFileRefs
			}
			if !info.IsDir() {
				// File ref (e.g. SKILL.md directly) — validate parent as skill dir.
				if filepath.Base(fullPath) == "SKILL.md" || filepath.Base(fullPath) == "skill.md" {
					singleSkills = append(singleSkills, filepath.Dir(fullPath))
				}
				continue
			}
			// Directory — could be a skill dir or a parent of skill dirs.
			// If it contains SKILL.md, it's a single skill directory.
			if config.RegularFileExists(filepath.Join(fullPath, "SKILL.md")) || config.RegularFileExists(filepath.Join(fullPath, "skill.md")) {
				singleSkills = append(singleSkills, fullPath)
			} else {
				skillDirs = append(skillDirs, fullPath)
			}
		}
	} else {
		// Native layer: check standard skill directory locations.
		candidates := []string{
			filepath.Join(layerPath, "skills"),
			filepath.Join(layerPath, ".claude", "skills"),
			filepath.Join(layerPath, ".cursor", "skills"),
		}
		for _, dir := range candidates {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				skillDirs = append(skillDirs, dir)
			}
		}
	}
	return skillDirs, singleSkills
}

// validateSkills discovers skill directories and checks them against the
// Agent Skills standard. All findings are emitted as warnings (non-blocking).
func validateSkills(result *Result, layerPath string, keelExists bool, cfg *schema.KeelConfig) {
	skillDirs, singleSkills := discoverSkillPaths(layerPath, keelExists, cfg)

	if len(skillDirs) == 0 && len(singleSkills) == 0 {
		return
	}

	skillWarnings := 0
	skillsFound := false

	// Validate parent directories (each subdirectory is a skill).
	for _, dir := range skillDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				skillsFound = true
				break
			}
		}
		warnings := skills.ValidateAll(dir)
		skillWarnings += len(warnings)
		for _, w := range warnings {
			result.addWarn(fmt.Sprintf("skill %s: %s", w.SkillDir, w.Message))
		}
	}

	// Validate individually-referenced skill directories.
	for _, dir := range singleSkills {
		skillsFound = true
		warnings := skills.ValidateDir(dir)
		skillWarnings += len(warnings)
		for _, w := range warnings {
			result.addWarn(fmt.Sprintf("skill %s: %s", w.SkillDir, w.Message))
		}
	}

	if skillsFound && skillWarnings == 0 {
		result.addPass("skills: Agent Skills format compliant")
	}
}

// resolveSnippetDirs returns the absolute paths of snippet directories from
// the merged config's Snippets FileRefs. It checks both source and path
// fields, keeping only existing directories.
func resolveSnippetDirs(layerPath string, refs []schema.FileRef) []string {
	var dirs []string
	for _, ref := range refs {
		target := ref.Source
		if target == "" {
			continue
		}
		fullPath := filepath.Join(layerPath, target)
		info, err := os.Stat(fullPath)
		if err != nil || !info.IsDir() {
			continue
		}
		dirs = append(dirs, fullPath)
	}
	return dirs
}

// collectSnippetNames walks snippet directories and collects defined snippet
// names (filename without .md extension) mapped to their file paths.
func collectSnippetNames(snippetDirs []string) map[string]string {
	names := make(map[string]string)
	for _, dir := range snippetDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			names[name] = filepath.Join(dir, e.Name())
		}
	}
	return names
}

// collectMarkdownFiles walks the layer's content directories (instructions,
// commands, agents, skills, rules) and returns paths of all .md files.
func collectMarkdownFiles(layerPath string, cfg *schema.KeelConfig) []string {
	var mdFiles []string

	// Walk each content category's directories.
	categories := [][]schema.FileRef{
		cfg.Instructions,
		cfg.Commands,
		cfg.Agents,
		cfg.Skills,
		cfg.Rules,
	}
	for _, refs := range categories {
		for _, ref := range refs {
			target := ref.Source
			if target == "" {
				continue
			}
			fullPath := filepath.Join(layerPath, target)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}
			if !info.IsDir() {
				if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
					mdFiles = append(mdFiles, fullPath)
				}
				continue
			}
			// Walk directory for .md files.
			_ = filepath.Walk(fullPath, func(path string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return nil
				}
				if !fi.IsDir() && strings.HasSuffix(strings.ToLower(fi.Name()), ".md") {
					mdFiles = append(mdFiles, path)
				}
				return nil
			})
		}
	}
	return mdFiles
}

// scanIncludeMarkers scans markdown content for keel:include markers and
// returns a map of snippet name to a list of parameter maps (one per
// include site). Markers without parameters get an empty map entry.
func scanIncludeMarkers(content []byte) map[string][]map[string]string {
	refs := make(map[string][]map[string]string)
	lines := bytes.Split(content, []byte("\n"))
	for _, line := range lines {
		matches := merge.IncludeMarkerRe.FindSubmatch(line)
		if matches == nil {
			continue
		}
		name := string(matches[2])
		paramStr := string(matches[3])
		params := make(map[string]string)
		if strings.TrimSpace(paramStr) != "" {
			// Reuse the same paramRe pattern as merge package.
			// The param regex is simple enough to replicate inline.
			for _, m := range merge.ParamRe.FindAllStringSubmatch(paramStr, -1) {
				params[m[1]] = m[2]
			}
		}
		refs[name] = append(refs[name], params)
	}
	return refs
}

// checkSnippetPlaceholders validates that snippet placeholders have matching
// parameters at every include site. Emits a warning for each unresolved
// placeholder.
func checkSnippetPlaceholders(result *Result, layerPath string, snippetDirs []string, cfg *schema.KeelConfig) {
	snippetFiles := collectSnippetNames(snippetDirs)
	if len(snippetFiles) == 0 {
		return
	}

	// Read snippet contents once.
	snippetContents := make(map[string][]byte)
	for name, path := range snippetFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		snippetContents[name] = data
	}

	// Collect all include markers from the layer's markdown files.
	mdFiles := collectMarkdownFiles(layerPath, cfg)

	// Build a map of snippet name -> union of all parameter sets across all
	// include sites. For placeholder checking, we check each include site
	// independently: if any site is missing a parameter, that's a warning.
	for _, mdPath := range mdFiles {
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		markers := scanIncludeMarkers(data)
		for snippetName, paramSets := range markers {
			content, ok := snippetContents[snippetName]
			if !ok {
				continue // undefined snippet — handled by inventory check
			}
			for _, params := range paramSets {
				unresolved := merge.FindUnresolvedPlaceholders(content, params)
				for _, key := range unresolved {
					result.addWarn(fmt.Sprintf("snippet %q has unresolved placeholder {{%s}}", snippetName, key))
				}
			}
		}
	}
}

// checkSnippetInventory cross-references defined snippets against referenced
// snippets to find orphans in either direction.
func checkSnippetInventory(result *Result, layerPath string, snippetDirs []string, cfg *schema.KeelConfig) {
	defined := collectSnippetNames(snippetDirs)

	// Collect all referenced snippet names from markdown files.
	referenced := make(map[string]bool)
	mdFiles := collectMarkdownFiles(layerPath, cfg)
	for _, mdPath := range mdFiles {
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		markers := scanIncludeMarkers(data)
		for name := range markers {
			referenced[name] = true
		}
	}

	// Referenced but not defined.
	for name := range referenced {
		if _, ok := defined[name]; !ok {
			result.addWarn(fmt.Sprintf("snippet %q is referenced but not defined", name))
		}
	}

	// Defined but not referenced.
	for name := range defined {
		if !referenced[name] {
			result.addWarn(fmt.Sprintf("snippet %q is defined but never referenced", name))
		}
	}
}
