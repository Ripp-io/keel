package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/snapshot"
)

func TestSnapshotCreateCmd(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Create manifest
	mf := manifest.CompiledManifest{
		Agent:   "claude",
		FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}},
	}
	data, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(data, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("instructions"), 0644))

	cmd := newSnapshotCreateCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{dir})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Snapshot created:")
	assert.Contains(t, buf.String(), "(claude)")

	// Verify snapshot was created
	snapshots, err := snapshot.List(dir)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	assert.Equal(t, "claude", snapshots[0].Agent)
}

func TestSnapshotCreateCmd_MultiAgent(t *testing.T) {
	dir := t.TempDir()

	// Create Claude manifest
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	cm := manifest.CompiledManifest{
		Agent:   "claude",
		FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}},
	}
	cd, err := json.MarshalIndent(&cm, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(cd, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("claude"), 0644))

	// Create Cursor manifest
	cursorDir := filepath.Join(dir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))
	crm := manifest.CompiledManifest{
		Agent:   "cursor",
		FileOps: []manifest.FileRecord{{Output: ".cursor/mcp.json", Strategy: "deep-merge"}},
	}
	crd, err := json.MarshalIndent(&crm, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, ".keel.json"), append(crd, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "mcp.json"), []byte("{}"), 0644))

	cmd := newSnapshotCreateCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{dir})
	require.NoError(t, cmd.Execute())

	output := buf.String()
	assert.Contains(t, output, "(claude)")
	assert.Contains(t, output, "(cursor)")

	// Verify snapshots were created
	snapshots, err := snapshot.List(dir)
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
}

func TestSnapshotCreateCmd_NoManifest(t *testing.T) {
	cmd := newSnapshotCreateCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{t.TempDir()})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "No compiled output found")
}

func TestSnapshotListCmd(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Create manifest
	mf := manifest.CompiledManifest{
		Agent:   "claude",
		FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}},
	}
	data, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(data, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("instructions"), 0644))

	// Create snapshots
	for i := 0; i < 2; i++ {
		_, err := snapshot.Create(dir, claudeDir)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)
	}

	cmd := newSnapshotListCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{dir})
	require.NoError(t, cmd.Execute())

	output := buf.String()
	assert.Contains(t, output, "claude")
	// Should have 2 lines (one per snapshot)
	lines := 0
	for _, c := range output {
		if c == '\n' {
			lines++
		}
	}
	assert.Equal(t, 2, lines)
}

func TestSnapshotListCmd_NoSnapshots(t *testing.T) {
	cmd := newSnapshotListCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{t.TempDir()})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "No snapshots found")
}

func TestSnapshotRestoreCmd(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Create manifest
	mf := manifest.CompiledManifest{
		Agent:   "claude",
		FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}},
	}
	data, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(data, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("original"), 0644))

	// Create snapshot
	snap, err := snapshot.Create(dir, claudeDir)
	require.NoError(t, err)

	// Modify file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("modified"), 0644))

	// Restore
	cmd := newSnapshotRestoreCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{snap.ID, dir})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Restored snapshot")
	assert.Contains(t, buf.String(), snap.ID)

	// Verify file restored
	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(content))
}

func TestSnapshotRestoreCmd_NotFound(t *testing.T) {
	cmd := newSnapshotRestoreCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"20260518T143022Z", t.TempDir()})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSnapshotPruneCmd(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Create manifest
	mf := manifest.CompiledManifest{
		Agent:   "claude",
		FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}},
	}
	data, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(data, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("instructions"), 0644))

	// Create 4 snapshots
	for i := 0; i < 4; i++ {
		_, err := snapshot.Create(dir, claudeDir)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)
	}

	cmd := newSnapshotPruneCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--keep", "2", dir})
	require.NoError(t, cmd.Execute())

	output := buf.String()
	assert.Contains(t, output, "Removed snapshot:")
	assert.Contains(t, output, "Pruned 2 snapshots")

	// Verify only 2 remain
	snapshots, err := snapshot.List(dir)
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
}

func TestSnapshotPruneCmd_NoSnapshots(t *testing.T) {
	cmd := newSnapshotPruneCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{t.TempDir()})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "No snapshots to prune")
}
