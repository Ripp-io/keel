package compiler

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

func init() {
	RegisterReader("keel", func() NativeReader { return &KeelDSLReader{} })
}

// KeelDSLReader reads a keel.yml DSL layer directory and produces a LayerIR
// with all content resolved (FileRefs to actual bytes). It performs no disk
// writes, no merging, and no force-lock logic.
type KeelDSLReader struct {
	Offline bool // prevents network access for remote FileRef resolution
}

func (r *KeelDSLReader) AgentName() string { return "keel" }

// Read reads a keel.yml layer at layerPath and returns a normalized LayerIR.
// Missing files/directories produce nil/empty fields, not errors.
func (r *KeelDSLReader) Read(layerPath string) (*LayerIR, error) {
	cfg, err := processDSLLayer(layerPath)
	if err != nil {
		return nil, fmt.Errorf("parsing keel.yml at %q: %w", layerPath, err)
	}

	// Resolve remote FileRefs (downloads to cache, rewrites Source in-place).
	cacheDir := remote.CacheDir()
	if err := resolveRemoteFileRefs(cfg, cacheDir, r.Offline); err != nil {
		return nil, fmt.Errorf("resolving remote file refs: %w", err)
	}

	ir := &LayerIR{
		SourceAgent: "keel",
		LayerPath:   layerPath,
		DSLConfig:   cfg,
	}

	// Metadata
	ir.Name = cfg.Name
	ir.Description = cfg.Description
	if len(cfg.Repos) > 0 {
		ir.Repos = cfg.Repos
	}

	// Simple pass-through fields
	if len(cfg.Variables) > 0 {
		ir.Variables = cfg.Variables
	}
	if len(cfg.Deps) > 0 {
		ir.Deps = cfg.Deps
	}
	if len(cfg.Files) > 0 {
		ir.FilePolicies = cfg.Files
	}

	// Instructions
	r.readInstructions(layerPath, cfg, ir)

	// Categories (commands, agents, skills, plugins, agent-memory, rules)
	r.readCategories(layerPath, cfg, ir)

	// MCP
	r.readMCP(layerPath, cfg, ir)

	// Hooks
	r.readHooks(layerPath, cfg, ir)

	// Permissions
	r.readPermissions(layerPath, cfg, ir)

	// Settings
	r.readSettings(layerPath, cfg, ir)

	// Environment
	r.readEnvironment(layerPath, cfg, ir)

	// Snippets
	r.readSnippets(layerPath, cfg, ir)

	// Bin
	r.readBin(layerPath, cfg, ir)

	// Loop
	r.readLoop(layerPath, cfg, ir)

	// WorktreeInclude
	r.readWorktreeInclude(layerPath, cfg, ir)

	return ir, nil
}

// readInstructions resolves instruction FileRefs and reads their content.
func (r *KeelDSLReader) readInstructions(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	paths := resolveInstructionFiles(layerPath, cfg.Instructions)
	for _, p := range paths {
		data, err := pathutil.SafeReadFile(p)
		if err != nil {
			slog.Debug("could not read instruction file", "path", p, "error", err)
			continue
		}
		ir.Instructions = append(ir.Instructions, InstructionBlock{
			Filename: filepath.Base(p),
			Content:  string(data),
		})
	}
}

// categoryMapping pairs a DSL FileRef slice with the canonical category name.
type categoryMapping struct {
	name string
	refs []schema.FileRef
}

// readCategories resolves category FileRefs (commands, agents, skills, etc.)
// and reads their content into ir.Categories.
func (r *KeelDSLReader) readCategories(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	mappings := []categoryMapping{
		{"commands", cfg.Commands},
		{"agents", cfg.Agents},
		{"skills", cfg.Skills},
		{"plugins", cfg.Plugins},
		{"agent-memory", cfg.AgentMemory},
		{"rules", cfg.Rules},
		{"init", cfg.Init},
	}

	for _, m := range mappings {
		if len(m.refs) == 0 {
			continue
		}
		r.readCategoryRefs(layerPath, m.name, m.refs, ir)
	}
}

