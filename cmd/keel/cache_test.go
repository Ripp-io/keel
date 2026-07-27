package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
)

func TestNewCacheCmd(t *testing.T) {
	cmd := newCacheCmd()
	assert.Equal(t, "cache", cmd.Use)

	subCmds := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subCmds[sub.Use] = true
	}
	for _, name := range []string{"list", "clean"} {
		assert.True(t, subCmds[name], "expected subcommand %q", name)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"30d", 30 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"12h", 12 * time.Hour, false},
		{"45m", 45 * time.Minute, false},
		{"0d", 0, false},
		{"", 0, true},
		{"30", 0, true},
		{"30x", 0, true},
		{"abcd", 0, true},
		{"-5d", 0, true},
		{"d", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCacheList_Empty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, runCacheList(&buf, t.TempDir()))
	assert.Contains(t, buf.String(), "No cached remote layers.")
}

func TestCacheList_WithEntries(t *testing.T) {
	cacheDir := t.TempDir()

	gitHash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	gitDir := filepath.Join(cacheDir, "git", gitHash)
	require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "repo"), 0755))
	gitMeta := remote.GitMeta{URL: "https://git.example.com/org/policy.git", LastFetched: time.Now().Add(-2 * time.Hour)}
	metaData, err := json.MarshalIndent(gitMeta, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "meta.json"), metaData, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "repo", "data"), bytes.Repeat([]byte("x"), 1024), 0644))

	httpHash := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	httpDir := filepath.Join(cacheDir, "http", httpHash)
	require.NoError(t, os.MkdirAll(filepath.Join(httpDir, "content"), 0755))
	httpMeta := remote.HTTPMeta{URL: "https://example.com/layers-v2.tar.gz", LastFetched: time.Now().Add(-3 * 24 * time.Hour)}
	metaData, err = json.MarshalIndent(httpMeta, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(httpDir, "meta.json"), metaData, 0644))

	var buf bytes.Buffer
	require.NoError(t, runCacheList(&buf, cacheDir))
	output := buf.String()
	assert.Contains(t, output, "Remote layer cache")
	assert.Contains(t, output, "git")
	assert.Contains(t, output, "git.example.com/org/policy.git")
	assert.Contains(t, output, "http")
	assert.Contains(t, output, "example.com/layers-v2.tar.gz")
	assert.Contains(t, output, "2 entries")
}

func TestCacheClean(t *testing.T) {
	cacheDir := t.TempDir()
	gitDir := filepath.Join(cacheDir, "git", "somehash")
	require.NoError(t, os.MkdirAll(gitDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "data"), []byte("content"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "http", "anotherhash"), 0755))

	var buf bytes.Buffer
	require.NoError(t, runCacheClean(&buf, cacheDir))
	assert.Contains(t, buf.String(), "Removed")

	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCacheClean_Empty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, runCacheClean(&buf, t.TempDir()))
	assert.Contains(t, buf.String(), "No cached remote layers.")
}

func TestCacheClean_NonexistentDir(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, runCacheClean(&buf, "/nonexistent/path/cache"))
	assert.Contains(t, buf.String(), "No cached remote layers.")
}

func TestCacheClean_WithNestedEntries(t *testing.T) {
	cacheDir := t.TempDir()
	gitDir := filepath.Join(cacheDir, "git", "hash1")
	require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "repo"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "meta.json"), []byte("{}"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "git", "hash2", "repo"), 0755))
	httpDir := filepath.Join(cacheDir, "http", "hash3")
	require.NoError(t, os.MkdirAll(filepath.Join(httpDir, "content"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(httpDir, "content", "data.txt"), bytes.Repeat([]byte("x"), 2048), 0644))

	var buf bytes.Buffer
	require.NoError(t, runCacheClean(&buf, cacheDir))
	output := buf.String()
	assert.Contains(t, output, "Removed")
	assert.Contains(t, output, "3")
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCollectCacheEntries_EmptyDir(t *testing.T) {
	entries, err := collectCacheEntries(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCollectCacheEntries_SkipsInvalidMeta(t *testing.T) {
	cacheDir := t.TempDir()
	gitDir := filepath.Join(cacheDir, "git", "invalidhash")
	require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "repo"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "meta.json"), []byte("{invalid json"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "http", "nometa", "content"), 0755))

	entries, err := collectCacheEntries(cacheDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCollectCacheEntries_SkipsFiles(t *testing.T) {
	cacheDir := t.TempDir()
	gitTypeDir := filepath.Join(cacheDir, "git")
	require.NoError(t, os.MkdirAll(gitTypeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitTypeDir, "notadir"), []byte("file"), 0644))

	entries, err := collectCacheEntries(cacheDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), bytes.Repeat([]byte("x"), 100), 0644))
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(sub, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "b"), bytes.Repeat([]byte("y"), 200), 0644))

	size, err := dirSize(dir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, size, int64(300))
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"}, {512, "512 B"}, {1024, "1.0 KB"},
		{1536, "1.5 KB"}, {1048576, "1.0 MB"}, {1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, formatSize(tt.bytes))
		})
	}
}

