package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CacheDir
// ---------------------------------------------------------------------------

func TestCacheDir_Default(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".cache", "keel", "remote"), CacheDir())
}

func TestCacheDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/custom-cache")
	assert.Equal(t, filepath.Join("/tmp/custom-cache", "keel", "remote"), CacheDir())
}

// ---------------------------------------------------------------------------
// Meta round-trip
// ---------------------------------------------------------------------------

func TestMetaRoundTrip_Git(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.json")

	now := time.Now().Truncate(time.Second).UTC()
	original := GitMeta{
		URL:             "https://git.example.com/org/policy.git",
		LastFetched:     now,
		MirrorSizeBytes: 1048576,
	}

	require.NoError(t, WriteMeta(metaPath, original))

	got, err := ReadGitMeta(metaPath)
	require.NoError(t, err)

	assert.Equal(t, original.URL, got.URL)
	assert.True(t, got.LastFetched.Equal(original.LastFetched), "LastFetched = %v, want %v", got.LastFetched, original.LastFetched)
	assert.Equal(t, original.MirrorSizeBytes, got.MirrorSizeBytes)
}

func TestMetaRoundTrip_HTTP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.json")

	now := time.Now().Truncate(time.Second).UTC()
	original := HTTPMeta{
		URL:              "https://example.com/layers.tar.gz",
		LastFetched:      now,
		ETag:             `"abc123"`,
		LastModified:     "Wed, 06 May 2026 10:00:00 GMT",
		ContentSizeBytes: 524288,
		ArchiveFormat:    "tar.gz",
	}

	require.NoError(t, WriteMeta(metaPath, original))

	got, err := ReadHTTPMeta(metaPath)
	require.NoError(t, err)

	assert.Equal(t, original.URL, got.URL)
	assert.True(t, got.LastFetched.Equal(original.LastFetched), "LastFetched = %v, want %v", got.LastFetched, original.LastFetched)
	assert.Equal(t, original.ETag, got.ETag)
	assert.Equal(t, original.LastModified, got.LastModified)
	assert.Equal(t, original.ContentSizeBytes, got.ContentSizeBytes)
	assert.Equal(t, original.ArchiveFormat, got.ArchiveFormat)
}

func TestWriteMeta_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "nested", "deep", "meta.json")

	meta := GitMeta{
		URL:         "https://example.com/repo.git",
		LastFetched: time.Now().UTC(),
	}
	require.NoError(t, WriteMeta(metaPath, meta))

	_, err := os.Stat(metaPath)
	assert.NoError(t, err, "meta.json should have been created")
}

func TestWriteMeta_FilePermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("file permissions test not applicable on Windows")
	}
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.json")

	meta := GitMeta{URL: "https://example.com/repo.git", LastFetched: time.Now().UTC()}
	require.NoError(t, WriteMeta(metaPath, meta))

	info, err := os.Stat(metaPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestReadGitMeta_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := ReadGitMeta("/nonexistent/path/meta.json")
	assert.Error(t, err, "expected error for missing file")
}

func TestReadHTTPMeta_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := ReadHTTPMeta("/nonexistent/path/meta.json")
	assert.Error(t, err, "expected error for missing file")
}

func TestReadGitMeta_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.json")
	require.NoError(t, os.WriteFile(metaPath, []byte("{invalid json"), 0644))

	_, err := ReadGitMeta(metaPath)
	assert.Error(t, err, "expected error for malformed JSON")
}

func TestWriteMeta_JSONContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.json")

	meta := HTTPMeta{
		URL:         "https://example.com/archive.tar.gz",
		LastFetched: time.Date(2026, 5, 7, 14, 30, 0, 0, time.UTC),
		ETag:        `"xyz"`,
	}
	require.NoError(t, WriteMeta(metaPath, meta))

	raw, err := os.ReadFile(metaPath)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &m), "written file should be valid JSON")
	assert.Equal(t, "https://example.com/archive.tar.gz", m["url"])
	assert.Equal(t, `"xyz"`, m["etag"])
}

// ---------------------------------------------------------------------------
// EnsureCacheDir
// ---------------------------------------------------------------------------

