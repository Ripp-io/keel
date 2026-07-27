package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// writeKeelStateFile writes a .keel.json manifest inside the agent dir.
func writeKeelStateFile(t *testing.T, outputDir, outputRoot string, manifest CompiledManifest) {
	t.Helper()
	agentDir := filepath.Join(outputDir, outputRoot)
	require.NoError(t, os.MkdirAll(agentDir, 0755), "mkdir agent dir")
	data, err := json.MarshalIndent(&manifest, "", "  ")
	require.NoError(t, err, "marshaling manifest")
	data = append(data, '\n')
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, ".keel.json"), data, 0644), "writing .keel.json")
}

func TestPruneStaleOutput_NoManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result, err := PruneStaleOutput(dir, ".claude", nil, nil)
	require.NoError(t, err)
	assert.Nil(t, result, "expected nil result for first compile")
}

func TestPruneStaleOutput_RemovesStaleFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFile(t, dir, ".claude/settings.json", []byte(`{}`))
	testutil.WriteFile(t, dir, "CLAUDE.md", []byte("old instructions"))
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		Agent: "claude",
		FileOps: []FileRecord{
			{Output: ".claude/settings.json", Strategy: "deep-merge"},
			{Output: "CLAUDE.md", Strategy: "concatenate"},
		},
	})
	newFiles := []FileRecord{
		{Output: ".claude/settings.json", Strategy: "deep-merge"},
	}
	result, err := PruneStaleOutput(dir, ".claude", newFiles, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []string{"CLAUDE.md"}, result.Removed)
	assert.False(t, testutil.FileExists(filepath.Join(dir, "CLAUDE.md")), "CLAUDE.md should have been deleted")
	assert.True(t, testutil.FileExists(filepath.Join(dir, ".claude/settings.json")), "settings.json should still exist")
}

func TestPruneStaleOutput_SameFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFile(t, dir, ".claude/settings.json", []byte(`{}`))
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		FileOps: []FileRecord{
			{Output: ".claude/settings.json", Strategy: "deep-merge"},
		},
	})
	newFiles := []FileRecord{
		{Output: ".claude/settings.json", Strategy: "deep-merge"},
	}
	result, err := PruneStaleOutput(dir, ".claude", newFiles, nil)
	require.NoError(t, err)
	assert.Empty(t, result.Removed)
}

func TestPruneStaleOutput_PreservesUserFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFile(t, dir, ".claude/settings.json", []byte(`{}`))
	testutil.WriteFile(t, dir, ".claude/my-custom-skill.md", []byte("user content"))
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		FileOps: []FileRecord{
			{Output: ".claude/settings.json", Strategy: "deep-merge"},
		},
	})
	result, err := PruneStaleOutput(dir, ".claude", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{".claude/settings.json"}, result.Removed)
	assert.True(t, testutil.FileExists(filepath.Join(dir, ".claude/my-custom-skill.md")), "user-created file should be preserved")
}

func TestPruneStaleOutput_OverlayDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFile(t, dir, ".claude/commands/deploy.md", []byte("deploy"))
	testutil.WriteFile(t, dir, ".claude/commands/review.md", []byte("review"))
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		FileOps: []FileRecord{
			{Output: ".claude/commands", Strategy: "overlay"},
		},
	})
	newOverlays := map[string][]string{
		".claude/commands/deploy.md": {"team"},
	}
	result, err := PruneStaleOutput(dir, ".claude", nil, newOverlays)
	require.NoError(t, err)
	assert.Equal(t, []string{".claude/commands/review.md"}, result.Removed)
	assert.True(t, testutil.FileExists(filepath.Join(dir, ".claude/commands/deploy.md")), "deploy.md should still exist (in new overlays)")
}

func TestPruneStaleOutput_RemovesEmptyParents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFile(t, dir, ".claude/agents/reviewer.md", []byte("reviewer"))
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		FileOps: []FileRecord{
			{Output: ".claude/agents", Strategy: "overlay"},
		},
	})
	result, err := PruneStaleOutput(dir, ".claude", nil, nil)
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.False(t, testutil.FileExists(filepath.Join(dir, ".claude/agents")),
		".claude/agents/ directory should have been removed (empty after prune)")
}

func TestPruneStaleOutput_MissingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		FileOps: []FileRecord{
			{Output: "CLAUDE.md", Strategy: "concatenate"},
		},
	})
	result, err := PruneStaleOutput(dir, ".claude", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"CLAUDE.md"}, result.Missing)
	assert.Empty(t, result.Removed)
}

func TestPruneStaleOutput_SkipsManifestFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		FileOps: []FileRecord{
			{Output: "CLAUDE.md", Strategy: "concatenate"},
		},
	})
	testutil.WriteFile(t, dir, "CLAUDE.md", []byte("old"))
	result, err := PruneStaleOutput(dir, ".claude", nil, nil)
	require.NoError(t, err)
	assert.True(t, testutil.FileExists(filepath.Join(dir, ".claude/.keel.json")), ".keel.json should not be pruned")
	assert.Equal(t, []string{"CLAUDE.md"}, result.Removed)
}