func TestRunCacheCleanStale_EmptyDir(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, runCacheCleanStale(&buf, t.TempDir(), 24*time.Hour))
	assert.Contains(t, buf.String(), "No entries older")
}

func TestRunCacheCleanStale_RemovesOldEntries(t *testing.T) {
	cacheDir := t.TempDir()

	oldDir := filepath.Join(cacheDir, "git", "oldhash_for_clean_stale_test")
	require.NoError(t, os.MkdirAll(filepath.Join(oldDir, "repo"), 0755))
	oldMeta := remote.GitMeta{URL: "https://example.com/old-repo.git", LastFetched: time.Now().Add(-48 * time.Hour).UTC()}
	metaData, err := json.MarshalIndent(oldMeta, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "meta.json"), metaData, 0644))

	newDir := filepath.Join(cacheDir, "git", "newhash_for_clean_stale_test")
	require.NoError(t, os.MkdirAll(filepath.Join(newDir, "repo"), 0755))
	newMeta := remote.GitMeta{URL: "https://example.com/new-repo.git", LastFetched: time.Now().Add(-1 * time.Hour).UTC()}
	metaData, err = json.MarshalIndent(newMeta, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(newDir, "meta.json"), metaData, 0644))

	oldHTTPDir := filepath.Join(cacheDir, "http", "oldhttphash_for_clean_stale")
	require.NoError(t, os.MkdirAll(filepath.Join(oldHTTPDir, "content"), 0755))
	oldHTTPMeta := remote.HTTPMeta{URL: "https://example.com/old-archive.tar.gz", LastFetched: time.Now().Add(-72 * time.Hour).UTC()}
	metaData, err = json.MarshalIndent(oldHTTPMeta, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(oldHTTPDir, "meta.json"), metaData, 0644))

	var buf bytes.Buffer
	require.NoError(t, runCacheCleanStale(&buf, cacheDir, 24*time.Hour))
	assert.Contains(t, buf.String(), "Removed 2 entries")
	assert.NoDirExists(t, oldDir)
	assert.DirExists(t, newDir)
	assert.NoDirExists(t, oldHTTPDir)
}

func TestRunCacheCleanStale_AllRecent(t *testing.T) {
	cacheDir := t.TempDir()
	dir := filepath.Join(cacheDir, "git", "recent_hash")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "repo"), 0755))
	meta := remote.GitMeta{URL: "https://example.com/recent.git", LastFetched: time.Now().UTC()}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.json"), metaData, 0644))

	var buf bytes.Buffer
	require.NoError(t, runCacheCleanStale(&buf, cacheDir, 24*time.Hour))
	assert.Contains(t, buf.String(), "No entries older")
	assert.DirExists(t, dir)
}

func TestCacheCleanCmd_InvalidOlderThan(t *testing.T) {
	cmd := newCacheCleanCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		olderThan, _ := cmd.Flags().GetString("older-than")
		if olderThan != "" {
			if _, err := parseDuration(olderThan); err != nil {
				return fmt.Errorf("invalid --older-than value: %w", err)
			}
		}
		return nil
	}
	cmd.SetArgs([]string{"--older-than", "invalid"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestFormatAge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{"seconds ago", now.Add(-30 * time.Second), "just now"},
		{"minutes ago", now.Add(-5 * time.Minute), "5m ago"},
		{"hours ago", now.Add(-3 * time.Hour), "3h ago"},
		{"days ago", now.Add(-7 * 24 * time.Hour), "7d ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatAge(tt.when))
		})
	}
}
