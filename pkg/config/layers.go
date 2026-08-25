// Package config handles layer resolution and format detection for keel configuration.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
	"gopkg.in/yaml.v3"
)

// LayerFormat represents the detected format of a config layer.
type LayerFormat string

// FormatKeel is the Keel DSL format.
const FormatKeel LayerFormat = "keel"

// AgentMeta defines detection and identification data for an agent format.
// Add new agents here -- detection, compilation, and validation derive from this.
type AgentMeta struct {
	Format LayerFormat // format identifier, also used as agent name
	Files  []string    // marker files for auto-detection (e.g. "CLAUDE.md")
	Dirs   []string    // marker directories for auto-detection (e.g. ".claude")
}

// agents is the registry of supported agents. Populated via RegisterAgent()
// calls from init() functions, following the same pattern as RegisterCompiler.
// Access via AgentList() for a read-only copy.
var agents []AgentMeta

// RegisterAgent registers agent metadata for format detection and validation.
// Call this from init() in each adapter file alongside RegisterCompiler.
func RegisterAgent(meta AgentMeta) {
	agents = append(agents, meta)
}

// AgentList returns a copy of the registered agent metadata.
// The returned slice is safe to iterate without affecting the registry.
func AgentList() []AgentMeta {
	cp := make([]AgentMeta, len(agents))
	copy(cp, agents)
	return cp
}

// AgentNames returns supported agent names.
func AgentNames() []string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = string(a.Format)
	}
	return names
}

// IsAgentFormat reports whether f belongs to a registered agent.
func IsAgentFormat(f LayerFormat) bool {
	for _, a := range agents {
		if a.Format == f {
			return true
		}
	}
	return false
}

// LookupAgent returns the AgentMeta for the named agent, or nil if not found.
func LookupAgent(name string) *AgentMeta {
	for i := range agents {
		if string(agents[i].Format) == name {
			return &agents[i]
		}
	}
	return nil
}

// Layer represents a single config layer with its path and metadata.
type Layer struct {
	Path    string
	Type    string // "base", "project", "user"
	Format  LayerFormat
	Version string      // schema version from keel.yml (e.g. "v1")
	Source  string      // Original remote URL, empty for local layers
	Filter  *LayerFilter // Optional filter for this layer
}

// ResolveLayers resolves layer paths, inserts user config based on priority,
// and returns the ordered layer slice.
// Remote segments (Git URLs, HTTPS archives) are fetched to the local cache.
// The offline flag prevents network access; cached data is used if available.
// The maxRemoteLayers parameter limits the number of remote layers (0 = default of 50, -1 = unlimited).
func ResolveLayers(layerEntries []LayerEntry, userConfig, userPriority string, offline bool, maxRemoteLayers int) ([]Layer, error) {
	var layers []Layer

	// Build list of paths and filters from layer entries.
	type pathWithFilter struct {
		path   string
		filter *LayerFilter
	}
	var configPaths []pathWithFilter
	for _, entry := range layerEntries {
		p := strings.TrimSpace(entry.Source)
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "/*") && remote.Classify(p) == remote.SourceLocal {
			expanded, err := expandGlob(p)
			if err != nil {
				return nil, err
			}
			// Glob expansion replicates the filter to all expanded paths.
			for _, exp := range expanded {
				configPaths = append(configPaths, pathWithFilter{exp, entry.Filter})
			}
		} else {
			configPaths = append(configPaths, pathWithFilter{p, entry.Filter})
		}
	}

	// Set default limit if maxRemoteLayers is 0.
	const defaultMaxRemoteLayers = 50
	limit := maxRemoteLayers
	if limit == 0 {
		limit = defaultMaxRemoteLayers
	}

	// Resolve remote segments to local cache paths.
	cacheDir := remote.CacheDir()
	type resolvedSegment struct {
		localPath   string
		originalURL string
		filter      *LayerFilter
	}
	var resolved []resolvedSegment
	remoteCount := 0
	for _, pf := range configPaths {
		localPath, originalURL, err := remote.Resolve(pf.path, cacheDir, offline)
		if err != nil {
			return nil, fmt.Errorf("resolving layer source %q: %w", pf.path, err)
		}

		// Count remote layers (originalURL is non-empty for git:// and https://).
		if originalURL != "" {
			remoteCount++
			// Enforce limit unless set to -1 (unlimited).
			if limit != -1 && remoteCount > limit {
				return nil, fmt.Errorf("exceeded maximum remote layer count (%d); increase with --max-remote-layers or reduce remote layer dependencies", limit)
			}
		}

		// For remote sources without explicit subpath, check for keel/ convention.
		if originalURL != "" {
			localPath = applyKeelDirFallback(localPath, pf.path)
		}

		resolved = append(resolved, resolvedSegment{localPath, originalURL, pf.filter})
	}

	// Validate resolved paths exist and are directories.
	for _, r := range resolved {
		info, err := os.Stat(r.localPath)
		if err != nil {
			return nil, fmt.Errorf("layer path %q does not exist: %w", r.localPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("layer path %q is not a directory", r.localPath)
		}
	}

	// Check user config exists
	userConfigExists := false
	if userConfig != "" {
		if info, err := os.Stat(userConfig); err == nil && info.IsDir() {
			userConfigExists = true
		}
	}

	// Build ordered layer list
	for _, r := range resolved {
		format, version := DetectLayerFormat(r.localPath)
		layers = append(layers, Layer{
			Path:    r.localPath,
			Type:    classifyLayerType(r.localPath),
			Format:  format,
			Version: version,
			Source:  r.originalURL,
			Filter:  r.filter,
		})
	}

	// User config: low priority = prepend, high priority = append
	if userConfigExists {
		format, version := DetectLayerFormat(userConfig)
		userLayer := Layer{
			Path:    userConfig,
			Type:    "user",
			Format:  format,
			Version: version,
		}
		if strings.ToLower(userPriority) == "low" {
			layers = append([]Layer{userLayer}, layers...)
		} else {
			layers = append(layers, userLayer)
		}
	}

	return layers, nil
}

