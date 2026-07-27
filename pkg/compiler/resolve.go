package compiler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// processDSLLayer parses keel.yml and merges with defaults.
// Returns pure defaults if keel.yml is absent.
func processDSLLayer(layerPath string) (*schema.KeelConfig, error) {
	keelPath := filepath.Join(layerPath, "keel.yml")

	if _, err := os.Stat(keelPath); os.IsNotExist(err) {
		return schema.MergeWithDefaults(nil), nil
	}

	cfg, err := schema.Parse(keelPath)
	if err != nil {
		return nil, err
	}

	return schema.MergeWithDefaults(cfg), nil
}

// resolveInstructionFiles resolves FileRef entries relative to layerPath.
//
// Security note: there is a TOCTOU window between the ContainsPath check
// (which resolves symlinks) and the subsequent file read. An attacker with
// write access to the layer directory could replace a symlink between the
// check and the read. This is a known limitation — exploitation requires
// the attacker to already have significant filesystem access.
func resolveInstructionFiles(layerPath string, refs []schema.FileRef) []string {
	var files []string
	for _, ref := range refs {
		if ref.Source != "" {
			fullPath := filepath.Join(layerPath, ref.Source)
			// Validate path stays within layer (symlink protection)
			if !pathutil.ContainsPath(layerPath, fullPath) {
				slog.Warn("instruction source escapes layer boundary, skipping", "source", ref.Source, "layer", layerPath)
				continue
			}
			info, err := os.Stat(fullPath)
			if err != nil {
				slog.Debug("instruction source not found, skipping", "source", ref.Source, "layer", layerPath)
				continue
			}
			if !info.IsDir() {
				files = append(files, fullPath)
			} else {
				// Directory reference — walk and collect .md files
				entries, err := os.ReadDir(fullPath)
				if err != nil {
					slog.Warn("reading instruction directory", "dir", fullPath, "layer", layerPath, "error", err)
					continue
				}
				// Sort by name for deterministic order
				sort.Slice(entries, func(i, j int) bool {
					return entries[i].Name() < entries[j].Name()
				})
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
						files = append(files, filepath.Join(fullPath, entry.Name()))
					}
				}
			}
		}
	}
	return files
}

// resolvedRef holds the result of resolving a single FileRef entry.
type resolvedRef struct {
	sourcePath string // absolute path to file
	isDir      bool   // true if path: reference (directory)
}

// hookFile is a resolved hook JSON file with its relative path for provenance.
type hookFile struct {
	path string // absolute path to the .json file
	rel  string // relative path from layerPath (for manifest records)
}

// resolveHookFiles resolves hook FileRef entries to individual .json files.
// Source refs point directly to a file or directory. For directories,
// all .json files within are discovered. Non-.json files are skipped.
func resolveHookFiles(layerPath string, refs []schema.FileRef) []hookFile {
	var result []hookFile
	for _, ref := range refs {
		if ref.Source != "" {
			fullPath := filepath.Join(layerPath, ref.Source)
			if !pathutil.ContainsPath(layerPath, fullPath) {
				slog.Warn("hook source escapes layer boundary, skipping", "source", ref.Source, "layer", layerPath)
				continue
			}
			info, err := os.Stat(fullPath)
			if err != nil {
				slog.Debug("hook source not found, skipping", "source", ref.Source, "layer", layerPath)
				continue
			}
			if !info.IsDir() {
				result = append(result, hookFile{path: fullPath, rel: ref.Source})
			} else {
				// Directory reference — enumerate .json files
				entries, err := os.ReadDir(fullPath)
				if err != nil {
					slog.Debug("hook directory cannot be read, skipping", "path", fullPath, "layer", layerPath)
					continue
				}
				for _, e := range entries {
					if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
						continue
					}
					filePath := filepath.Join(fullPath, e.Name())
					rel := filepath.Join(ref.Source, e.Name())
					result = append(result, hookFile{path: filePath, rel: rel})
				}
			}
		}
	}
	return result
}

// resolveFileRefs resolves FileRef entries relative to layerPath.
func resolveFileRefs(layerPath string, refs []schema.FileRef) []resolvedRef {
	var resolved []resolvedRef
	for _, ref := range refs {
		if ref.Source != "" {
			fullPath := filepath.Join(layerPath, ref.Source)
			// Validate path stays within layer (symlink protection)
			if !pathutil.ContainsPath(layerPath, fullPath) {
				slog.Warn("file source escapes layer boundary, skipping", "source", ref.Source, "layer", layerPath)
				continue
			}
			info, err := os.Stat(fullPath)
			if err != nil {
				slog.Debug("file source not found, skipping", "source", ref.Source, "layer", layerPath)
				continue
			}
			resolved = append(resolved, resolvedRef{
				sourcePath: fullPath,
				isDir:      info.IsDir(),
			})
		}
	}
	return resolved
}

// readHooksJSON reads a hooks.json file and returns the parsed map.
// The format is: {"EventName": [{"matcher": "...", "hooks": [...], "force": true}]}
// Returns nil if the file does not exist.
func readHooksJSON(path string) (map[string]any, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := pathutil.SafeReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading hooks file %q: %w", path, err)
	}

	var hooks map[string]any
	if err := json.Unmarshal(data, &hooks); err != nil {
		return nil, fmt.Errorf("parsing hooks JSON %q: %w", path, err)
	}

	// Remove _keel governance metadata so it is not treated as an event name.
	// Force-lock detection uses per-entry "force" fields, not the _keel key.
	delete(hooks, "_keel")

	return hooks, nil
}

