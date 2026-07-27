// Package manifest defines post-compilation types and operations for the Keel compiler.
// This includes the compiled manifest structure, content hashing, and cleanup operations.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

const keelStateFile = ".keel.json"

// ManifestSignature holds the cryptographic signature of the compiled manifest.
type ManifestSignature struct {
	Algorithm   string `yaml:"algorithm"     json:"algorithm"`
	KeyID       string `yaml:"key_id"        json:"key_id"`
	Signature   string `yaml:"signature"     json:"signature"`       // base64-encoded
	SignedAt    string `yaml:"signed_at"     json:"signed_at"`       // RFC3339
	ContentHash string `yaml:"content_hash"  json:"content_hash"`    // SHA-256 of manifest JSON without this field
}

// CompiledManifest is the output written to .keel.json after compilation.
// It records what was compiled, from which layers, and what governance
// was applied. This struct is separate from KeelConfig because the two
// serve different roles: KeelConfig is DSL input, CompiledManifest is
// compiler output.
type CompiledManifest struct {
	// Shared metadata (also present in source layers)
	Name        string                 `yaml:"name,omitempty"         json:"name,omitempty"`
	Schema      string                 `yaml:"schema,omitempty"       json:"schema,omitempty"`
	Description string                 `yaml:"description,omitempty"  json:"description,omitempty"`
	Repos       map[string]schema.Repo `yaml:"repos,omitempty"        json:"repos,omitempty"`

	// Manifest-only fields
	Agent       string            `yaml:"agent,omitempty"        json:"agent,omitempty"`
	Scope       string            `yaml:"scope,omitempty"        json:"scope,omitempty"`
	CompiledAt  string            `yaml:"compiled_at,omitempty"  json:"compiled_at,omitempty"`
	KeelVersion string            `yaml:"keel_version,omitempty" json:"keel_version,omitempty"`
	InitDir     string            `yaml:"init_dir,omitempty"     json:"init_dir,omitempty"`
	LayerStack  []LayerRecord     `yaml:"layers,omitempty"       json:"layers,omitempty"`
	FileOps     []FileRecord      `yaml:"file_ops,omitempty"     json:"file_ops,omitempty"`
	ForceLocked []ForceLockRecord `yaml:"force_locked,omitempty" json:"force_locked,omitempty"`

	// InstalledDeps records resolved external binary dependencies.
	InstalledDeps []InstalledDepRecord `yaml:"installed_deps,omitempty" json:"installed_deps,omitempty"`

	// Overlays maps individual overlay output file paths to the layers that
	// wrote them. Used by cleanup to know exactly which files Keel owns in
	// overlay directories (rather than walking the filesystem and claiming
	// everything).
	Overlays map[string][]string `yaml:"overlays,omitempty" json:"overlays,omitempty"`

	// ManagedMCPServers lists the MCP server names that Keel manages in the
	// MCP output file. Used for merge-into files (e.g., ~/.claude.json) where
	// Keel owns only specific servers, not the entire file.
	ManagedMCPServers []string `yaml:"managed_mcp_servers,omitempty" json:"managed_mcp_servers,omitempty"`

	// Governance captures compile-time policy decisions for compliance reporting.
	Governance *GovernanceRecord `yaml:"governance,omitempty" json:"governance,omitempty"`

	// EffectiveState captures the effective configuration state at compile time.
	// This is the authoritative compliance record — no reconstruction needed.
	EffectiveState *EffectiveState `yaml:"effective_state,omitempty" json:"effective_state,omitempty"`

	// Signature is the cryptographic signature of this manifest.
	// Covers the JSON serialization with this field set to nil.
	Signature *ManifestSignature `yaml:"signature,omitempty" json:"signature,omitempty"`
}

// LayerRecord describes an input layer in the compilation manifest.
type LayerRecord struct {
	Path              string `yaml:"path"                          json:"path"`
	Source            string `yaml:"source,omitempty"              json:"source,omitempty"`
	Name              string `yaml:"name,omitempty"                json:"name,omitempty"`
	Type              string `yaml:"type"                          json:"type"`
	Format            string `yaml:"format"                        json:"format"`
	Version           string `yaml:"version,omitempty"             json:"version,omitempty"`
	Hash              string `yaml:"hash,omitempty"                json:"hash,omitempty"`
	LayerSignature    string `yaml:"layer_signature,omitempty"     json:"layer_signature,omitempty"`
	SignerKeyID       string `yaml:"signer_key_id,omitempty"       json:"signer_key_id,omitempty"`
	SignatureVerified bool   `yaml:"signature_verified,omitempty"  json:"signature_verified,omitempty"`
}