// DetectLayerFormat determines a layer's format and version.
// Only the keel.yml schema field drives format detection; a keel.yml
// present for metadata alone does not imply DSL format.
//
// Resolution order:
//  1. keel.yml schema field → agent format + version (from agents registry)
//  2. Agent marker files    → agent format (no version)
//  3. Fallback              → FormatKeel (DSL with defaults)
func DetectLayerFormat(path string) (LayerFormat, string) {
	// 1. Authoritative: keel.yml schema field
	keelPath := filepath.Join(path, "keel.yml")
	if RegularFileExists(keelPath) {
		data, err := pathutil.SafeReadFile(keelPath)
		if err != nil {
			return FormatKeel, ""
		}
		var partial struct {
			Schema string `yaml:"schema"`
		}
		if err := yaml.Unmarshal(data, &partial); err != nil {
			return FormatKeel, ""
		}

		// Parse "v1-claude" → version="v1", agent="claude"
		version, agent, _ := strings.Cut(partial.Schema, "-")
		if version == "" {
			version = "v1"
		}

		// Look up agent in registry
		for _, a := range agents {
			if string(a.Format) == agent {
				return a.Format, version
			}
		}
		return FormatKeel, version
	}

	// 2. Detect agent by marker files
	for _, a := range agents {
		for _, f := range a.Files {
			if RegularFileExists(filepath.Join(path, f)) {
				return a.Format, ""
			}
		}
		for _, d := range a.Dirs {
			if info, err := os.Stat(filepath.Join(path, d)); err == nil && info.IsDir() {
				return a.Format, ""
			}
		}
	}

	// 3. No keel.yml, no markers — assume keel DSL with defaults
	return FormatKeel, ""
}

// classifyLayerType infers the layer type from its path.
func classifyLayerType(path string) string {
	if filepath.Base(path) == "base" {
		return "base"
	}
	return "project"
}

// validateAgentMarkers checks that no two agents share marker files or directories.
// Call this from tests to catch registration-time ambiguity.
func validateAgentMarkers() error {
	seen := make(map[string]string) // marker -> agent format
	for _, a := range agents {
		name := string(a.Format)
		for _, f := range a.Files {
			if prev, ok := seen[f]; ok {
				return fmt.Errorf("marker file %q is registered by both %q and %q", f, prev, name)
			}
			seen[f] = name
		}
		for _, d := range a.Dirs {
			key := "dir:" + d
			if prev, ok := seen[key]; ok {
				return fmt.Errorf("marker directory %q is registered by both %q and %q", d, prev, name)
			}
			seen[key] = name
		}
	}
	return nil
}

// RegularFileExists checks if a path exists and is a regular file (not a directory).
func RegularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// expandGlob returns sorted child directories for path ending in /*.
// Non-directory entries are skipped.
// If the parent directory doesn't exist, returns an empty slice (not an error).
func expandGlob(pattern string) ([]string, error) {
	parent := strings.TrimSuffix(pattern, "/*")

	info, err := os.Stat(parent)
	if err != nil {
		// Parent doesn't exist — not an error, just no layers
		return nil, nil
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("glob parent %q is not a directory", parent)
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, fmt.Errorf("reading glob directory %q: %w", parent, err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			paths = append(paths, filepath.Join(parent, e.Name()))
		}
	}
	// os.ReadDir returns entries sorted by name, so paths are already alphabetical
	return paths, nil
}

// applyKeelDirFallback checks whether a resolved remote layer path should
// fall back to a keel/ subdirectory. This implements the convention that
// repos store shared agent-agnostic config in a keel/ directory.
//
// The fallback only activates when:
//   - The original source had no explicit subpath (e.g., no ":path" in the fragment)
//   - The resolved root is not itself a recognized layer format
//   - A keel/ subdirectory exists and IS a recognized layer format
//
// If any condition is not met, the original path is returned unchanged.
func applyKeelDirFallback(resolvedPath, originalSource string) string {
	// Check if original source specified an explicit subpath.
	src, err := remote.Parse(originalSource)
	if err != nil || src.SubPath != "" {
		return resolvedPath
	}

	// Check if root is already a recognized layer.
	rootFormat, _ := DetectLayerFormat(resolvedPath)
	if isRecognizedLayer(resolvedPath, rootFormat) {
		return resolvedPath
	}

	// Check for keel/ subdirectory.
	keelDir := filepath.Join(resolvedPath, "keel")
	keelFormat, _ := DetectLayerFormat(keelDir)
	if isRecognizedLayer(keelDir, keelFormat) {
		slog.Info("using keel/ subdirectory as layer root", "source", originalSource)
		return keelDir
	}

	return resolvedPath
}

// isRecognizedLayer returns true if the directory contains marker files
// that indicate it is a configuration layer (keel.yml, CLAUDE.md, .claude/, .cursor/, etc.).
func isRecognizedLayer(path string, format LayerFormat) bool {
	// Non-default format means a specific marker was found
	if format != FormatKeel {
		return true
	}
	// For FormatKeel (the default/fallback), check if keel.yml actually exists
	return RegularFileExists(filepath.Join(path, "keel.yml"))
}
