package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GitMeta holds metadata for a cached Git mirror.
type GitMeta struct {
	URL             string    `json:"url"`
	LastFetched     time.Time `json:"last_fetched"`
	MirrorSizeBytes int64     `json:"mirror_size_bytes,omitempty"`
	ContentHash     string    `json:"content_hash,omitempty"` // SHA-256 hash of content directory
}

// HTTPMeta holds metadata for a cached HTTP archive.
type HTTPMeta struct {
	URL              string    `json:"url"`
	LastFetched      time.Time `json:"last_fetched"`
	ETag             string    `json:"etag,omitempty"`
	LastModified     string    `json:"last_modified,omitempty"`
	ContentSizeBytes int64     `json:"content_size_bytes,omitempty"`
	ArchiveFormat    string    `json:"archive_format,omitempty"`
	ContentHash      string    `json:"content_hash,omitempty"` // SHA-256 hash of content directory
}

// CacheDir returns the default cache base directory.
// It uses $XDG_CACHE_HOME/keel/remote/ if XDG_CACHE_HOME is set,
// otherwise ~/.cache/keel/remote/.
func CacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fall back to a reasonable default if home dir is unavailable.
			home = "."
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "keel", "remote")
}

// CacheKey returns a deterministic, filesystem-safe directory name for a URL.
// The key is the hex-encoded SHA-256 of the normalized URL.
// Normalization: lowercase scheme and host, strip trailing slash, strip fragment.
// The path component is preserved as-is (case-sensitive).
func CacheKey(rawURL string) string {
	normalized := normalizeURL(rawURL)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// normalizeURL lowercases scheme and host, strips trailing slashes and fragments.
func normalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		// If URL parsing fails, hash the raw string as-is.
		return rawURL
	}

	// Lowercase scheme.
	parsed.Scheme = strings.ToLower(parsed.Scheme)

	// Lowercase host (includes port if present).
	parsed.Host = strings.ToLower(parsed.Host)

	// Strip fragment.
	parsed.Fragment = ""
	parsed.RawFragment = ""

	// Strip trailing slash from path, unless path is just "/".
	if len(parsed.Path) > 1 {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}

	return parsed.String()
}

// ReadMeta reads a JSON file at the given path and unmarshals it into T.
// T is typically GitMeta or HTTPMeta.
func ReadMeta[T any](path string) (T, error) {
	var meta T
	data, err := os.ReadFile(path)
	if err != nil {
		return meta, fmt.Errorf("reading meta %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, fmt.Errorf("parsing meta %s: %w", path, err)
	}
	return meta, nil
}

// ReadGitMeta reads a GitMeta from a JSON file at the given path.
func ReadGitMeta(path string) (GitMeta, error) {
	return ReadMeta[GitMeta](path)
}

// ReadHTTPMeta reads an HTTPMeta from a JSON file at the given path.
func ReadHTTPMeta(path string) (HTTPMeta, error) {
	return ReadMeta[HTTPMeta](path)
}

// WriteMeta writes a meta value (GitMeta or HTTPMeta) as JSON to the given path.
// Parent directories are created if they do not exist.
// The file is written with 0644 permissions.
func WriteMeta(path string, meta any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating meta directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling meta: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing meta %s: %w", path, err)
	}
	return nil
}

// EnsureCacheDir creates the cache directory structure for a given source type
// and hash. It returns the path to the hash-level directory.
//
// For subtype "git": creates <cacheBase>/git/<hash>/ with repo/ and worktrees/ subdirs.
// For subtype "http": creates <cacheBase>/http/<hash>/ with content/ subdir.
//
// Directories are created with 0700 permissions. The function is idempotent.
func EnsureCacheDir(cacheBase, subtype, hash string) (string, error) {
	var subdirs []string
	switch subtype {
	case "git":
		subdirs = []string{"repo", "worktrees"}
	case "http":
		subdirs = []string{"content"}
	case "files":
		subdirs = []string{"content"}
	default:
		return "", fmt.Errorf("unknown cache subtype %q: expected \"git\", \"http\", or \"files\"", subtype)
	}

	hashDir := filepath.Join(cacheBase, subtype, hash)

	// Create the hash-level directory with restricted permissions.
	if err := os.MkdirAll(hashDir, 0700); err != nil {
		return "", fmt.Errorf("creating cache directory %s: %w", hashDir, err)
	}

	// Ensure the hash-level directory itself has 0700 permissions,
	// even if MkdirAll inherited parent permissions.
	if err := os.Chmod(hashDir, 0700); err != nil {
		return "", fmt.Errorf("setting permissions on %s: %w", hashDir, err)
	}

	for _, sub := range subdirs {
		p := filepath.Join(hashDir, sub)
		if err := os.MkdirAll(p, 0700); err != nil {
			return "", fmt.Errorf("creating subdirectory %s: %w", p, err)
		}
	}

	return hashDir, nil
}

// Clean removes all contents under the given cache directory.
// The directory itself is preserved (emptied, not deleted).
func Clean(cacheDir string) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return fmt.Errorf("reading cache directory %s: %w", cacheDir, err)
	}

	for _, entry := range entries {
		p := filepath.Join(cacheDir, entry.Name())
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("removing %s: %w", p, err)
		}
	}
	return nil
}

// CleanStale removes cache entries whose meta.json LastFetched is older than
// maxAge. Entries without a meta.json file are left untouched.
//
// It walks the git/ and http/ subdirectories under cacheDir, reading meta.json
// from each hash-level directory.
func CleanStale(cacheDir string, maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)

	for _, subtype := range []string{"git", "http", "files"} {
		subtypeDir := filepath.Join(cacheDir, subtype)
		entries, err := os.ReadDir(subtypeDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("reading %s: %w", subtypeDir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			hashDir := filepath.Join(subtypeDir, entry.Name())
			metaPath := filepath.Join(hashDir, "meta.json")

			lastFetched, err := readLastFetched(metaPath)
			if err != nil {
				// No meta.json or unparseable -- skip this entry.
				continue
			}

			if lastFetched.Before(cutoff) {
				if err := os.RemoveAll(hashDir); err != nil {
					return fmt.Errorf("removing stale entry %s: %w", hashDir, err)
				}
			}
		}
	}

	return nil
}

// readLastFetched extracts the last_fetched timestamp from a meta.json file.
// It works for both GitMeta and HTTPMeta since they share the field name.
func readLastFetched(path string) (time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}

	var m struct {
		LastFetched time.Time `json:"last_fetched"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return time.Time{}, err
	}
	return m.LastFetched, nil
}
