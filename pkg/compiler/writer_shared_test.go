package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeJSONFileKey_NewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	newData := map[string]any{
		"mcpServers": map[string]any{
			"server1": map[string]any{"command": "cmd1"},
		},
	}
	require.NoError(t, MergeJSONFileKey(path, newData, "mcpServers"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))
	servers := result["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "server1")
}

func TestMergeJSONFileKey_PreservesOtherKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Write existing file with OAuth token and a user MCP server.
	existing := map[string]any{
		"oauthToken": "sk-ant-secret",
		"ideFlags":   map[string]any{"connected": true},
		"mcpServers": map[string]any{
			"user-server": map[string]any{"command": "user-cmd"},
		},
	}
	existingData, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(path, existingData, 0644))

	// Merge Keel servers.
	newData := map[string]any{
		"mcpServers": map[string]any{
			"keel-server": map[string]any{"command": "keel-cmd"},
		},
	}
	require.NoError(t, MergeJSONFileKey(path, newData, "mcpServers"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	// OAuth token preserved.
	assert.Equal(t, "sk-ant-secret", result["oauthToken"])
	// IDE flags preserved.
	assert.NotNil(t, result["ideFlags"])
	// Both servers present.
	servers := result["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "user-server")
	assert.Contains(t, servers, "keel-server")
}

func TestMergeJSONFileKey_OverlappingServers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Write existing file with a server that Keel will also provide.
	existing := map[string]any{
		"mcpServers": map[string]any{
			"shared-server": map[string]any{"command": "old-cmd", "args": []string{"old"}},
			"user-server":   map[string]any{"command": "user-cmd"},
		},
	}
	existingData, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(path, existingData, 0644))

	// Keel provides a newer version of shared-server.
	newData := map[string]any{
		"mcpServers": map[string]any{
			"shared-server": map[string]any{"command": "new-cmd", "args": []string{"new"}},
		},
	}
	require.NoError(t, MergeJSONFileKey(path, newData, "mcpServers"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers := result["mcpServers"].(map[string]any)
	// Keel's version wins for shared-server.
	shared := servers["shared-server"].(map[string]any)
	assert.Equal(t, "new-cmd", shared["command"])
	// User server still present.
	assert.Contains(t, servers, "user-server")
}

func TestMergeJSONFileKey_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Write invalid JSON.
	require.NoError(t, os.WriteFile(path, []byte("not-json{"), 0644))

	newData := map[string]any{
		"mcpServers": map[string]any{
			"server1": map[string]any{"command": "cmd1"},
		},
	}
	err := MergeJSONFileKey(path, newData, "mcpServers")
	assert.Error(t, err, "should return error for invalid JSON")
	assert.Contains(t, err.Error(), "parsing existing file")
}

func TestMergeJSONFileKey_NoMergeKeyInNewData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Write existing file.
	existing := map[string]any{
		"oauthToken": "sk-ant-secret",
		"mcpServers": map[string]any{
			"existing": map[string]any{"command": "cmd"},
		},
	}
	existingData, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(path, existingData, 0644))

	// Merge with no mcpServers key.
	newData := map[string]any{
		"otherKey": "value",
	}
	require.NoError(t, MergeJSONFileKey(path, newData, "mcpServers"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	// Existing data preserved unchanged.
	assert.Equal(t, "sk-ant-secret", result["oauthToken"])
	servers := result["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "existing")
}

func TestWriteBinFiles_PathTraversalRejection(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	bins := []BinFile{
		{
			RelPath: "../../escape",
			Content: []byte("malicious"),
		},
	}

	_, err := WriteBinFiles(bins, tmpDir)
	require.Error(t, err, "expected error for path traversal")
	assert.Contains(t, err.Error(), `bin file "../../escape" escapes output directory`)
}

func TestWriteBinFiles_ValidFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	bins := []BinFile{
		{
			RelPath: "mytool",
			Content: []byte("#!/bin/sh\necho hello"),
		},
		{
			RelPath: "subdir/othertool",
			Content: []byte("#!/bin/sh\necho world"),
		},
	}

	records, err := WriteBinFiles(bins, tmpDir)
	require.NoError(t, err)
	require.Len(t, records, 2, "should have one record per file")

	// Check both file records exist
	var foundTool1, foundTool2 bool
	for _, rec := range records {
		if rec.Output == ".keel/bin/mytool" && rec.Strategy == "overlay" {
			foundTool1 = true
			assert.NotEmpty(t, rec.Hash, "mytool should have a hash")
		}
		if rec.Output == ".keel/bin/subdir/othertool" && rec.Strategy == "overlay" {
			foundTool2 = true
			assert.NotEmpty(t, rec.Hash, "othertool should have a hash")
		}
	}
	assert.True(t, foundTool1, "expected record for .keel/bin/mytool")
	assert.True(t, foundTool2, "expected record for .keel/bin/subdir/othertool")

	// Verify files were created with executable permissions
	tool1Path := filepath.Join(tmpDir, ".keel", "bin", "mytool")
	content1, err := os.ReadFile(tool1Path)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho hello", string(content1))
	info1, err := os.Stat(tool1Path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info1.Mode().Perm())

	tool2Path := filepath.Join(tmpDir, ".keel", "bin", "subdir", "othertool")
	content2, err := os.ReadFile(tool2Path)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho world", string(content2))
	info2, err := os.Stat(tool2Path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info2.Mode().Perm())
}