// readCategoryRefs resolves FileRef entries for a single category and populates
// ir.Categories. For directory refs, it walks the directory. For file refs, it
// reads the single file. Force flags from FileRefs are propagated to
// ir.FilePolicies so the accumulator can register force locks.
func (r *KeelDSLReader) readCategoryRefs(layerPath, category string, refs []schema.FileRef, ir *LayerIR) {
	for i, fileRef := range refs {
		resolved := resolveFileRefs(layerPath, []schema.FileRef{fileRef})
		for _, ref := range resolved {
			if ref.isDir {
				// Use the existing ReadCategoryDir helper for directory refs.
				if err := ReadCategoryDir(ref.sourcePath, category, ir); err != nil {
					slog.Debug("reading category directory", "category", category, "dir", ref.sourcePath, "error", err)
				}
				// If the FileRef has force: true, register force for all files in the dir.
				if refs[i].Force {
					r.registerDirForce(ref.sourcePath, category, ir)
				}
			} else {
				// Single file ref: read and add to category.
				data, err := pathutil.SafeReadFile(ref.sourcePath)
				if err != nil {
					slog.Debug("reading category file", "category", category, "file", ref.sourcePath, "error", err)
					continue
				}
				if ir.Categories == nil {
					ir.Categories = make(map[string][]CategoryFile)
				}
				relPath := filepath.Base(ref.sourcePath)
				ir.Categories[category] = append(ir.Categories[category], CategoryFile{
					RelPath: relPath,
					Content: data,
				})
				// Propagate force: true to FilePolicies.
				if refs[i].Force {
					if ir.FilePolicies == nil {
						ir.FilePolicies = make(map[string]schema.FilePolicy)
					}
					ir.FilePolicies[category+"/"+relPath] = schema.FilePolicy{Force: true}
				}
			}
		}
	}
}

// registerDirForce walks a directory and registers force policies for all files.
func (r *KeelDSLReader) registerDirForce(dir, category string, ir *LayerIR) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		if ir.FilePolicies == nil {
			ir.FilePolicies = make(map[string]schema.FilePolicy)
		}
		ir.FilePolicies[category+"/"+rel] = schema.FilePolicy{Force: true}
		return nil
	})
}

// readMCP reads MCP server JSON files and populates ir.MCP and ir.MCPPolicy.
func (r *KeelDSLReader) readMCP(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	if cfg.MCP == nil || len(cfg.MCP.Servers) == 0 {
		return
	}

	if cfg.MCP.Policy != nil {
		ir.MCPPolicy = cfg.MCP.Policy
	}

	merged := make(map[string]any)
	for _, ref := range cfg.MCP.Servers {
		if ref.Source == "" {
			continue
		}

		fullPath := filepath.Join(layerPath, ref.Source)

		data, err := pathutil.SafeReadFile(fullPath)
		if err != nil {
			slog.Debug("MCP server file not found, skipping", "path", fullPath, "error", err)
			continue
		}

		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			slog.Warn("parsing MCP server JSON", "path", fullPath, "error", err)
			continue
		}

		// Deep merge into accumulated MCP config (consistent with accumulator's MCP merge).
		merged = merge.DeepMergeMapsWithPolicies(merged, parsed, nil)
	}

	if len(merged) > 0 {
		ir.MCP = merged
	}
}

// readHooks resolves hook FileRef entries and reads their JSON content.
// Hook keys are stored in PascalCase (the canonical IR format).
func (r *KeelDSLReader) readHooks(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	hookFiles := resolveHookFiles(layerPath, cfg.Hooks)
	if len(hookFiles) == 0 {
		return
	}

	merged := make(map[string]any)
	for _, hf := range hookFiles {
		data, err := readHooksJSON(hf.path)
		if err != nil {
			slog.Warn("reading hook file", "path", hf.path, "error", err)
			continue
		}
		if data == nil {
			continue
		}
		for k, v := range data {
			// Concatenate arrays for the same event key across files.
			mergeHookEvent(merged, k, v)
		}
	}

	if len(merged) > 0 {
		ir.Hooks = merged
	}
}

// readPermissions reads permissions JSON files and populates ir.Permissions
// and ir.PermissionsPolicy.
func (r *KeelDSLReader) readPermissions(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	if cfg.Permissions == nil {
		return
	}

	if cfg.Permissions.Policy != nil {
		ir.PermissionsPolicy = cfg.Permissions.Policy
	}

	if len(cfg.Permissions.Rules) == 0 {
		return
	}

	perms, permPolicies := readAndMergePermissions(layerPath, cfg.Permissions.Rules)
	if len(perms) > 0 {
		ir.Permissions = perms
	}

	// If permissions.json contained _keel.merge: additive, propagate it
	// as the PermissionsPolicy so the accumulator applies additive union.
	if ir.PermissionsPolicy == nil && len(permPolicies) > 0 {
		for _, p := range permPolicies {
			if p.Merge != "" {
				ir.PermissionsPolicy = &schema.PermissionsPolicy{
					Merge: string(p.Merge),
				}
				break
			}
		}
	}
}