func TestEnsureCacheDir_Git(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	hash := "abcdef1234567890"

	dir, err := EnsureCacheDir(base, "git", hash)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(base, "git", hash), dir)

	// Verify subdirectories were created.
	for _, sub := range []string{"repo", "worktrees"} {
		p := filepath.Join(dir, sub)
		info, err := os.Stat(p)
		require.NoError(t, err, "expected %q to exist", sub)
		assert.True(t, info.IsDir(), "expected %q to be a directory", sub)
	}
}

func TestEnsureCacheDir_HTTP(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	hash := "abcdef1234567890"

	dir, err := EnsureCacheDir(base, "http", hash)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(base, "http", hash), dir)

	info, err := os.Stat(filepath.Join(dir, "content"))
	require.NoError(t, err, "expected content/ to exist")
	assert.True(t, info.IsDir(), "expected content/ to be a directory")
}

func TestEnsureCacheDir_Idempotent(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	hash := "idempotent123"

	dir1, err := EnsureCacheDir(base, "git", hash)
	require.NoError(t, err)

	// Place a marker file to verify it survives the second call.
	marker := filepath.Join(dir1, "repo", "marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("present"), 0644))

	dir2, err := EnsureCacheDir(base, "git", hash)
	require.NoError(t, err)

	assert.Equal(t, dir1, dir2, "idempotent calls should return same path")

	_, err = os.Stat(marker)
	assert.NoError(t, err, "marker file should survive second call")
}

func TestEnsureCacheDir_Permissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions test not applicable on Windows")
	}
	base := t.TempDir()

	dir, err := EnsureCacheDir(base, "git", "permtest")
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), info.Mode().Perm())
}

func TestEnsureCacheDir_UnknownSubtype(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	_, err := EnsureCacheDir(base, "ftp", "hash123")
	assert.Error(t, err, "expected error for unknown subtype")
}

// ---------------------------------------------------------------------------
// Clean
// ---------------------------------------------------------------------------

func TestClean(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	// Create some cache structure.
	gitDir := filepath.Join(base, "git", "abc123")
	require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "repo"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "meta.json"), []byte("{}"), 0644))

	httpDir := filepath.Join(base, "http", "def456")
	require.NoError(t, os.MkdirAll(filepath.Join(httpDir, "content"), 0700))

	require.NoError(t, Clean(base))

	entries, err := os.ReadDir(base)
	require.NoError(t, err)
	assert.Empty(t, entries, "directory should be empty after Clean")
}

func TestClean_NonexistentDir(t *testing.T) {
	t.Parallel()
	err := Clean("/nonexistent/path/that/does/not/exist")
	assert.Error(t, err, "expected error for nonexistent directory")
}

// ---------------------------------------------------------------------------
// CleanStale
// ---------------------------------------------------------------------------

func TestCleanStale(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	// Create an "old" git entry.
	oldDir := filepath.Join(base, "git", "oldhash123")
	require.NoError(t, os.MkdirAll(filepath.Join(oldDir, "repo"), 0700))
	oldMeta := GitMeta{
		URL:         "https://example.com/old-repo.git",
		LastFetched: time.Now().Add(-48 * time.Hour).UTC(),
	}
	require.NoError(t, WriteMeta(filepath.Join(oldDir, "meta.json"), oldMeta))

	// Create a "new" git entry.
	newDir := filepath.Join(base, "git", "newhash456")
	require.NoError(t, os.MkdirAll(filepath.Join(newDir, "repo"), 0700))
	newMeta := GitMeta{
		URL:         "https://example.com/new-repo.git",
		LastFetched: time.Now().UTC(),
	}
	require.NoError(t, WriteMeta(filepath.Join(newDir, "meta.json"), newMeta))

	// Create an "old" http entry.
	oldHTTPDir := filepath.Join(base, "http", "oldhttpABC")
	require.NoError(t, os.MkdirAll(filepath.Join(oldHTTPDir, "content"), 0700))
	oldHTTPMeta := HTTPMeta{
		URL:         "https://example.com/old-archive.tar.gz",
		LastFetched: time.Now().Add(-72 * time.Hour).UTC(),
	}
	require.NoError(t, WriteMeta(filepath.Join(oldHTTPDir, "meta.json"), oldHTTPMeta))

	// Clean entries older than 24 hours.
	require.NoError(t, CleanStale(base, 24*time.Hour))

	assert.NoDirExists(t, oldDir, "old git entry should be removed")
	assert.DirExists(t, newDir, "new git entry should survive")
	assert.NoDirExists(t, oldHTTPDir, "old http entry should be removed")
}

