package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindKeelState_DirectoryWithState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a .keel.json directly in the directory
	manifest := &CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
	}
	statePath := filepath.Join(dir, ".keel.json")
	require.NoError(t, WriteKeelState(statePath, manifest))

	manifests, paths, err := FindKeelState(dir, []string{".claude", ".cursor"})
	require.NoError(t, err)
	require.Len(t, manifests, 1)
	require.Len(t, paths, 1)
	assert.Equal(t, "claude", manifests[0].Agent)
	assert.Equal(t, dir, paths[0])
}

func TestFindKeelState_AgentSubdirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create .keel.json in .claude and .cursor subdirs
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	claudeManifest := &CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
	}
	require.NoError(t, WriteKeelState(filepath.Join(claudeDir, ".keel.json"), claudeManifest))

	cursorDir := filepath.Join(dir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))
	cursorManifest := &CompiledManifest{
		Agent:       "cursor",
		KeelVersion: "1.0.0",
	}
	require.NoError(t, WriteKeelState(filepath.Join(cursorDir, ".keel.json"), cursorManifest))

	manifests, paths, err := FindKeelState(dir, []string{".claude", ".cursor", ".aider"})
	require.NoError(t, err)
	require.Len(t, manifests, 2)
	require.Len(t, paths, 2)

	// Verify both agents were found
	agents := make(map[string]bool)
	for _, m := range manifests {
		agents[m.Agent] = true
	}
	assert.True(t, agents["claude"])
	assert.True(t, agents["cursor"])
}

func TestFindKeelState_NoState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	manifests, paths, err := FindKeelState(dir, []string{".claude", ".cursor"})
	require.NoError(t, err)
	assert.Empty(t, manifests)
	assert.Empty(t, paths)
}

func TestLoadKeelState_ValidFile_IO(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	manifest := &CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
	}
	statePath := filepath.Join(dir, ".keel.json")
	require.NoError(t, WriteKeelState(statePath, manifest))

	loaded, err := LoadKeelState(dir)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "claude", loaded.Agent)
	assert.Equal(t, "1.0.0", loaded.KeelVersion)
}

func TestLoadKeelState_Missing_IO(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	loaded, err := LoadKeelState(dir)
	require.NoError(t, err)
	assert.Nil(t, loaded, "LoadKeelState should return nil when no state file exists")
}

func TestLoadKeelState_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	statePath := filepath.Join(dir, ".keel.json")
	require.NoError(t, os.WriteFile(statePath, []byte("invalid json{"), 0644))

	loaded, err := LoadKeelState(dir)
	require.Error(t, err)
	assert.Nil(t, loaded)
	assert.Contains(t, err.Error(), "parsing compiled state")
}

func TestWriteKeelState_CreatesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	manifest := &CompiledManifest{
		Agent:       "aider",
		KeelVersion: "1.0.0",
	}
	statePath := filepath.Join(dir, ".keel.json")
	err := WriteKeelState(statePath, manifest)
	require.NoError(t, err)

	// Verify file exists and is readable
	data, err := os.ReadFile(statePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"agent": "aider"`)
	assert.Contains(t, string(data), `"keel_version": "1.0.0"`)

	// Verify trailing newline for POSIX compatibility
	assert.True(t, len(data) > 0 && data[len(data)-1] == '\n', "state file should end with newline")
}

func TestHashContent_Deterministic(t *testing.T) {
	t.Parallel()

	content := []byte("test content")
	hash1 := HashContent(content)
	hash2 := HashContent(content)

	assert.Equal(t, hash1, hash2, "HashContent should be deterministic")
	assert.Contains(t, hash1, "sha256:")
}

func TestHashContent_DifferentContent(t *testing.T) {
	t.Parallel()

	hash1 := HashContent([]byte("content 1"))
	hash2 := HashContent([]byte("content 2"))

	assert.NotEqual(t, hash1, hash2, "different content should produce different hashes")
}

func TestHashContent_EmptyContent(t *testing.T) {
	t.Parallel()

	hash := HashContent([]byte{})
	assert.Contains(t, hash, "sha256:")
	assert.NotEmpty(t, hash)
}

func TestHashLayerDir_BasicDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create some files
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0644))

	hash, err := HashLayerDir(dir)
	require.NoError(t, err)
	assert.Contains(t, hash, "sha256:")
}

func TestHashLayerDir_Deterministic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create files in non-lexical order
	require.NoError(t, os.WriteFile(filepath.Join(dir, "z.txt"), []byte("z"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "m.txt"), []byte("m"), 0644))

	hash1, err := HashLayerDir(dir)
	require.NoError(t, err)

	hash2, err := HashLayerDir(dir)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "HashLayerDir should be deterministic regardless of creation order")
}

func TestHashLayerDir_SkipsGitDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a .git directory with content
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644))

	// Create a regular file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644))

	hash1, err := HashLayerDir(dir)
	require.NoError(t, err)

	// Hash should be the same without .git directory
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "file.txt"), []byte("content"), 0644))

	hash2, err := HashLayerDir(dir2)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, ".git directory should be ignored in hash calculation")
}

func TestHashLayerDir_EmptyDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	hash, err := HashLayerDir(dir)
	require.NoError(t, err)
	assert.Contains(t, hash, "sha256:")
}

func TestHashLayerDir_NestedDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create nested structure
	nested := filepath.Join(dir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "file.txt"), []byte("nested"), 0644))

	hash, err := HashLayerDir(dir)
	require.NoError(t, err)
	assert.Contains(t, hash, "sha256:")
}

func TestHashLayerDir_NonExistent(t *testing.T) {
	t.Parallel()

	_, err := HashLayerDir("/nonexistent/directory")
	require.Error(t, err)
}