func TestCleanAllOutput_RemovesEverything(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFile(t, dir, ".claude/settings.json", []byte(`{}`))
	testutil.WriteFile(t, dir, ".claude/commands/deploy.md", []byte("deploy"))
	testutil.WriteFile(t, dir, "CLAUDE.md", []byte("instructions"))
	testutil.WriteFile(t, dir, ".claude/my-custom-file.txt", []byte("user content"))
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		FileOps: []FileRecord{
			{Output: ".claude/settings.json", Strategy: "deep-merge"},
			{Output: ".claude/commands", Strategy: "overlay"},
			{Output: "CLAUDE.md", Strategy: "concatenate"},
		},
	})
	result, err := CleanAllOutput(dir, ".claude", false)
	require.NoError(t, err)
	sort.Strings(result.Removed)
	expected := []string{
		".claude/.keel.json",
		".claude/commands/deploy.md",
		".claude/settings.json",
		"CLAUDE.md",
	}
	require.Len(t, result.Removed, len(expected))
	assert.Equal(t, expected, result.Removed)
	assert.True(t, testutil.FileExists(filepath.Join(dir, ".claude/my-custom-file.txt")), "user-created file should be preserved")
}

func TestCleanAllOutput_PreservesUserFilesInOverlayDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFile(t, dir, ".claude/skills/ripp-debug/SKILL.md", []byte("keel skill"))
	testutil.WriteFile(t, dir, ".claude/skills/ripp-plan/SKILL.md", []byte("keel skill"))
	testutil.WriteFile(t, dir, ".claude/skills/add-compiler-adapter/SKILL.md", []byte("user skill"))
	testutil.WriteFile(t, dir, ".claude/skills/config-surface-audit/SKILL.md", []byte("user skill"))
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		FileOps: []FileRecord{
			{Output: ".claude/skills/", Strategy: "overlay"},
		},
		Overlays: map[string][]string{
			".claude/skills/ripp-debug/SKILL.md": {"layer"},
			".claude/skills/ripp-plan/SKILL.md":  {"layer"},
		},
	})
	result, err := CleanAllOutput(dir, ".claude", false)
	require.NoError(t, err)
	sort.Strings(result.Removed)
	expected := []string{
		".claude/.keel.json",
		".claude/skills/ripp-debug/SKILL.md",
		".claude/skills/ripp-plan/SKILL.md",
	}
	require.Len(t, result.Removed, len(expected))
	assert.Equal(t, expected, result.Removed)
	assert.True(t, testutil.FileExists(filepath.Join(dir, ".claude/skills/add-compiler-adapter/SKILL.md")),
		"user skill add-compiler-adapter should be preserved")
	assert.True(t, testutil.FileExists(filepath.Join(dir, ".claude/skills/config-surface-audit/SKILL.md")),
		"user skill config-surface-audit should be preserved")
}

func TestCleanAllOutput_NoManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result, err := CleanAllOutput(dir, ".claude", false)
	require.NoError(t, err)
	assert.Nil(t, result, "expected nil result")
}

func TestLoadKeelState_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cm, err := LoadKeelState(dir)
	require.NoError(t, err)
	assert.Nil(t, cm, "expected nil manifest")
}

func TestLoadKeelState_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeKeelStateFile(t, dir, ".", CompiledManifest{
		Agent: "claude",
		FileOps: []FileRecord{
			{Output: "CLAUDE.md", Strategy: "concatenate"},
		},
	})
	cm, err := LoadKeelState(dir)
	require.NoError(t, err)
	require.NotNil(t, cm, "expected non-nil manifest")
	assert.Equal(t, "claude", cm.Agent)
	assert.Len(t, cm.FileOps, 1)
}

func TestRemoveManagedMCPServers_RemovesOnlyManaged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")
	testutil.WriteJSON(t, path, map[string]any{
		"oauthToken": "sk-ant-secret",
		"mcpServers": map[string]any{
			"keel-server1": map[string]any{"command": "cmd1"},
			"keel-server2": map[string]any{"command": "cmd2"},
			"user-server":  map[string]any{"command": "user-cmd"},
		},
	})

	err := removeManagedMCPServers(path, []string{"keel-server1", "keel-server2"})
	require.NoError(t, err)

	result := testutil.ReadJSONMap(t, path)

	// OAuth token preserved.
	assert.Equal(t, "sk-ant-secret", result["oauthToken"])
	// User server preserved, Keel servers removed.
	servers := result["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "user-server")
	assert.NotContains(t, servers, "keel-server1")
	assert.NotContains(t, servers, "keel-server2")
	assert.Len(t, servers, 1)
}