// FileRecord tracks a single output file in the compilation manifest.
type FileRecord struct {
	Output   string       `yaml:"output"           json:"output"`
	Sources  []FileSource `yaml:"sources,omitempty" json:"sources,omitempty"`
	Strategy string       `yaml:"strategy"          json:"strategy"`
	Hash     string       `yaml:"hash,omitempty"    json:"hash,omitempty"` // SHA-256 hash of written content (for user-editable files)
}

// FileSource records which layer contributed to an output file.
type FileSource struct {
	Layer string `yaml:"layer" json:"layer"`
	File  string `yaml:"file"  json:"file"`
}

// ForceLockRecord is a force-locked item in the compilation manifest.
type ForceLockRecord struct {
	Category string `yaml:"category"  json:"category"`
	Key      string `yaml:"key"       json:"key"`
	LockedBy string `yaml:"locked_by" json:"locked_by"`
}

// InstalledDepRecord tracks a resolved dependency in the compilation manifest.
type InstalledDepRecord struct {
	Name   string `json:"name"`
	Scope  string `json:"scope"`  // "project" or "global"
	Method string `json:"method"` // "install", "download", or "skipped"
	Path   string `json:"path"`
}

// GovernanceRecord captures compile-time policy decisions in the manifest.
type GovernanceRecord struct {
	PermissionsPolicy *PermissionsPolicyRecord `yaml:"permissions_policy,omitempty" json:"permissions_policy,omitempty"`
	MCPPolicy         *schema.MCPPolicy        `yaml:"mcp_policy,omitempty"         json:"mcp_policy,omitempty"`
	HookLockStrategy  string                   `yaml:"hook_lock_strategy,omitempty" json:"hook_lock_strategy,omitempty"`
	OverrideConflicts []OverrideConflict       `yaml:"override_conflicts,omitempty" json:"override_conflicts,omitempty"`
}

// PermissionsPolicyRecord tracks which permission merge policy was applied.
type PermissionsPolicyRecord struct {
	Merge string `yaml:"merge" json:"merge"`                       // "additive" or "replace"
	SetBy string `yaml:"set_by,omitempty" json:"set_by,omitempty"` // layer that set the policy
}

// OverrideConflict records when a layer attempted to modify a force-locked item.
type OverrideConflict struct {
	Category    string `yaml:"category"     json:"category"`
	Key         string `yaml:"key"          json:"key"`
	AttemptedBy string `yaml:"attempted_by" json:"attempted_by"`
	LockedBy    string `yaml:"locked_by"    json:"locked_by"`
}

// EffectiveState captures the effective configuration state at compile time.
// This is the authoritative compliance record — no reconstruction needed.
type EffectiveState struct {
	Permissions  *EffectivePermissions  `yaml:"permissions,omitempty"  json:"permissions,omitempty"`
	MCPServers   []EffectiveMCPServer   `yaml:"mcp_servers,omitempty"  json:"mcp_servers,omitempty"`
	Hooks        []EffectiveHook        `yaml:"hooks,omitempty"        json:"hooks,omitempty"`
	Settings     map[string]any         `yaml:"settings,omitempty"     json:"settings,omitempty"`
	Instructions *EffectiveInstructions `yaml:"instructions,omitempty" json:"instructions,omitempty"`
	Categories   []EffectiveCategory    `yaml:"categories,omitempty"   json:"categories,omitempty"`
}

type EffectivePermissions struct {
	Allow []string `yaml:"allow,omitempty" json:"allow,omitempty"`
	Deny  []string `yaml:"deny,omitempty"  json:"deny,omitempty"`
}

type EffectiveMCPServer struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"` // "stdio", "sse", "streamable-http"
}

type EffectiveHook struct {
	Event    string `yaml:"event"    json:"event"`
	Handlers int    `yaml:"handlers" json:"handlers"`
}

type EffectiveInstructions struct {
	TokenEstimate int `yaml:"token_estimate" json:"token_estimate"`
	Bytes         int `yaml:"bytes"          json:"bytes"`
}

