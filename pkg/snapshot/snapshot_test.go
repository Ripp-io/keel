package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestCreate(t *testing.T) {
	dir := t.TempDir()

	// Set up a mock compiled output
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Create manifest
	mf := manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: ".claude/settings.json", Strategy: "deep-merge"},
			{Output: "CLAUDE.md", Strategy: "concatenate"},
		},
		Overlays: map[string][]string{
			".claude/commands/test.md": {"layer1"},
		},
	}
	manifestData, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(manifestData, '\n'), 0644))

	// Create the files referenced in FileOps
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"model":"claude-sonnet-4"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Instructions"), 0644))

	// Create overlay file
	commandsDir := filepath.Join(claudeDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("# Command"), 0644))

	// Create snapshot
	snap, err := Create(dir, claudeDir)
	require.NoError(t, err)
	require.NotNil(t, snap)

	// Verify snapshot structure
	assert.Equal(t, "claude", snap.Agent)
	assert.NotEmpty(t, snap.ID)
	assert.NotEmpty(t, snap.Dir)

	// Verify files were copied
	assert.FileExists(t, filepath.Join(snap.Dir, ".claude/settings.json"))
	assert.FileExists(t, filepath.Join(snap.Dir, "CLAUDE.md"))
	assert.FileExists(t, filepath.Join(snap.Dir, ".claude/commands/test.md"))
	assert.FileExists(t, filepath.Join(snap.Dir, ".claude/.keel.json"))
	assert.FileExists(t, filepath.Join(snap.Dir, "snapshot.json"))

	// Verify file content
	settings, err := os.ReadFile(filepath.Join(snap.Dir, ".claude/settings.json"))
	require.NoError(t, err)
	assert.Contains(t, string(settings), "claude-sonnet-4")

	instructions, err := os.ReadFile(filepath.Join(snap.Dir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Instructions", string(instructions))
}

func TestCreateNoManifest(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	snap, err := Create(dir, claudeDir)
	assert.Error(t, err)
	assert.Nil(t, snap)
	assert.Contains(t, err.Error(), "no .keel.json found")
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Create manifest
	mf := manifest.CompiledManifest{
		Agent:   "claude",
		FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}},
	}
	manifestData, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(manifestData, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("instructions"), 0644))

	// Create multiple snapshots with delays to ensure different timestamps
	var snapshots []*Snapshot
	for i := 0; i < 3; i++ {
		snap, err := Create(dir, claudeDir)
		require.NoError(t, err)
		snapshots = append(snapshots, snap)
		time.Sleep(1 * time.Second) // Ensure different IDs
	}

	// List snapshots
	listed, err := List(dir)
	require.NoError(t, err)
	require.Len(t, listed, 3)

	// Verify newest first
	for i := 0; i < len(listed)-1; i++ {
		assert.True(t, listed[i].CreatedAt.After(listed[i+1].CreatedAt),
			"snapshots should be sorted newest first")
	}

	// Verify all snapshots have correct agent
	for _, snap := range listed {
		assert.Equal(t, "claude", snap.Agent)
	}
}

func TestListNoSnapshots(t *testing.T) {
	dir := t.TempDir()

	snapshots, err := List(dir)
	require.NoError(t, err)
	assert.Nil(t, snapshots)
}

func TestRestore(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Create manifest
	mf := manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: ".claude/settings.json", Strategy: "deep-merge"},
			{Output: "CLAUDE.md", Strategy: "concatenate"},
		},
	}
	manifestData, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(manifestData, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"model":"original"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("original"), 0644))

	// Create snapshot
	snap, err := Create(dir, claudeDir)
	require.NoError(t, err)

	// Modify files
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"model":"modified"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("modified"), 0644))

	// Verify modifications
	settings, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	require.NoError(t, err)
	assert.Contains(t, string(settings), "modified")

	// Restore snapshot
	err = Restore(snap.Dir, dir)
	require.NoError(t, err)

	// Verify files match snapshot
	settings, err = os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	require.NoError(t, err)
	assert.Contains(t, string(settings), "original")

	instructions, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(instructions))

	// Verify manifest restored
	assert.FileExists(t, filepath.Join(claudeDir, ".keel.json"))
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Create manifest
	mf := manifest.CompiledManifest{
		Agent:   "claude",
		FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}},
	}
	manifestData, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(manifestData, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("instructions"), 0644))

	// Create 5 snapshots
	var snapshots []*Snapshot
	for i := 0; i < 5; i++ {
		snap, err := Create(dir, claudeDir)
		require.NoError(t, err)
		snapshots = append(snapshots, snap)
		time.Sleep(1 * time.Second)
	}

	// Prune to keep only 2
	removed, err := Prune(dir, 2)
	require.NoError(t, err)
	require.Len(t, removed, 3)

	// Verify only 2 remain
	listed, err := List(dir)
	require.NoError(t, err)
	require.Len(t, listed, 2)

	// Verify the 2 newest remain
	for _, snap := range listed[:2] {
		assert.DirExists(t, snap.Dir)
	}

	// Verify the 3 oldest were removed
	for _, id := range removed {
		found := false
		for _, snap := range listed {
			if snap.ID == id {
				found = true
				break
			}
		}
		assert.False(t, found, "removed snapshot should not be in list")
	}
}

func TestPruneKeepsAll(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Create manifest
	mf := manifest.CompiledManifest{
		Agent:   "claude",
		FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}},
	}
	manifestData, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(manifestData, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("instructions"), 0644))

	// Create 2 snapshots
	for i := 0; i < 2; i++ {
		_, err := Create(dir, claudeDir)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)
	}

	// Prune to keep 5 (more than exist)
	removed, err := Prune(dir, 5)
	require.NoError(t, err)
	assert.Nil(t, removed)

	// Verify both still exist
	listed, err := List(dir)
	require.NoError(t, err)
	require.Len(t, listed, 2)
}
