package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/drift"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestDetectDrift_OverlayWithUserFiles(t *testing.T) {
	t.Parallel()

	// Create output with 3 Keel files + 2 user files in commands/, modify 1 Keel file.
	// Drift should report only the modified Keel file, not the user files.

	dir := t.TempDir()
	commandsDir := filepath.Join(dir, ".claude", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// Keel-managed files.
	keel1Content := []byte("Keel command 1")
	keel2Content := []byte("Keel command 2")
	keel3Content := []byte("Keel command 3")
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "keel1.md"), keel1Content, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "keel2.md"), keel2Content, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "keel3.md"), keel3Content, 0644))

	// User-created files (not in manifest).
	userContent := "User command"
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "user1.md"), []byte(userContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "user2.md"), []byte(userContent), 0644))

	// Create manifest tracking only Keel files with per-file records using correct hashes.
	cm := &manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: ".claude/commands/keel1.md", Strategy: "overlay", Hash: manifest.HashContent(keel1Content)},
			{Output: ".claude/commands/keel2.md", Strategy: "overlay", Hash: manifest.HashContent(keel2Content)},
			{Output: ".claude/commands/keel3.md", Strategy: "overlay", Hash: manifest.HashContent(keel3Content)},
		},
		Overlays: map[string][]string{
			".claude/commands/keel1.md": {"layer-a"},
			".claude/commands/keel2.md": {"layer-a"},
			".claude/commands/keel3.md": {"layer-a"},
		},
	}

	// Delete keel2.md (overlay files are checked by existence only).
	require.NoError(t, os.Remove(filepath.Join(commandsDir, "keel2.md")))

	drift, err := drift.Detect(dir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift(), "should detect drift when Keel file is deleted")

	// keel2.md should be reported as missing.
	assert.Len(t, drift.Missing, 1, "should report 1 missing file")
	assert.Equal(t, ".claude/commands/keel2.md", drift.Missing[0].Path)

	// User files should not be in drift report.
	for _, entry := range drift.Missing {
		assert.NotContains(t, entry.Path, "user1.md", "user files should not be in drift report")
		assert.NotContains(t, entry.Path, "user2.md", "user files should not be in drift report")
	}
	for _, entry := range drift.Modified {
		assert.NotContains(t, entry.Path, "user1.md", "user files should not be in drift report")
		assert.NotContains(t, entry.Path, "user2.md", "user files should not be in drift report")
	}
}

func TestDetectDrift_MissingOverlayFile(t *testing.T) {
	t.Parallel()

	// Delete one overlay file from commands/ — drift should report it as missing.

	dir := t.TempDir()
	commandsDir := filepath.Join(dir, ".claude", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// Create files.
	cmd1Content := []byte("Command 1")
	cmd2Content := []byte("Command 2")
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "cmd1.md"), cmd1Content, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "cmd2.md"), cmd2Content, 0644))

	// Create manifest with per-file records using correct hashes.
	cm := &manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: ".claude/commands/cmd1.md", Strategy: "overlay", Hash: manifest.HashContent(cmd1Content)},
			{Output: ".claude/commands/cmd2.md", Strategy: "overlay", Hash: manifest.HashContent(cmd2Content)},
		},
		Overlays: map[string][]string{
			".claude/commands/cmd1.md": {"layer-a"},
			".claude/commands/cmd2.md": {"layer-a"},
		},
	}

	// Delete cmd1.md.
	require.NoError(t, os.Remove(filepath.Join(commandsDir, "cmd1.md")))

	drift, err := drift.Detect(dir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift(), "should detect drift")

	assert.Len(t, drift.Missing, 1, "should report 1 missing file")
	assert.Equal(t, ".claude/commands/cmd1.md", drift.Missing[0].Path)
	assert.Len(t, drift.Modified, 0, "should report 0 modified files")
}

func TestDetectDrift_NoDrift_WithOverlays(t *testing.T) {
	t.Parallel()

	// Compile, then detect — should report no drift.

	dir := t.TempDir()
	commandsDir := filepath.Join(dir, ".claude", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// Create files.
	cmd1Content := []byte("Command 1")
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "cmd1.md"), cmd1Content, 0644))

	instructionContent := "# Instructions\n\nTest."
	instructionPath := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(instructionPath, []byte(instructionContent), 0644))

	// Create manifest with hash for instruction file and per-file overlay record with correct hash.
	cm := &manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: "CLAUDE.md", Strategy: "user-editable", Hash: manifest.HashContent([]byte(instructionContent))},
			{Output: ".claude/commands/cmd1.md", Strategy: "overlay", Hash: manifest.HashContent(cmd1Content)},
		},
		Overlays: map[string][]string{
			".claude/commands/cmd1.md": {"layer-a"},
		},
	}

	drift, err := drift.Detect(dir, cm)
	require.NoError(t, err)
	assert.False(t, drift.HasDrift(), "should not detect drift")
	assert.Len(t, drift.Missing, 0)
	assert.Len(t, drift.Modified, 0)
}

