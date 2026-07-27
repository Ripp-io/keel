package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage remote layer cache",
		Long: `Manage the local cache of remote layers fetched via --layer.

Remote layers (Git repos, HTTPS archives) are cached locally under
~/.cache/keel/remote/ (or $XDG_CACHE_HOME/keel/remote/) for fast
reuse across compilations.

Use the subcommands to inspect and clean the cache.`,
	}

	cmd.AddCommand(newCacheListCmd())
	cmd.AddCommand(newCacheCleanCmd())

	return cmd
}

func newCacheListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cached remote layers",
		Long:  "List all cached remote layers with type, URL, age, and size on disk.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCacheList(cmd.OutOrStdout(), remote.CacheDir())
		},
	}
}

func newCacheCleanCmd() *cobra.Command {
	var olderThan string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove cached remote layers",
		Long: `Remove all cached remote layer data.

Use --older-than to remove only entries older than a given duration.
Supported suffixes: d (days), h (hours), m (minutes).

Examples:
  keel cache clean                   # remove all cached layers
  keel cache clean --older-than 30d  # remove entries older than 30 days
  keel cache clean --older-than 12h  # remove entries older than 12 hours`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cacheDir := remote.CacheDir()

			if olderThan != "" {
				dur, err := parseDuration(olderThan)
				if err != nil {
					return fmt.Errorf("invalid --older-than value: %w", err)
				}
				return runCacheCleanStale(cmd.OutOrStdout(), cacheDir, dur)
			}

			return runCacheClean(cmd.OutOrStdout(), cacheDir)
		},
	}

	cmd.Flags().StringVar(&olderThan, "older-than", "", "Remove entries older than duration (e.g., 30d, 12h, 45m)")

	return cmd
}

// cacheEntry represents a single cached remote layer for display.
type cacheEntry struct {
	Type string
	URL  string
	Age  time.Time
	Size int64
}

// runCacheList lists all cached remote layers with type, URL, age, and size.
func runCacheList(w io.Writer, cacheDir string) error {
	entries, err := collectCacheEntries(cacheDir)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "No cached remote layers.")
		return nil
	}

	fmt.Fprintf(w, "Remote layer cache (%s/):\n\n", cacheDir)

	// Print header.
	fmt.Fprintf(w, "  %-6s %-52s %-9s %s\n", "TYPE", "URL", "AGE", "SIZE")

	var totalSize int64
	for _, e := range entries {
		totalSize += e.Size
		fmt.Fprintf(w, "  %-6s %-52s %-9s %s\n",
			e.Type, e.URL, formatAge(e.Age), formatSize(e.Size))
	}

	fmt.Fprintf(w, "\n  %d entries, %s total\n", len(entries), formatSize(totalSize))

	return nil
}

// collectCacheEntries reads all git/ and http/ subdirectories under cacheDir
// and builds a list of cache entries from their meta.json files.
func collectCacheEntries(cacheDir string) ([]cacheEntry, error) {
	var entries []cacheEntry

	for _, subtype := range []string{"git", "http"} {
		subtypeDir := filepath.Join(cacheDir, subtype)
		hashDirs, err := os.ReadDir(subtypeDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", subtypeDir, err)
		}

		for _, d := range hashDirs {
			if !d.IsDir() {
				continue
			}

			hashDir := filepath.Join(subtypeDir, d.Name())
			metaPath := filepath.Join(hashDir, "meta.json")

			var entryURL string
			var lastFetched time.Time

			switch subtype {
			case "git":
				meta, err := remote.ReadGitMeta(metaPath)
				if err != nil {
					continue // skip entries without valid meta
				}
				entryURL = meta.URL
				lastFetched = meta.LastFetched
			case "http":
				meta, err := remote.ReadHTTPMeta(metaPath)
				if err != nil {
					continue
				}
				entryURL = meta.URL
				lastFetched = meta.LastFetched
			}

			size, _ := dirSize(hashDir)

			entries = append(entries, cacheEntry{
				Type: subtype,
				URL:  entryURL,
				Age:  lastFetched,
				Size: size,
			})
		}
	}

	return entries, nil
}

// runCacheClean removes all cache contents and reports the result.
func runCacheClean(w io.Writer, cacheDir string) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(w, "No cached remote layers.")
			return nil
		}
		return fmt.Errorf("reading cache directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "No cached remote layers.")
		return nil
	}

	// Calculate total size before cleaning.
	totalSize, _ := dirSize(cacheDir)

	// Count entries across subdirectories.
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subDir := filepath.Join(cacheDir, e.Name())
		subs, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, s := range subs {
			if s.IsDir() {
				count++
			}
		}
	}

	if err := remote.Clean(cacheDir); err != nil {
		return fmt.Errorf("cleaning cache: %w", err)
	}

	fmt.Fprintf(w, "Removed %d entries, freed %s.\n", count, formatSize(totalSize))
	return nil
}

// runCacheCleanStale removes entries older than maxAge and reports the result.
func runCacheCleanStale(w io.Writer, cacheDir string, maxAge time.Duration) error {
	// Count entries before cleaning.
	beforeEntries, _ := collectCacheEntries(cacheDir)

	if err := remote.CleanStale(cacheDir, maxAge); err != nil {
		return fmt.Errorf("cleaning stale entries: %w", err)
	}

	afterEntries, _ := collectCacheEntries(cacheDir)
	removed := len(beforeEntries) - len(afterEntries)

	if removed == 0 {
		fmt.Fprintln(w, "No entries older than the specified duration.")
	} else {
		fmt.Fprintf(w, "Removed %d entries.\n", removed)
	}

	return nil
}

// parseDuration parses a duration string with d (days), h (hours), or m (minutes)
// suffix. Examples: "30d", "12h", "45m".
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q: must be a number followed by d, h, or m", s)
	}

	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}

	if n < 0 {
		return 0, fmt.Errorf("invalid duration %q: value must not be negative", s)
	}

	switch suffix {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("invalid duration %q: unknown suffix %q (use d, h, or m)", s, string(suffix))
	}
}

// dirSize returns the total size in bytes of all files under a directory.
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}

// formatSize formats a byte count as a human-readable string.
func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}

	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(bytes)
	for _, unit := range units {
		size /= 1024
		if size < 1024 || unit == "TB" {
			return fmt.Sprintf("%.1f %s", size, unit)
		}
	}

	return fmt.Sprintf("%d B", bytes)
}

// formatAge formats a timestamp as a human-readable relative age string.
func formatAge(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

