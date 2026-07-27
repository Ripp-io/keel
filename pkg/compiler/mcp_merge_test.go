package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeJSONFileKey_SecondCompilePreservesUserContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, ".claude.json")

	// Simulate user-created content in .claude.json before Keel.
	userContent := map[string]any{
		"oauthToken": "secret123",
		"userFlag":   true,
		"mcpServers": map[string]any{
			"user-server": map[string]any{
				"command": "user-script",
			},
		},
	}
	require.NoError(t, WriteJSONFile(targetPath, userContent))

	// First compile: Keel adds a managed server.
	firstData := map[string]any{
		"mcpServers": map[string]any{
			"keel-server-1": map[string]any{
				"command": "keel-script-1",
			},
		},
	}
	require.NoError(t, MergeJSONFileKey(targetPath, firstData, "mcpServers"))

	// Verify first compile merged correctly.
	data, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	var afterFirst map[string]any
	require.NoError(t, json.Unmarshal(data, &afterFirst))

	assert.Equal(t, "secret123", afterFirst["oauthToken"], "user OAuth token should be preserved")
	assert.Equal(t, true, afterFirst["userFlag"], "user flag should be preserved")

	servers, ok := afterFirst["mcpServers"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, servers, "user-server", "user server should be preserved")
	assert.Contains(t, servers, "keel-server-1", "keel server should be added")

	// Second compile: Keel updates the managed server and adds a new one.
	secondData := map[string]any{
		"mcpServers": map[string]any{
			"keel-server-1": map[string]any{
				"command": "keel-script-1-updated",
			},
			"keel-server-2": map[string]any{
				"command": "keel-script-2",
			},
		},
	}
	require.NoError(t, MergeJSONFileKey(targetPath, secondData, "mcpServers"))

	// Verify second compile preserved user content and updated Keel servers.
	data, err = os.ReadFile(targetPath)
	require.NoError(t, err)
	var afterSecond map[string]any
	require.NoError(t, json.Unmarshal(data, &afterSecond))

	assert.Equal(t, "secret123", afterSecond["oauthToken"], "user OAuth token should still be preserved")
	assert.Equal(t, true, afterSecond["userFlag"], "user flag should still be preserved")

	servers, ok = afterSecond["mcpServers"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, servers, "user-server", "user server should still be preserved")
	assert.Contains(t, servers, "keel-server-1", "keel server 1 should be updated")
	assert.Contains(t, servers, "keel-server-2", "keel server 2 should be added")

	// Check that keel-server-1 was updated.
	server1, ok := servers["keel-server-1"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "keel-script-1-updated", server1["command"])
}

func TestMergeJSONFileKey_MalformedJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, ".claude.json")

	// Write invalid JSON to the merge target.
	require.NoError(t, os.WriteFile(targetPath, []byte("{invalid json"), 0644))

	newData := map[string]any{
		"mcpServers": map[string]any{
			"keel-server": map[string]any{"command": "keel"},
		},
	}

	err := MergeJSONFileKey(targetPath, newData, "mcpServers")
	assert.Error(t, err, "should return error for malformed JSON")
	assert.Contains(t, err.Error(), "parsing existing file")
}

func TestMergeJSONFileKey_FileNotExist(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, ".claude.json")

	// File doesn't exist — should write newData as-is.
	newData := map[string]any{
		"mcpServers": map[string]any{
			"keel-server": map[string]any{"command": "keel"},
		},
	}

	require.NoError(t, MergeJSONFileKey(targetPath, newData, "mcpServers"))

	// Verify file was created with newData.
	data, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers, ok := result["mcpServers"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, servers, "keel-server")
}