// readAndMergePermissions reads permissions.json files referenced by FileRef
// entries and deep-merges them. Each file has format: {"allow": [...], "deny": [...]}.
// _keel policies are stripped from each file and used to control merge behavior
// (e.g., additive array union when _keel.merge is "additive").
// Returns the merged permissions map (may be empty) and any extracted policies
// (re-keyed under the given parentPath for use in cross-layer merges).
func readAndMergePermissions(layerPath string, refs []schema.FileRef) (map[string]any, map[string]policy.KeelPolicy) {
	merged := make(map[string]any)
	// Accumulate policies across files so earlier layers' policies persist.
	allPolicies := make(map[string]policy.KeelPolicy)
	for _, ref := range refs {
		if ref.Source == "" {
			continue
		}

		fullPath := filepath.Join(layerPath, ref.Source)

		// Validate path stays within layer
		if !pathutil.ContainsPath(layerPath, fullPath) {
			slog.Warn("permissions source escapes layer boundary, skipping", "source", ref.Source, "layer", layerPath)
			continue
		}

		data, err := pathutil.SafeReadFile(fullPath)
		if err != nil {
			slog.Debug("permissions file not found or too large, skipping", "path", fullPath, "error", err)
			continue
		}

		var perms map[string]any
		if err := json.Unmarshal(data, &perms); err != nil {
			slog.Warn("parsing permissions JSON", "path", fullPath, "error", err)
			continue
		}

		// Strip _keel policies and accumulate them
		cleaned, policies, err := merge.StripKeelPolicies(perms)
		if err != nil {
			slog.Warn("stripping _keel from permissions", "path", fullPath, "error", err)
			continue
		}
		for k, v := range policies {
			allPolicies[k] = v
		}

		// Merge with policy-aware strategy (e.g., additive array union)
		merged = merge.DeepMergeMapsWithPolicies(merged, cleaned, allPolicies)
	}
	return merged, allPolicies
}

// resolveRemoteFileRefs scans a KeelConfig for FileRef entries with remote
// HTTPS URLs in their Source field and downloads them to the cache. The Source
// field is replaced in-place with the local cache path so that downstream
// compilation sees only local paths.
//
// Only Source fields starting with "https://" are resolved. Entries with Path
// set (directories) are skipped — remote directories are not supported for
// individual FileRef sources. Empty Source fields are also skipped.
func resolveRemoteFileRefs(cfg *schema.KeelConfig, cacheBase string, offline bool) error {
	// Collect all FileRef slices that may contain remote URLs.
	refSlices := [][]schema.FileRef{
		cfg.Instructions,
		cfg.Commands,
		cfg.Agents,
		cfg.Skills,
		cfg.Plugins,
		cfg.AgentMemory,
		cfg.Rules,
		cfg.Hooks,
		cfg.Environment,
		cfg.Init,
		cfg.Bin,
	}
	if cfg.MCP != nil {
		refSlices = append(refSlices, cfg.MCP.Servers)
	}
	if cfg.Permissions != nil {
		refSlices = append(refSlices, cfg.Permissions.Rules)
	}

	for _, refs := range refSlices {
		for i := range refs {
			if err := resolveFileRefSource(&refs[i], cacheBase, offline); err != nil {
				return err
			}
		}
	}
	return nil
}

// isRemoteSource returns true if the source string looks like a remote URL
// (starts with https:// or http://). FetchFile enforces HTTPS at download
// time; this function only classifies the source for dispatch.
func isRemoteSource(source string) bool {
	lower := strings.ToLower(source)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")
}

// resolveFileRefSource resolves a single FileRef source if it is a remote URL.
func resolveFileRefSource(ref *schema.FileRef, cacheBase string, offline bool) error {
	// Skip empty or local entries.
	if ref.Source == "" {
		return nil
	}
	if !isRemoteSource(ref.Source) {
		return nil
	}

	cachedPath, err := remote.FetchFile(ref.Source, cacheBase, offline)
	if err != nil {
		return fmt.Errorf("fetching remote file ref %q: %w", ref.Source, err)
	}

	slog.Debug("resolved remote FileRef", "url", ref.Source, "cached", cachedPath)
	ref.Source = cachedPath
	return nil
}

// readSettingsWithFallbacks tries the primary path and then each
// fallback in order, returning the first file found. Used by convention layers
// where settings.json or cli-config.json may exist instead of the agent-prefixed name.
func readSettingsWithFallbacks(layerPath string, primary string, fallbacks ...string) map[string]any {
	if data := readExternalSettingsJSON(layerPath, primary); data != nil {
		return data
	}
	for _, fb := range fallbacks {
		if data := readExternalSettingsJSON(layerPath, fb); data != nil {
			return data
		}
	}
	return nil
}

// readExternalSettingsJSON reads an external settings JSON file (e.g.
// settings.claude.json or settings.cursor.json) and returns its contents
// as a map. Returns nil if the file does not exist or cannot be parsed.
func readExternalSettingsJSON(layerPath, relPath string) map[string]any {
	if relPath == "" {
		return nil
	}

	fullPath := filepath.Join(layerPath, relPath)

	// Validate path stays within layer
	if !pathutil.ContainsPath(layerPath, fullPath) {
		slog.Warn("settings file escapes layer boundary, skipping", "path", relPath, "layer", layerPath)
		return nil
	}

	data, err := pathutil.SafeReadFile(fullPath)
	if err != nil {
		slog.Debug("settings file not found or too large, skipping", "path", fullPath, "error", err)
		return nil
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		slog.Warn("parsing settings JSON", "path", fullPath, "error", err)
		return nil
	}

	// Strip _keel governance policies — resolved during merge, not stored in IR.
	cleaned, _, err := merge.StripKeelPolicies(settings)
	if err != nil {
		slog.Warn("stripping _keel policies failed, using raw data", "error", err)
		return settings
	}

	return cleaned
}
