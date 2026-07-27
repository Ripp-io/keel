package remote

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupBareRepo creates a normal Git repo with commits and files, then creates
// a bare clone of it to simulate a remote server. Returns the bare repo path
// as a file:// URL. This avoids any network access in tests.
func setupBareRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	bareDir := filepath.Join(dir, "bare.git")

	// Initialize a normal repo.
	runTestGit(t, srcDir, "init")
	runTestGit(t, srcDir, "config", "user.email", "test@test.com")
	runTestGit(t, srcDir, "config", "user.name", "Test User")

	// Create files and commit.
	for path, content := range files {
		full := filepath.Join(srcDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755), "creating dir for %s", path)
		require.NoError(t, os.WriteFile(full, []byte(content), 0644), "writing %s", path)
	}
	runTestGit(t, srcDir, "add", ".")
	runTestGit(t, srcDir, "commit", "-m", "initial commit")

	// Create a bare clone.
	cmd := exec.Command("git", "clone", "--bare", srcDir, bareDir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git clone --bare failed:\n%s", out)

	return bareDir
}

// setupBareRepoWithBranch creates a bare repo with a specific branch containing
// different content from the default branch.
func setupBareRepoWithBranch(t *testing.T, defaultFiles map[string]string, branchName string, branchFiles map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	bareDir := filepath.Join(dir, "bare.git")

	// Initialize a normal repo.
	runTestGit(t, srcDir, "init")
	runTestGit(t, srcDir, "config", "user.email", "test@test.com")
	runTestGit(t, srcDir, "config", "user.name", "Test User")

	// Create default branch files and commit.
	for path, content := range defaultFiles {
		full := filepath.Join(srcDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755), "creating dir for %s", path)
		require.NoError(t, os.WriteFile(full, []byte(content), 0644), "writing %s", path)
	}
	runTestGit(t, srcDir, "add", ".")
	runTestGit(t, srcDir, "commit", "-m", "initial commit on default branch")

	// Create a branch with different content.
	runTestGit(t, srcDir, "checkout", "-b", branchName)
	for path, content := range branchFiles {
		full := filepath.Join(srcDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755), "creating dir for %s", path)
		require.NoError(t, os.WriteFile(full, []byte(content), 0644), "writing %s", path)
	}
	runTestGit(t, srcDir, "add", ".")
	runTestGit(t, srcDir, "commit", "-m", "commit on "+branchName)

	// Switch back to default branch so bare clone has the right HEAD.
	runTestGit(t, srcDir, "checkout", "-")

	// Create a bare clone.
	cmd := exec.Command("git", "clone", "--bare", srcDir, bareDir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git clone --bare failed:\n%s", out)

	return bareDir
}

// runTestGit runs a git command in the given directory, creating it if needed.
func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	require.NoError(t, os.MkdirAll(dir, 0755), "creating dir %s", dir)

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s in %s failed:\n%s", strings.Join(args, " "), dir, out)
	return strings.TrimSpace(string(out))
}

// addCommitToBareRepo pushes a new commit to the bare repo by using a temporary
// working copy.
func addCommitToBareRepo(t *testing.T, bareDir string, files map[string]string, message string) {
	t.Helper()

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")

	cmd := exec.Command("git", "clone", bareDir, workDir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git clone for update failed:\n%s", out)

	runTestGit(t, workDir, "config", "user.email", "test@test.com")
	runTestGit(t, workDir, "config", "user.name", "Test User")

	for path, content := range files {
		full := filepath.Join(workDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755), "creating dir for %s", path)
		require.NoError(t, os.WriteFile(full, []byte(content), 0644), "writing %s", path)
	}
	runTestGit(t, workDir, "add", ".")
	runTestGit(t, workDir, "commit", "-m", message)
	runTestGit(t, workDir, "push")
}

func TestFetchGit_BasicClone(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"keel.yml":   "version: 1",
		"CLAUDE.md":  "# Instructions",
		"rules/a.md": "rule a",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		Original: bareDir,
	}

	localPath, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	info, err := os.Stat(localPath)
	require.NoError(t, err, "returned path should exist")
	require.True(t, info.IsDir(), "returned path should be a directory")

	// Verify expected files exist.
	for _, name := range []string{"keel.yml", "CLAUDE.md", "rules/a.md"} {
		assert.FileExists(t, filepath.Join(localPath, name))
	}

	// Verify content.
	data, err := os.ReadFile(filepath.Join(localPath, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "version: 1", string(data))
}

func TestFetchGit_BranchTargeting(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepoWithBranch(
		t,
		map[string]string{"file.txt": "default content"},
		"feature",
		map[string]string{"file.txt": "feature content", "extra.txt": "bonus"},
	)

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		Branch:   "feature",
		Original: bareDir + "#feature",
	}

	localPath, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(localPath, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "feature content", string(data))

	assert.FileExists(t, filepath.Join(localPath, "extra.txt"))
}

func TestFetchGit_SubPath(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"layers/base/keel.yml":     "version: 1",
		"layers/base/CLAUDE.md":    "# Base",
		"layers/security/keel.yml": "version: 2",
		"README.md":                "# Repo",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		SubPath:  "layers/base",
		Original: bareDir + "#:layers/base",
	}

	localPath, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(localPath, filepath.Join("layers", "base")),
		"expected path ending in layers/base, got %q", localPath)

	data, err := os.ReadFile(filepath.Join(localPath, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "version: 1", string(data))
}