func TestCleanStale_NoMetaJSON(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	orphanDir := filepath.Join(base, "git", "orphan123")
	require.NoError(t, os.MkdirAll(orphanDir, 0700))

	require.NoError(t, CleanStale(base, 24*time.Hour))

	assert.DirExists(t, orphanDir, "orphan dir without meta.json should survive")
}

// ---------------------------------------------------------------------------
// AcquireLock / ReleaseLock
// ---------------------------------------------------------------------------

func TestAcquireLock_Basic(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("flock not supported on Windows")
	}

	dir := t.TempDir()

	f, err := AcquireLock(dir)
	require.NoError(t, err)
	require.NotNil(t, f, "AcquireLock() returned nil file")

	lockPath := filepath.Join(dir, ".lock")
	assert.FileExists(t, lockPath)

	ReleaseLock(f)

	_, err = f.Write([]byte("test"))
	assert.Error(t, err, "write to closed file should fail")
}

func TestAcquireLock_ReleaseThenReacquire(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("flock not supported on Windows")
	}

	dir := t.TempDir()

	f1, err := AcquireLock(dir)
	require.NoError(t, err)
	ReleaseLock(f1)

	f2, err := AcquireLock(dir)
	require.NoError(t, err)
	ReleaseLock(f2)
}

func TestReleaseLock_NilSafe(t *testing.T) {
	t.Parallel()
	// Should not panic.
	ReleaseLock(nil)
}

// ---------------------------------------------------------------------------
// CacheKey edge cases
// ---------------------------------------------------------------------------

func TestCacheKey_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		urlA      string
		urlB      string
		wantEqual bool
	}{
		{
			name:      "identical URLs",
			urlA:      "https://example.com/repo.git",
			urlB:      "https://example.com/repo.git",
			wantEqual: true,
		},
		{
			name:      "trailing slash normalization",
			urlA:      "https://example.com/repo.git/",
			urlB:      "https://example.com/repo.git",
			wantEqual: true,
		},
		{
			name:      "scheme case normalization",
			urlA:      "HTTPS://example.com/repo.git",
			urlB:      "https://example.com/repo.git",
			wantEqual: true,
		},
		{
			name:      "host case normalization",
			urlA:      "https://EXAMPLE.COM/repo.git",
			urlB:      "https://example.com/repo.git",
			wantEqual: true,
		},
		{
			name:      "fragment stripped",
			urlA:      "https://example.com/repo.git#main",
			urlB:      "https://example.com/repo.git",
			wantEqual: true,
		},
		{
			name:      "different paths produce different keys",
			urlA:      "https://example.com/repo-a.git",
			urlB:      "https://example.com/repo-b.git",
			wantEqual: false,
		},
		{
			name:      "path is case-sensitive",
			urlA:      "https://example.com/Repo.git",
			urlB:      "https://example.com/repo.git",
			wantEqual: false,
		},
		{
			name:      "different hosts produce different keys",
			urlA:      "https://alpha.com/repo.git",
			urlB:      "https://beta.com/repo.git",
			wantEqual: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			keyA := CacheKey(tt.urlA)
			keyB := CacheKey(tt.urlB)
			if tt.wantEqual {
				assert.Equal(t, keyA, keyB, "expected equal keys for %q and %q", tt.urlA, tt.urlB)
			} else {
				assert.NotEqual(t, keyA, keyB, "expected different keys for %q and %q", tt.urlA, tt.urlB)
			}
		})
	}
}

func TestCacheKey_NonURLInput(t *testing.T) {
	t.Parallel()
	key := CacheKey("just-a-local-path")
	assert.Len(t, key, 64, "should produce a 64-char hex hash")
	assert.Regexp(t, `^[0-9a-f]{64}$`, key)
}
