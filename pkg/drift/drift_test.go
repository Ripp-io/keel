package drift

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

func TestDetect_NoDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content1 := []byte("file one content")
	content2 := []byte("file two content")
	testutil.WriteFile(t, dir, "file1.txt", content1)
	testutil.WriteFile(t, dir, "file2.txt", content2)

	m := &manifest.CompiledManifest{
		Agent: "test",
		FileOps: []manifest.FileRecord{
			{Output: "file1.txt", Hash: manifest.HashContent(content1)},
			{Output: "file2.txt", Hash: manifest.HashContent(content2)},
		},
	}

	result, err := Detect(dir, m)
	require.NoError(t, err)
	assert.Equal(t, "test", result.Agent)
	assert.Empty(t, result.Modified, "expected no modified files")
	assert.Empty(t, result.Missing, "expected no missing files")
	assert.False(t, result.HasDrift())
}

func TestDetect_ModifiedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	originalContent := []byte("original content")
	modifiedContent := []byte("modified content")
	testutil.WriteFile(t, dir, "modified.txt", modifiedContent)

	m := &manifest.CompiledManifest{
		Agent: "test",
		FileOps: []manifest.FileRecord{
			{Output: "modified.txt", Hash: manifest.HashContent(originalContent)},
		},
	}

	result, err := Detect(dir, m)
	require.NoError(t, err)
	assert.Len(t, result.Modified, 1)
	assert.Equal(t, "modified.txt", result.Modified[0].Path)
	assert.Equal(t, manifest.HashContent(originalContent), result.Modified[0].Expected)
	assert.Equal(t, manifest.HashContent(modifiedContent), result.Modified[0].Actual)
	assert.Empty(t, result.Missing)
	assert.True(t, result.HasDrift())
}

func TestDetect_MissingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := []byte("content")
	m := &manifest.CompiledManifest{
		Agent: "test",
		FileOps: []manifest.FileRecord{
			{Output: "missing.txt", Hash: manifest.HashContent(content)},
		},
	}

	result, err := Detect(dir, m)
	require.NoError(t, err)
	assert.Empty(t, result.Modified)
	assert.Len(t, result.Missing, 1)
	assert.Equal(t, "missing.txt", result.Missing[0].Path)
	assert.Equal(t, manifest.HashContent(content), result.Missing[0].Expected)
	assert.Empty(t, result.Missing[0].Actual)
	assert.True(t, result.HasDrift())
}

func TestDetect_MixedChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	original := []byte("original")
	modified := []byte("modified")
	unchanged := []byte("unchanged")

	testutil.WriteFile(t, dir, "modified.txt", modified)
	testutil.WriteFile(t, dir, "unchanged.txt", unchanged)

	m := &manifest.CompiledManifest{
		Agent: "test",
		FileOps: []manifest.FileRecord{
			{Output: "modified.txt", Hash: manifest.HashContent(original)},
			{Output: "missing.txt", Hash: manifest.HashContent(original)},
			{Output: "unchanged.txt", Hash: manifest.HashContent(unchanged)},
		},
	}

	result, err := Detect(dir, m)
	require.NoError(t, err)
	assert.Len(t, result.Modified, 1)
	assert.Equal(t, "modified.txt", result.Modified[0].Path)
	assert.Len(t, result.Missing, 1)
	assert.Equal(t, "missing.txt", result.Missing[0].Path)
	assert.True(t, result.HasDrift())
}

func TestDetect_EmptyManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	m := &manifest.CompiledManifest{
		Agent:   "test",
		FileOps: []manifest.FileRecord{},
	}

	result, err := Detect(dir, m)
	require.NoError(t, err)
	assert.Empty(t, result.Modified)
	assert.Empty(t, result.Missing)
	assert.False(t, result.HasDrift())
}

func TestDetect_SkipsDirectoryEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	m := &manifest.CompiledManifest{
		Agent: "test",
		FileOps: []manifest.FileRecord{
			{Output: ".claude/commands/", Strategy: "overlay"},
		},
	}

	result, err := Detect(dir, m)
	require.NoError(t, err)
	assert.Empty(t, result.Modified)
	assert.Empty(t, result.Missing)
}