// readEnvironment reads environment JSON files referenced by cfg.Environment into ConfigBlocks.
func (r *KeelDSLReader) readEnvironment(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	if len(cfg.Environment) == 0 {
		return
	}

	merged := make(map[string]any)
	for _, ref := range cfg.Environment {
		if ref.Source == "" {
			continue
		}

		fullPath := filepath.Join(layerPath, ref.Source)

		data, err := pathutil.SafeReadFile(fullPath)
		if err != nil {
			slog.Debug("environment file not found, skipping", "path", fullPath, "error", err)
			continue
		}

		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			slog.Warn("parsing environment JSON", "path", fullPath, "error", err)
			continue
		}

		for k, v := range parsed {
			merged[k] = v
		}
	}

	if len(merged) > 0 {
		if ir.ConfigBlocks == nil {
			ir.ConfigBlocks = make(map[string]map[string]any)
		}
		ir.ConfigBlocks["environment"] = merged
	}
}

// readSnippets loads snippet files from cfg.Snippets FileRefs. Each .md file's
// name (without extension) becomes the snippet name.
func (r *KeelDSLReader) readSnippets(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	if len(cfg.Snippets) == 0 {
		return
	}

	snippets := make(map[string][]byte)
	for _, ref := range cfg.Snippets {
		if ref.Source == "" {
			continue
		}

		fullPath := filepath.Join(layerPath, ref.Source)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			LoadSnippetsFromDir(fullPath, snippets)
		} else if strings.HasSuffix(ref.Source, ".md") {
			registerSnippetFile(fullPath, snippets)
		}
	}

	if len(snippets) > 0 {
		ir.Snippets = snippets
	}
}

// LoadSnippetsFromDir walks a directory and registers all .md files as snippets.
func LoadSnippetsFromDir(dir string, snippets map[string][]byte) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		registerSnippetFile(filepath.Join(dir, entry.Name()), snippets)
	}
}

// registerSnippetFile reads a file and adds it to the snippet map.
func registerSnippetFile(path string, snippets map[string][]byte) {
	data, err := pathutil.SafeReadFile(path)
	if err != nil {
		return
	}
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	snippets[name] = data
}

// readBin reads bin files from cfg.Bin FileRefs. For each file, reads content
// and file mode.
func (r *KeelDSLReader) readBin(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	if len(cfg.Bin) == 0 {
		return
	}

	resolved := resolveFileRefs(layerPath, cfg.Bin)
	for _, ref := range resolved {
		if ref.isDir {
			r.readBinDir(ref.sourcePath, ir)
		} else {
			r.readBinFile(ref.sourcePath, filepath.Base(ref.sourcePath), ir)
		}
	}
}

// readBinDir walks a directory and reads all files as bin entries.
func (r *KeelDSLReader) readBinDir(dir string, ir *LayerIR) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}

		r.readBinFile(path, rel, ir)
		return nil
	})
	if err != nil {
		slog.Debug("walking bin directory", "dir", dir, "error", err)
	}
}

// readBinFile reads a single file and appends it to ir.Bin.
func (r *KeelDSLReader) readBinFile(path, relPath string, ir *LayerIR) {
	data, err := pathutil.SafeReadFile(path)
	if err != nil {
		slog.Debug("reading bin file", "path", path, "error", err)
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		slog.Debug("stat bin file", "path", path, "error", err)
		return
	}

	ir.Bin = append(ir.Bin, BinFile{
		RelPath: relPath,
		Content: data,
		Mode:    info.Mode(),
	})
}

// readLoop resolves loop FileRefs and reads their content into ContentBlocks.
func (r *KeelDSLReader) readLoop(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	r.readContentBlock(layerPath, cfg.Loop, "loop", ir)
}

// readWorktreeInclude resolves worktree-include FileRefs and reads their content into ContentBlocks.
func (r *KeelDSLReader) readWorktreeInclude(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	r.readContentBlock(layerPath, cfg.WorktreeInclude, "worktree-include", ir)
}

// readContentBlock resolves FileRefs and reads their content into ir.ContentBlocks
// under the given block name. Shared by readLoop and readWorktreeInclude.
func (r *KeelDSLReader) readContentBlock(layerPath string, refs []schema.FileRef, blockName string, ir *LayerIR) {
	paths := resolveInstructionFiles(layerPath, refs)
	for _, p := range paths {
		data, err := pathutil.SafeReadFile(p)
		if err != nil {
			slog.Debug("could not read "+blockName+" file", "path", p, "error", err)
			continue
		}
		if ir.ContentBlocks == nil {
			ir.ContentBlocks = make(map[string][]InstructionBlock)
		}
		ir.ContentBlocks[blockName] = append(ir.ContentBlocks[blockName], InstructionBlock{
			Filename: filepath.Base(p),
			Content:  string(data),
		})
	}
}