type EffectiveCategory struct {
	Name      string `yaml:"name"       json:"name"`
	FileCount int    `yaml:"file_count" json:"file_count"`
}

// FindKeelState auto-discovers .keel.json in a directory.
// It checks:
//  1. <dir>/.keel.json directly
//  2. Known agent subdirs passed as agentDirs (e.g. [".claude", ".cursor"])
//
// Returns the loaded manifest(s) and their paths.
// If exactly one is found, returns it. If multiple are found, returns all.
// Returns nil, nil if none found.
func FindKeelState(dir string, agentDirs []string) ([]*CompiledManifest, []string, error) {
	// Check directly in dir
	if cm, err := LoadKeelState(dir); err != nil {
		return nil, nil, err
	} else if cm != nil {
		return []*CompiledManifest{cm}, []string{dir}, nil
	}

	// Scan known agent subdirs
	var manifests []*CompiledManifest
	var paths []string
	for _, agentDir := range agentDirs {
		fullPath := filepath.Join(dir, agentDir)
		cm, err := LoadKeelState(fullPath)
		if err != nil {
			return nil, nil, err
		}
		if cm != nil {
			manifests = append(manifests, cm)
			paths = append(paths, fullPath)
		}
	}

	return manifests, paths, nil
}

// LoadKeelState reads a compiled .keel.json from the given directory.
// The directory should be the agent config dir (e.g. outputDir/.claude).
// Returns nil, nil if no state file exists (first-time compile).
func LoadKeelState(dir string) (*CompiledManifest, error) {
	statePath := filepath.Join(dir, keelStateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading compiled state: %w", err)
	}
	var cm CompiledManifest
	if err := json.Unmarshal(data, &cm); err != nil {
		return nil, fmt.Errorf("parsing compiled state: %w", err)
	}
	return &cm, nil
}

// WriteKeelState writes a compiled manifest to .keel.json in the agent's config directory.
func WriteKeelState(path string, cm *CompiledManifest) error {
	data, err := json.MarshalIndent(cm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling .keel.json: %w", err)
	}

	// Append trailing newline for POSIX compatibility.
	data = append(data, '\n')

	return os.WriteFile(path, data, 0644)
}

// HashContent computes the SHA-256 hash of content, matching the format used
// by HashLayerDir. Returns a string like "sha256:abcdef...".
// Used to detect user edits to instruction files between compiles.
func HashContent(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// HashLayerDir computes a deterministic SHA-256 hash of a directory's content.
// It walks the directory recursively in lexical order, hashing relative file
// paths and their content. Symlinks and .git/ directories are skipped.
// Filesystem metadata (timestamps, permissions, ownership) is ignored.
func HashLayerDir(dirPath string) (string, error) {
	h := sha256.New()

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directories entirely.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Skip signature file — it's metadata about the layer, not content.
		// This ensures signing is idempotent: signing a layer doesn't change its hash.
		if !d.IsDir() && d.Name() == "keel.sig" {
			return nil
		}

		// Skip directories — we only hash files.
		if d.IsDir() {
			return nil
		}

		// Skip symlinks.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("computing relative path for %q: %w", path, err)
		}

		// Write relative path with null byte delimiters to prevent
		// path/content boundary confusion in the hash stream.
		// Use forward slashes for cross-platform determinism.
		normalizedPath := filepath.ToSlash(relPath)
		h.Write([]byte(normalizedPath))
		h.Write([]byte{0})

		content, err := pathutil.SafeReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %q: %w", path, err)
		}
		h.Write(content)
		h.Write([]byte{0})

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking directory %q: %w", dirPath, err)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// PruneResult reports what a cleanup operation did.
type PruneResult struct {
	Removed []string // relative paths of files that were deleted
	Missing []string // files in old manifest that were already gone
}