func TestFetchGit_SubPathNotFound(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"keel.yml": "version: 1",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		SubPath:  "nonexistent/path",
		Original: bareDir + "#:nonexistent/path",
	}

	_, err := FetchGit(src, cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFetchGit_SubPathIsFile(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"keel.yml": "version: 1",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		SubPath:  "keel.yml",
		Original: bareDir + "#:keel.yml",
	}

	_, err := FetchGit(src, cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestFetchGit_DefaultBranch(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"config.yml": "default branch content",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		Original: bareDir,
	}

	localPath, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(localPath, "config.yml"))
	require.NoError(t, err)
	assert.Equal(t, "default branch content", string(data))
}

func TestFetchGit_IncrementalFetch(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"file.txt": "original content",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		Original: bareDir,
	}

	// First fetch: clone.
	localPath, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(localPath, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "original content", string(data))

	// Add a new commit to the bare repo.
	addCommitToBareRepo(t, bareDir, map[string]string{
		"file.txt": "updated content",
		"new.txt":  "new file",
	}, "second commit")

	// Second fetch: should pick up changes.
	localPath2, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	data, err = os.ReadFile(filepath.Join(localPath2, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "updated content", string(data))
	assert.FileExists(t, filepath.Join(localPath2, "new.txt"))
}

func TestFetchGit_Offline_CacheHit(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"policy.yml": "strict: true",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		Original: bareDir,
	}

	// First: populate the cache online.
	_, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	// Now fetch offline -- should succeed using cache.
	localPath, err := FetchGit(src, cacheBase, true)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(localPath, "policy.yml"))
	require.NoError(t, err)
	assert.Equal(t, "strict: true", string(data))
}

func TestFetchGit_Offline_CacheMiss(t *testing.T) {
	t.Parallel()
	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   "https://example.com/nonexistent/repo.git",
		Original: "https://example.com/nonexistent/repo.git",
	}

	_, err := FetchGit(src, cacheBase, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no cached data")
	assert.Contains(t, err.Error(), "--offline")
}

func TestFetchGit_Idempotent(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"file.txt": "content",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		Original: bareDir,
	}

	path1, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	path2, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	assert.Equal(t, path1, path2, "idempotent calls should return same path")

	data, err := os.ReadFile(filepath.Join(path2, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(data))
}

func TestRunGit_GitNotFound(t *testing.T) {
	resetGitPathCache()
	defer resetGitPathCache()

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	_, err := runGit("version")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git is required")
}

func TestFetchGit_MetaWritten(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"keel.yml": "version: 1",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		Original: bareDir,
	}

	_, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	hash := CacheKey(bareDir)
	metaPath := filepath.Join(cacheBase, "git", hash, "meta.json")

	meta, err := ReadGitMeta(metaPath)
	require.NoError(t, err)

	assert.Equal(t, bareDir, meta.URL)
	assert.False(t, meta.LastFetched.IsZero(), "meta.LastFetched should not be zero")
}

// ---------------------------------------------------------------------------
// containsPath tests
// ContainsPath is tested in pkg/pathutil/contains_test.go.

// ---------------------------------------------------------------------------
// FetchGit ext:: rejection tests
// ---------------------------------------------------------------------------

func TestFetchGit_ExtProtocol_Rejected(t *testing.T) {
	t.Parallel()
	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   "ext::sh -c 'evil command'",
		Original: "ext::sh -c 'evil command'",
	}

	_, err := FetchGit(src, cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ext::")
	assert.Contains(t, err.Error(), "dangerous")
}

// ---------------------------------------------------------------------------
// FetchGit subpath containment escape
// ---------------------------------------------------------------------------

func TestFetchGit_SubPathEscape_Rejected(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"keel.yml": "version: 1",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		SubPath:  "../../etc",
		Original: bareDir + "#:../../etc",
	}

	_, err := FetchGit(src, cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes")
}

func TestSanitizeBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{name: "simple", branch: "main", want: "main"},
		{name: "with slash", branch: "feature/auth", want: "feature_auth"},
		{name: "multiple slashes", branch: "release/v2/beta", want: "release_v2_beta"},
		{name: "no slashes", branch: "develop", want: "develop"},
		{name: "tag", branch: "v1.0.0", want: "v1.0.0"},
		{name: "dot-dot traversal", branch: "..", want: "_invalid_branch_"},
		{name: "single dot", branch: ".", want: "_invalid_branch_"},
		{name: "null byte", branch: "main\x00evil", want: "_invalid_branch_"},
		{name: "dot-dot with slash becomes safe", branch: "../escape", want: ".._escape"},
		{name: "spaces replaced", branch: "my branch", want: "my_branch"},
		{name: "semicolons replaced", branch: "branch;evil", want: "branch_evil"},
		{name: "backticks replaced", branch: "branch`cmd`", want: "branch_cmd_"},
		{name: "quotes replaced", branch: `branch"quoted"`, want: "branch_quoted_"},
		{name: "special chars collapsed", branch: "a & b | c", want: "a_b_c"},
		{name: "parentheses replaced", branch: "fix(scope)", want: "fix_scope_"},
		{name: "at sign replaced", branch: "user@host", want: "user_host"},
		{name: "tilde replaced", branch: "~username", want: "_username"},
		{name: "consecutive underscores collapsed", branch: "a///b", want: "a_b"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, sanitizeBranch(tt.branch), "sanitizeBranch(%q)", tt.branch)
		})
	}
}