func TestRemoveManagedMCPServers_RemovesKeyWhenEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")
	testutil.WriteJSON(t, path, map[string]any{
		"oauthToken": "sk-ant-secret",
		"mcpServers": map[string]any{
			"keel-server": map[string]any{"command": "cmd1"},
		},
	})

	err := removeManagedMCPServers(path, []string{"keel-server"})
	require.NoError(t, err)

	result := testutil.ReadJSONMap(t, path)
	assert.Equal(t, "sk-ant-secret", result["oauthToken"])
	// mcpServers key should be removed entirely.
	_, hasMCP := result["mcpServers"]
	assert.False(t, hasMCP, "mcpServers should be removed when empty")
}

func TestRemoveManagedMCPServers_MissingFile(t *testing.T) {
	t.Parallel()
	err := removeManagedMCPServers(filepath.Join(t.TempDir(), "nonexistent.json"), []string{"server"})
	assert.NoError(t, err, "missing file should not error")
}

func TestRemoveManagedMCPServers_NoMCPKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")
	testutil.WriteJSON(t, path, map[string]any{
		"oauthToken": "sk-ant-secret",
	})

	err := removeManagedMCPServers(path, []string{"server"})
	require.NoError(t, err)

	result := testutil.ReadJSONMap(t, path)
	assert.Equal(t, "sk-ant-secret", result["oauthToken"])
}

func TestCleanAllOutput_MergeIntoFile_PreservesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a shared .claude.json with both user and Keel-managed servers.
	claudeJSON := filepath.Join(dir, ".claude.json")
	testutil.WriteJSON(t, claudeJSON, map[string]any{
		"oauthToken": "sk-ant-preserve-me",
		"mcpServers": map[string]any{
			"user-server": map[string]any{"command": "user-cmd"},
			"keel-server": map[string]any{"command": "keel-cmd"},
		},
	})

	// Also create a regular settings file.
	testutil.WriteFile(t, dir, ".claude/settings.json", []byte(`{}`))

	// Write manifest with merge-into strategy and managed servers.
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		Agent: "claude",
		FileOps: []FileRecord{
			{Output: ".claude/settings.json", Strategy: "deep-merge"},
			{Output: ".claude.json", Strategy: "merge-into"},
		},
		ManagedMCPServers: []string{"keel-server"},
	})

	result, err := CleanAllOutput(dir, ".claude", false)
	require.NoError(t, err)
	require.NotNil(t, result)

	// .claude.json should still exist (not deleted).
	assert.True(t, testutil.FileExists(claudeJSON), ".claude.json should not be deleted")

	// Read and verify contents.
	afterClean := testutil.ReadJSONMap(t, claudeJSON)

	// OAuth token still preserved.
	assert.Equal(t, "sk-ant-preserve-me", afterClean["oauthToken"])

	// User server still present, Keel server removed.
	if servers, ok := afterClean["mcpServers"].(map[string]any); ok {
		assert.Contains(t, servers, "user-server")
		assert.NotContains(t, servers, "keel-server")
	}

	// settings.json and .keel.json should be deleted normally.
	assert.Contains(t, result.Removed, ".claude/settings.json")
	assert.Contains(t, result.Removed, ".claude/.keel.json")
	// .claude.json should NOT appear in removed list.
	assert.NotContains(t, result.Removed, ".claude.json")
}

func TestPruneStaleOutput_MergeIntoFile_NotDeleted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a shared .claude.json.
	claudeJSON := filepath.Join(dir, ".claude.json")
	testutil.WriteJSON(t, claudeJSON, map[string]any{
		"oauthToken": "sk-ant-preserve-me",
		"mcpServers": map[string]any{
			"user-server": map[string]any{"command": "user-cmd"},
			"keel-server": map[string]any{"command": "keel-cmd"},
		},
	})

	// Old manifest had merge-into file.
	writeKeelStateFile(t, dir, ".claude", CompiledManifest{
		Agent: "claude",
		FileOps: []FileRecord{
			{Output: ".claude.json", Strategy: "merge-into"},
		},
		ManagedMCPServers: []string{"keel-server"},
	})

	// New compilation no longer produces .claude.json.
	result, err := PruneStaleOutput(dir, ".claude", nil, nil)
	require.NoError(t, err)

	// .claude.json should still exist.
	assert.True(t, testutil.FileExists(claudeJSON), ".claude.json should not be deleted during prune")

	// Should not appear in removed list.
	if result != nil {
		assert.NotContains(t, result.Removed, ".claude.json")
	}

	// Keel-managed servers should have been cleaned from the file.
	afterPrune := testutil.ReadJSONMap(t, claudeJSON)
	assert.Equal(t, "sk-ant-preserve-me", afterPrune["oauthToken"])
	if servers, ok := afterPrune["mcpServers"].(map[string]any); ok {
		assert.Contains(t, servers, "user-server")
		assert.NotContains(t, servers, "keel-server")
	}
}