func TestDetect_SkipsFilesWithoutHash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	testutil.WriteFile(t, dir, "file.txt", []byte("content"))

	m := &manifest.CompiledManifest{
		Agent: "test",
		FileOps: []manifest.FileRecord{
			{Output: "file.txt", Hash: ""},
		},
	}

	result, err := Detect(dir, m)
	require.NoError(t, err)
	assert.Empty(t, result.Modified)
	assert.Empty(t, result.Missing)
}

func TestDetect_NestedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := []byte("nested content")
	testutil.WriteFile(t, dir, "a/b/c/nested.txt", content)

	m := &manifest.CompiledManifest{
		Agent: "test",
		FileOps: []manifest.FileRecord{
			{Output: "a/b/c/nested.txt", Hash: manifest.HashContent(content)},
		},
	}

	result, err := Detect(dir, m)
	require.NoError(t, err)
	assert.Empty(t, result.Modified)
	assert.Empty(t, result.Missing)
}

func TestDetectSemantic_NoEffectiveState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	m := &manifest.CompiledManifest{
		Agent:          "test",
		EffectiveState: nil,
	}

	reader := func(outputDir string) (*manifest.EffectiveState, error) {
		return &manifest.EffectiveState{}, nil
	}

	result, err := DetectSemantic(dir, m, reader)
	require.NoError(t, err)
	assert.Nil(t, result, "expected nil when manifest has no effective_state")
}

func TestDetectSemantic_NoChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	state := &manifest.EffectiveState{
		Permissions: &manifest.EffectivePermissions{
			Allow: []string{"read", "write"},
			Deny:  []string{"delete"},
		},
		MCPServers: []manifest.EffectiveMCPServer{
			{Name: "server1", Type: "stdio"},
		},
	}

	m := &manifest.CompiledManifest{
		Agent:          "test",
		EffectiveState: state,
	}

	reader := func(outputDir string) (*manifest.EffectiveState, error) {
		return state, nil
	}

	result, err := DetectSemantic(dir, m, reader)
	require.NoError(t, err)
	if result != nil {
		assert.False(t, result.HasChanges())
	}
}

func TestDetectSemantic_PermissionsChanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	oldState := &manifest.EffectiveState{
		Permissions: &manifest.EffectivePermissions{
			Allow: []string{"read"},
			Deny:  []string{"delete"},
		},
	}

	newState := &manifest.EffectiveState{
		Permissions: &manifest.EffectivePermissions{
			Allow: []string{"read", "write"},
			Deny:  []string{},
		},
	}

	m := &manifest.CompiledManifest{
		Agent:          "test",
		EffectiveState: oldState,
	}

	reader := func(outputDir string) (*manifest.EffectiveState, error) {
		return newState, nil
	}

	result, err := DetectSemantic(dir, m, reader)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.HasChanges())
	assert.NotNil(t, result.Permissions)
	assert.Contains(t, result.Permissions.AllowAdded, "write")
	assert.Contains(t, result.Permissions.DenyRemoved, "delete")
}

func TestDetectSemantic_MCPServersChanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	oldState := &manifest.EffectiveState{
		MCPServers: []manifest.EffectiveMCPServer{
			{Name: "server1", Type: "stdio"},
		},
	}

	newState := &manifest.EffectiveState{
		MCPServers: []manifest.EffectiveMCPServer{
			{Name: "server1", Type: "stdio"},
			{Name: "server2", Type: "sse"},
		},
	}

	m := &manifest.CompiledManifest{
		Agent:          "test",
		EffectiveState: oldState,
	}

	reader := func(outputDir string) (*manifest.EffectiveState, error) {
		return newState, nil
	}

	result, err := DetectSemantic(dir, m, reader)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.HasChanges())
	assert.NotNil(t, result.MCPServers)
	assert.Contains(t, result.MCPServers.Added, "server2")
}

func TestDetectSemantic_ReaderError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	m := &manifest.CompiledManifest{
		Agent: "test",
		EffectiveState: &manifest.EffectiveState{
			Permissions: &manifest.EffectivePermissions{
				Allow: []string{"read"},
			},
		},
	}

	reader := func(outputDir string) (*manifest.EffectiveState, error) {
		return nil, os.ErrNotExist
	}

	_, err := DetectSemantic(dir, m, reader)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading current state")
}
