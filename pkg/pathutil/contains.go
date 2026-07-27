// Package pathutil provides filesystem path validation utilities.
package pathutil

import (
	"path/filepath"
	"strings"
)

// ContainsPath reports whether target is inside the base directory.
// It resolves symlinks where possible and handles non-existent paths
// by walking ancestor directories.
//
// Falls back to filepath.Abs if symlink resolution fails (e.g., path does not exist).
// When a path does not exist, the longest existing ancestor is resolved via
// EvalSymlinks and the remaining components are re-joined, so platforms where
// the temp-dir root is itself a symlink (e.g. macOS /var -> /private/var) are
// handled correctly.
func ContainsPath(base, target string) bool {
	absBase := resolvePathBestEffort(base)
	if absBase == "" {
		return false
	}
	absTarget := resolvePathBestEffort(target)
	if absTarget == "" {
		return false
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, "../")
}

// resolvePathBestEffort resolves a path to an absolute, symlink-resolved form.
// It always converts to absolute first to avoid mixed relative/absolute
// comparisons when ContainsPath compares an existing base with a non-existent
// target. If the path does not exist, it walks up to the nearest existing
// ancestor, resolves that, and re-appends the remaining components. This
// ensures consistent resolution even when the path's prefix contains symlinks
// (e.g. /var -> /private/var on macOS).
func resolvePathBestEffort(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved
	}
	// Collect non-existent tail components, then resolve the existing prefix.
	var tail []string
	cur := abs
	for {
		resolved, err = filepath.EvalSymlinks(cur)
		if err == nil {
			// Re-join the resolved ancestor with the unresolved tail
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached filesystem root without success -- fall back to Abs
			return abs
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}