func TestDetectDrift_HashTrackedFileModified(t *testing.T) {
	t.Parallel()

	// Modify the instruction file (CLAUDE.md) — drift should detect via hash comparison.

	dir := t.TempDir()

	originalContent := "# Instructions\n\nOriginal content."
	instructionPath := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(instructionPath, []byte(originalContent), 0644))

	// Create manifest with hash for instruction file.
	cm := &manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: "CLAUDE.md", Strategy: "user-editable", Hash: manifest.HashContent([]byte(originalContent))},
		},
	}

	// Modify the file.
	modifiedContent := "# Instructions\n\nModified content."
	require.NoError(t, os.WriteFile(instructionPath, []byte(modifiedContent), 0644))

	drift, err := drift.Detect(dir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift(), "should detect drift")

	assert.Len(t, drift.Modified, 1, "should report 1 modified file")
	assert.Equal(t, "CLAUDE.md", drift.Modified[0].Path)
	assert.Equal(t, manifest.HashContent([]byte(originalContent)), drift.Modified[0].Expected)
	assert.Equal(t, manifest.HashContent([]byte(modifiedContent)), drift.Modified[0].Actual)
	assert.Len(t, drift.Missing, 0)
}

func TestDetectDrift_HashTrackedFileMissing(t *testing.T) {
	t.Parallel()

	// Delete a hash-tracked file — drift should report it as missing.

	dir := t.TempDir()

	originalContent := "# Instructions\n\nOriginal."
	instructionPath := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(instructionPath, []byte(originalContent), 0644))

	cm := &manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: "CLAUDE.md", Strategy: "user-editable", Hash: manifest.HashContent([]byte(originalContent))},
		},
	}

	// Delete the file.
	require.NoError(t, os.Remove(instructionPath))

	drift, err := drift.Detect(dir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift(), "should detect drift")

	assert.Len(t, drift.Missing, 1, "should report 1 missing file")
	assert.Equal(t, "CLAUDE.md", drift.Missing[0].Path)
	assert.Equal(t, manifest.HashContent([]byte(originalContent)), drift.Missing[0].Expected)
	assert.Len(t, drift.Modified, 0)
}

func TestDetectDrift_OnlyOverlayParentInFileOps(t *testing.T) {
	t.Parallel()

	// Only check overlays whose parent directory matches a file_ops entry.
	// This prevents false positives from categories that were accumulated but not written.

	dir := t.TempDir()
	commandsDir := filepath.Join(dir, ".claude", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// Create a file in commands/.
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "cmd.md"), []byte("Command"), 0644))

	// Manifest has an overlay entry but NO file_ops entry for .claude/commands/.
	// This simulates a category that was accumulated but not written by the target adapter.
	cm := &manifest.CompiledManifest{
		Agent:   "claude",
		FileOps: []manifest.FileRecord{
			// No entry for .claude/commands/
		},
		Overlays: map[string][]string{
			".claude/commands/cmd.md": {"layer-a"},
		},
	}

	drift, err := drift.Detect(dir, cm)
	require.NoError(t, err)
	// Should not report drift because the overlay parent is not in file_ops.
	assert.False(t, drift.HasDrift(), "should not report drift for overlays without parent in file_ops")
}

func TestDetectDrift_MultipleCategories(t *testing.T) {
	t.Parallel()

	// Test drift detection with multiple overlay categories.

	dir := t.TempDir()
	commandsDir := filepath.Join(dir, ".claude", "commands")
	skillsDir := filepath.Join(dir, ".claude", "skills")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(skillsDir, 0755))

	// Create files.
	cmdContent := []byte("Command")
	skillContent := []byte("Skill")
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "cmd.md"), cmdContent, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "skill.md"), skillContent, 0644))

	cm := &manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: ".claude/commands/cmd.md", Strategy: "overlay", Hash: manifest.HashContent(cmdContent)},
			{Output: ".claude/skills/skill.md", Strategy: "overlay", Hash: manifest.HashContent(skillContent)},
		},
		Overlays: map[string][]string{
			".claude/commands/cmd.md": {"layer-a"},
			".claude/skills/skill.md": {"layer-a"},
		},
	}

	// Delete one file from each category.
	require.NoError(t, os.Remove(filepath.Join(commandsDir, "cmd.md")))
	require.NoError(t, os.Remove(filepath.Join(skillsDir, "skill.md")))

	drift, err := drift.Detect(dir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift(), "should detect drift")

	assert.Len(t, drift.Missing, 2, "should report 2 missing files")
	missingPaths := make(map[string]bool)
	for _, entry := range drift.Missing {
		missingPaths[entry.Path] = true
	}
	assert.True(t, missingPaths[".claude/commands/cmd.md"], "cmd.md should be missing")
	assert.True(t, missingPaths[".claude/skills/skill.md"], "skill.md should be missing")
}

func TestDetectDrift_NoHashForOverlay(t *testing.T) {
	t.Parallel()

	// With per-file records, overlay files now HAVE hashes and content changes ARE detected.
	// This test verifies that overlay files with hashes properly detect content drift.

	dir := t.TempDir()
	commandsDir := filepath.Join(dir, ".claude", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	originalContent := "Original command"
	cmdPath := filepath.Join(commandsDir, "cmd.md")
	require.NoError(t, os.WriteFile(cmdPath, []byte(originalContent), 0644))

	cm := &manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: ".claude/commands/cmd.md", Strategy: "overlay", Hash: manifest.HashContent([]byte(originalContent))},
		},
		Overlays: map[string][]string{
			".claude/commands/cmd.md": {"layer-a"},
		},
	}

	// Modify the file content.
	modifiedContent := "Modified command"
	require.NoError(t, os.WriteFile(cmdPath, []byte(modifiedContent), 0644))

	drift, err := drift.Detect(dir, cm)
	require.NoError(t, err)
	// Content change SHOULD be detected now (overlays have hashes).
	assert.True(t, drift.HasDrift(), "overlay content changes should trigger drift with per-file hashing")
	assert.Len(t, drift.Modified, 1)
	assert.Equal(t, ".claude/commands/cmd.md", drift.Modified[0].Path)
}
