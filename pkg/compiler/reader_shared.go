package compiler

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// ReadJSONConfig reads a JSON config file from the first matching candidate
// path and unmarshals it. Returns nil, nil if no candidate exists.
// This is the shared pattern used by native readers for settings, MCP, hooks,
// and other JSON config files.
func ReadJSONConfig(candidates []string) (map[string]any, error) {
	var data []byte
	var err error
	for _, path := range candidates {
		data, err = pathutil.SafeReadFile(path)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	if data == nil {
		return nil, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return raw, nil
}

// ReadJSONConfigStripped reads a JSON config file and strips _keel governance
// policies from the result. Use this for settings files where _keel policies
// should be processed and removed before decomposition. For MCP files where
// _keel policies must be preserved for the accumulator's force-lock system,
// use ReadJSONConfig instead.
func ReadJSONConfigStripped(candidates []string) (map[string]any, error) {
	raw, err := ReadJSONConfig(candidates)
	if err != nil || raw == nil {
		return raw, err
	}

	cleaned, _, stripErr := merge.StripKeelPolicies(raw)
	if stripErr != nil {
		slog.Warn("stripping _keel policies failed, using raw data", "error", stripErr)
	} else {
		raw = cleaned
	}

	return raw, nil
}

// SettingsReadCandidates builds the full candidate path list for settings files
// in a native layer, deriving from the profile's SettingsCandidates. For each
// candidate name, it checks dotDir first (higher priority), then the layer root.
// This ensures readers and profiles stay in sync — new settings filenames added
// to the profile are automatically picked up by the reader.
func SettingsReadCandidates(layerPath, dotDir string, profileCandidates []string) []string {
	candidates := make([]string, 0, len(profileCandidates)*2)
	for _, name := range profileCandidates {
		candidates = append(candidates, filepath.Join(layerPath, dotDir, name))
	}
	for _, name := range profileCandidates {
		candidates = append(candidates, filepath.Join(layerPath, name))
	}
	return candidates
}

// ReadSnippets loads snippet files from a native layer's snippet directories.
// It checks snippets/ at the layer root and <dotDir>/snippets/.
// dotDir is the agent's dot-directory name (e.g., ".claude", ".cursor").
func ReadSnippets(layerPath, dotDir string, ir *LayerIR) {
	snippets := make(map[string][]byte)
	for _, snippetDir := range []string{
		filepath.Join(layerPath, "snippets"),
		filepath.Join(layerPath, dotDir, "snippets"),
	} {
		if info, err := os.Stat(snippetDir); err == nil && info.IsDir() {
			LoadSnippetsFromDir(snippetDir, snippets)
		}
	}
	if len(snippets) > 0 {
		ir.Snippets = snippets
	}
}

// ReadOtherDotDirFiles reads arbitrary files from the agent's dot-directory
// that aren't handled by specific reader methods. knownCategories lists
// directory names to skip (they're handled by readCategory or snippets).
// excludeFiles lists filenames in the dot-directory root to skip (they're
// handled by dedicated readers like readSettings or readMCP).
// Matching files are stored under the CategoryDotDir category key.
func ReadOtherDotDirFiles(layerPath, dotDir string, excludeFiles map[string]bool, knownCategories []string, ir *LayerIR) {
	dotDirPath := filepath.Join(layerPath, dotDir)
	info, err := os.Stat(dotDirPath)
	if err != nil || !info.IsDir() {
		return
	}

	excludeDirs := make(map[string]bool, len(knownCategories))
	for _, cat := range knownCategories {
		excludeDirs[cat] = true
	}

	var files []CategoryFile
	_ = filepath.WalkDir(dotDirPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(dotDirPath, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}

		// Skip excluded directories.
		topDir := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		if excludeDirs[topDir] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			files = append(files, CategoryFile{RelPath: rel, IsDir: true})
			return nil
		}

		// Skip excluded files.
		if excludeFiles[rel] {
			return nil
		}

		data, readErr := pathutil.SafeReadFile(path)
		if readErr != nil {
			return nil
		}
		files = append(files, CategoryFile{RelPath: rel, Content: data})
		return nil
	})

	if len(files) > 0 {
		if ir.Categories == nil {
			ir.Categories = make(map[string][]CategoryFile)
		}
		ir.Categories[CategoryDotDir] = files
	}
}