// PruneStaleOutput reads the existing .keel.json manifest from the agent dir,
// compares its file_ops against newFiles (the files the new compilation
// will write), and removes files that are no longer produced.
// Files not tracked by any manifest (user-created) are preserved.
// Returns nil, nil if no previous manifest exists.
func PruneStaleOutput(outputDir string, outputRoot string, newFiles []FileRecord, newOverlays map[string][]string) (*PruneResult, error) {
	agentDir := filepath.Join(outputDir, outputRoot)
	old, err := LoadKeelState(agentDir)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, nil
	}

	oldOwned := collectOwnedPaths(outputDir, old.FileOps, old.Overlays)
	newOwned := collectNewPaths(newFiles, newOverlays)

	// Handle merge-into files during prune: if the file existed in old manifest
	// as merge-into but is no longer produced, clean managed servers from it.
	// Also handle user-editable files: they should never be pruned even when not
	// in newOwned (skipped due to conflict detection).
	for _, f := range old.FileOps {
		if f.Strategy == "merge-into" && !newOwned[f.Output] {
			// Remove from oldOwned so the file itself isn't deleted.
			delete(oldOwned, f.Output)
			if len(old.ManagedMCPServers) > 0 {
				absPath := filepath.Join(outputDir, f.Output)
				if err := removeManagedMCPServers(absPath, old.ManagedMCPServers); err != nil {
					slog.Warn("failed to prune managed MCP servers", "path", f.Output, "error", err)
				}
			}
		} else if f.Strategy == "merge-into" {
			// Still produced — remove from oldOwned so the file isn't deleted
			// by the regular prune loop (it's managed via merge, not ownership).
			delete(oldOwned, f.Output)
		} else if f.Strategy == "user-editable" {
			// User-editable files should never be pruned, even if not in newOwned
			// (they may have been skipped due to conflict detection).
			delete(oldOwned, f.Output)
		}
	}

	result := &PruneResult{}
	for _, rel := range sortedKeys(oldOwned) {
		if isManifestFile(rel) {
			continue
		}
		if newOwned[rel] {
			continue
		}

		abs := filepath.Join(outputDir, rel)
		if !pathutil.ContainsPath(outputDir, abs) {
			slog.Warn("skipping stale file outside output directory", "path", rel)
			continue
		}
		if err := os.Remove(abs); err != nil {
			if os.IsNotExist(err) {
				result.Missing = append(result.Missing, rel)
				slog.Debug("stale file already removed", "path", rel)
				continue
			}
			slog.Warn("failed to remove stale file", "path", rel, "error", err)
			continue
		}
		result.Removed = append(result.Removed, rel)
		removeEmptyParents(abs, outputDir)
	}

	return result, nil
}

// CleanAllOutput reads the existing .keel.json manifest from the agent dir and
// removes every file listed in it, plus .keel.json itself.
// Files not in the manifest (user-created) are preserved.
// User-editable files (e.g. CLAUDE.md) are left in place unless force is true.
// Returns nil, nil if no manifest exists.
func CleanAllOutput(outputDir string, outputRoot string, force bool) (*PruneResult, error) {
	agentDir := filepath.Join(outputDir, outputRoot)
	old, err := LoadKeelState(agentDir)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, nil
	}

	owned := collectOwnedPaths(outputDir, old.FileOps, old.Overlays)

	// Handle merge-into files: remove only managed entries, don't delete the file.
	mergeIntoFiles := make(map[string]bool)
	for _, f := range old.FileOps {
		if f.Strategy == "merge-into" {
			mergeIntoFiles[f.Output] = true
			// Remove from owned set so the file itself isn't deleted.
			delete(owned, f.Output)
		}
	}

	// Handle user-editable files: leave in place unless force is set.
	if !force {
		for _, f := range old.FileOps {
			if f.Strategy == "user-editable" {
				delete(owned, f.Output)
				slog.Warn("preserved user-editable file (use --force to remove)", "path", f.Output)
			}
		}
	}

	// Clean managed MCP servers from merge-into files.
	if len(old.ManagedMCPServers) > 0 {
		for mergeFile := range mergeIntoFiles {
			absPath := filepath.Join(outputDir, mergeFile)
			if err := removeManagedMCPServers(absPath, old.ManagedMCPServers); err != nil {
				slog.Warn("failed to clean managed MCP servers", "path", mergeFile, "error", err)
			} else {
				slog.Debug("cleaned managed MCP servers from shared file",
					"path", mergeFile, "servers", old.ManagedMCPServers)
			}
		}
	}

	// Include .keel.json inside the agent dir
	keelStatePath := filepath.Join(outputRoot, keelStateFile)
	owned[keelStatePath] = true

	result := &PruneResult{}
	for _, rel := range sortedKeys(owned) {
		abs := filepath.Join(outputDir, rel)
		if !pathutil.ContainsPath(outputDir, abs) {
			slog.Warn("skipping stale file outside output directory", "path", rel)
			continue
		}
		if err := os.Remove(abs); err != nil {
			if os.IsNotExist(err) {
				result.Missing = append(result.Missing, rel)
				continue
			}
			slog.Warn("failed to remove file during clean", "path", rel, "error", err)
			continue
		}
		result.Removed = append(result.Removed, rel)
		removeEmptyParents(abs, outputDir)
	}

	return result, nil
}