// ReadBinDir reads bin/ files from a layer directory into ir.Bin.
// Mode is left at 0 so the writer applies the default 0755 (executable).
func ReadBinDir(layerPath string, ir *LayerIR) {
	binDir := filepath.Join(layerPath, "bin")
	info, err := os.Stat(binDir)
	if err != nil || !info.IsDir() {
		return
	}

	_ = filepath.WalkDir(binDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel, relErr := filepath.Rel(binDir, path)
		if relErr != nil {
			return relErr
		}
		data, readErr := pathutil.SafeReadFile(path)
		if readErr != nil {
			return nil
		}
		ir.Bin = append(ir.Bin, BinFile{
			RelPath: rel,
			Content: data,
			// Mode intentionally left at 0 — writer defaults to 0755.
		})
		return nil
	})
}

// ReadKeelYmlSidecar reads the keel.yml sidecar from a native layer directory
// to extract file policies and metadata. Native layers (v1-claude, v1-cursor)
// can have a keel.yml with a `files:` section for governance and `name:` for metadata.
func ReadKeelYmlSidecar(layerPath string, ir *LayerIR) {
	keelYml := filepath.Join(layerPath, "keel.yml")
	if _, err := os.Stat(keelYml); err != nil {
		return
	}
	cfg, err := schema.Parse(keelYml)
	if err != nil {
		slog.Debug("failed to parse keel.yml sidecar", "path", keelYml, "error", err)
		return
	}
	if len(cfg.Files) > 0 {
		ir.FilePolicies = cfg.Files
	}
	if cfg.Name != "" {
		ir.Name = cfg.Name
	}
	if cfg.Description != "" {
		ir.Description = cfg.Description
	}
	for k, v := range cfg.Repos {
		if ir.Repos == nil {
			ir.Repos = make(map[string]schema.Repo)
		}
		ir.Repos[k] = v
	}
}

// ReadCategoryDir reads all files from a category directory and populates
// ir.Categories with the results. This is the shared implementation used by
// both ClaudeReader and CursorReader — the only difference between them was
// the dot-directory prefix, which is resolved by the caller before passing
// catDir.
func ReadCategoryDir(catDir, category string, ir *LayerIR) error {
	info, err := os.Stat(catDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	var files []CategoryFile
	walkErr := filepath.WalkDir(catDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(catDir, path)
		if relErr != nil {
			return relErr
		}
		// Skip the root directory itself
		if rel == "." {
			return nil
		}

		if d.IsDir() {
			files = append(files, CategoryFile{
				RelPath: rel,
				IsDir:   true,
			})
			return nil
		}

		content, readErr := pathutil.SafeReadFile(path)
		if readErr != nil {
			return readErr
		}

		files = append(files, CategoryFile{
			RelPath: rel,
			Content: content,
		})
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	if len(files) > 0 {
		if ir.Categories == nil {
			ir.Categories = make(map[string][]CategoryFile)
		}
		ir.Categories[category] = files
		slog.Debug("read category directory", "category", category, "files", len(files), "layer", catDir)
	}

	return nil
}

// SimpleReader is a NativeReader for instructions-only agents (aider, cline,
// copilot, agentsmd). It reads the first matching instruction file and the
// keel.yml sidecar. Agents with richer native formats (claude, cursor)
// implement NativeReader directly.
type SimpleReader struct {
	Agent      string   // agent name (e.g., "aider")
	Candidates []string // instruction file candidates in priority order
}

func (r *SimpleReader) AgentName() string { return r.Agent }

func (r *SimpleReader) Read(layerPath string) (*LayerIR, error) {
	ir := &LayerIR{
		SourceAgent: r.Agent,
		LayerPath:   layerPath,
	}
	ReadInstructions(layerPath, r.Candidates, ir)
	ReadKeelYmlSidecar(layerPath, ir)
	return ir, nil
}

// ReadInstructions looks for instruction files in priority order and reads
// the first one found into ir.Instructions. Shared by all native readers.
func ReadInstructions(layerPath string, candidates []string, ir *LayerIR) {
	for _, name := range candidates {
		path := filepath.Join(layerPath, name)
		data, err := pathutil.SafeReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			slog.Debug("could not read instruction file", "path", path, "error", err)
			continue
		}
		slog.Debug("read instruction file", "file", name, "layer", layerPath)
		ir.Instructions = []InstructionBlock{
			{Filename: name, Content: string(data)},
		}
		return
	}
}

// ReadCategories reads category files from both dotDir/<category> and
// <root>/<category>. The dot-directory is checked first; root is a fallback.
func ReadCategories(layerPath, dotDir string, categories []string, ir *LayerIR) error {
	for _, cat := range categories {
		if err := ReadCategoryDir(filepath.Join(layerPath, dotDir, cat), cat, ir); err != nil {
			return err
		}
		if _, exists := ir.Categories[cat]; !exists {
			if err := ReadCategoryDir(filepath.Join(layerPath, cat), cat, ir); err != nil {
				return err
			}
		}
	}
	return nil
}