// collectOwnedPaths builds the set of individual file paths that the
// manifest claims ownership of.
//
// For overlay-strategy records, it uses the tracked overlays map (which
// records exact file paths Keel wrote) rather than walking the filesystem.
// This prevents cleanup from claiming ownership of files in overlay
// directories that were placed there independently (not by Keel).
//
// The overlays map may be nil for older manifests — in that case, overlay
// directories are walked as a fallback.
func collectOwnedPaths(outputDir string, fileOps []FileRecord, overlays map[string][]string) map[string]bool {
	owned := make(map[string]bool)

	// If we have tracked overlay files, use those instead of walking.
	hasTrackedOverlays := len(overlays) > 0
	if hasTrackedOverlays {
		for path := range overlays {
			owned[path] = true
		}
	}

	for _, f := range fileOps {
		if f.Strategy == "overlay" {
			if hasTrackedOverlays {
				// Already handled above — skip the directory walk.
				continue
			}
			// Fallback: no tracked overlays, walk the directory.
			dir := filepath.Join(outputDir, f.Output)
			_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				rel, relErr := filepath.Rel(outputDir, path)
				if relErr != nil {
					return nil
				}
				owned[rel] = true
				return nil
			})
		} else {
			owned[f.Output] = true
		}
	}
	return owned
}

// collectNewPaths builds the set of file paths that the new compilation
// will produce. It uses FileRecord.Output for non-overlay files and the
// Overlays map keys for individual overlay file paths.
func collectNewPaths(newFiles []FileRecord, newOverlays map[string][]string) map[string]bool {
	paths := make(map[string]bool)
	for _, f := range newFiles {
		if f.Strategy != "overlay" {
			paths[f.Output] = true
		}
	}
	for p := range newOverlays {
		paths[p] = true
	}
	return paths
}

// removeEmptyParents walks upward from a deleted file's parent directory,
// removing empty directories until it reaches outputDir or a non-empty dir.
func removeEmptyParents(filePath, outputDir string) {
	dir := filepath.Dir(filePath)
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return // can't determine absolute path, stop removing
	}

	for {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return // can't determine absolute path, stop removing
		}
		if absDir == absOutput || !strings.HasPrefix(absDir, absOutput) {
			break
		}

		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		if err := os.Remove(dir); err != nil {
			break
		}
		slog.Debug("removed empty directory", "path", dir)
		dir = filepath.Dir(dir)
	}
}

func isManifestFile(rel string) bool {
	return rel == keelStateFile ||
		strings.HasSuffix(rel, "/"+keelStateFile)
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// removeManagedMCPServers reads a JSON file, removes the specified server
// names from the mcpServers key, and writes back. If mcpServers becomes
// empty, the key is removed. The file itself is never deleted (it contains
// other content like OAuth tokens).
func removeManagedMCPServers(path string, serverNames []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to clean
		}
		return err
	}

	var content map[string]any
	if err := json.Unmarshal(data, &content); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	servers, ok := content["mcpServers"].(map[string]any)
	if !ok || len(servers) == 0 {
		return nil // no servers to clean
	}

	for _, name := range serverNames {
		delete(servers, name)
	}

	if len(servers) == 0 {
		delete(content, "mcpServers")
	} else {
		content["mcpServers"] = servers
	}

	return writeJSONFile(path, content)
}

// writeJSONFile marshals data as indented JSON and writes it to path
// with a trailing newline. Uses atomic write (temp file + rename) to
// prevent corruption on crash.
func writeJSONFile(path string, data any) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling JSON for %s: %w", filepath.Base(path), err)
	}
	out = append(out, '\n')

	// Write to temp file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".keel-write-*")
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", path, err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }() // Clean up on error

	if _, err := tmpFile.Write(out); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing temp file for %s: %w", path, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file for %s: %w", path, err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file to %s: %w", path, err)
	}

	return nil
}
