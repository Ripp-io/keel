package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

func TestInstructionConflict_FirstCompileWithExistingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a temp dir with a CLAUDE.md containing "user content"
	userContent := "# User content\n\nThis was here before keel."
	testutil.WriteFile(t, tmpDir, "CLAUDE.md", []byte(userContent))

	// Create a keel layer with instructions
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
instructions:
  - source: instructions.md
`))
	testutil.WriteFile(t, keelLayerDir, "instructions.md", []byte("# Keel Layer Instructions\n\nThis is from keel."))

	// First compile WITHOUT --force — existing CLAUDE.md should be preserved
	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify: CLAUDE.md was NOT overwritten (conflict detected)
	claudeContent, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, userContent, string(claudeContent), "user content should be preserved")

	// Verify: conflict was reported
	assert.NotEmpty(t, result.Governance.InstructionConflicts)
	assert.Contains(t, result.Governance.InstructionConflicts, "CLAUDE.md")

	// Compile again WITH ForceInstructions=true — should overwrite
	result2, err := Compile("claude", layers, tmpDir, CompileOptions{ForceInstructions: true})
	require.NoError(t, err)
	require.NotNil(t, result2)

	// Verify: CLAUDE.md now has compiled content
	claudeContent2, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(claudeContent2), "Keel Layer Instructions")
	assert.Empty(t, result2.Governance.InstructionConflicts)
}

func TestInstructionConflict_RecompileAfterUserEdit(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create keel layer
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
instructions:
  - source: instructions.md
`))
	testutil.WriteFile(t, keelLayerDir, "instructions.md", []byte("# Keel instructions"))

	// First compile (no pre-existing CLAUDE.md)
	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result1, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result1)

	// CLAUDE.md now exists with keel-compiled content
	claudeContent1, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(claudeContent1), "Keel instructions")

	// Recompile — should overwrite silently (disk hash matches manifest hash)
	result2, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	assert.Empty(t, result2.Governance.InstructionConflicts, "no conflict when file unchanged")

	// Edit CLAUDE.md on disk (simulate user edit)
	userEdit := "# User edited content\n\nI changed this."
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(userEdit), 0644))

	// Recompile WITHOUT --force — should skip with conflict
	result3, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, result3.Governance.InstructionConflicts, "conflict detected after user edit")
	assert.Contains(t, result3.Governance.InstructionConflicts, "CLAUDE.md")

	// Verify file was NOT overwritten
	claudeContent3, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, userEdit, string(claudeContent3), "user edit should be preserved")

	// Recompile WITH force — should overwrite
	result4, err := Compile("claude", layers, tmpDir, CompileOptions{ForceInstructions: true})
	require.NoError(t, err)
	assert.Empty(t, result4.Governance.InstructionConflicts, "no conflict when force is set")

	// Verify file WAS overwritten
	claudeContent4, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(claudeContent4), "Keel instructions")
	assert.NotContains(t, string(claudeContent4), "User edited content")
}

func TestInstructionConflict_CleanLeavesFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create keel layer
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
instructions:
  - source: instructions.md
`))
	testutil.WriteFile(t, keelLayerDir, "instructions.md", []byte("# Keel instructions"))

	// Compile with instructions → CLAUDE.md exists
	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, testutil.FileExists(filepath.Join(tmpDir, "CLAUDE.md")))

	// Clean WITHOUT force → CLAUDE.md still exists
	cleanResult, err := manifest.CleanAllOutput(tmpDir, ".claude", false)
	require.NoError(t, err)
	// cleanResult may be nil if no manifest exists (shouldn't happen but be defensive)
	assert.True(t, testutil.FileExists(filepath.Join(tmpDir, "CLAUDE.md")), "user-editable file should remain")

	// Verify CLAUDE.md is NOT in the removed list (if cleanResult is not nil)
	if cleanResult != nil {
		for _, removed := range cleanResult.Removed {
			assert.NotEqual(t, "CLAUDE.md", removed, "CLAUDE.md should not be removed")
		}
	}

	// Recompile with --force to get a fresh manifest (CLAUDE.md exists without manifest)
	result2, err := Compile("claude", layers, tmpDir, CompileOptions{ForceInstructions: true})
	require.NoError(t, err)
	require.NotNil(t, result2)

	// Now clean WITH force → CLAUDE.md removed
	_, err = manifest.CleanAllOutput(tmpDir, ".claude", true)
	require.NoError(t, err)
	assert.False(t, testutil.FileExists(filepath.Join(tmpDir, "CLAUDE.md")), "file should be removed with force")
}

func TestInstructionConflict_ImplicitLayerDoesNotMove(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Setup pre-existing CLAUDE.md + settings.json
	nativeInstructions := "# Native instructions\n\nThis is native."
	testutil.WriteFile(t, tmpDir, "CLAUDE.md", []byte(nativeInstructions))
	testutil.WriteFile(t, tmpDir, ".claude/settings.json", []byte(`{"model": "claude-sonnet"}`))

	// Create keel layer
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
instructions:
  - source: instructions.md
`))
	testutil.WriteFile(t, keelLayerDir, "instructions.md", []byte("# Keel instructions"))

	// First compile
	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify: CLAUDE.md is NOT in the implicit layer (user-owned, never moved)
	assert.False(t, testutil.FileExists(filepath.Join(tmpDir, ".keel/.claude/CLAUDE.md")), "CLAUDE.md should NOT be moved (user-owned)")
}

func TestInstructionConflict_Cursor(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	instrOutput := ".cursor/rules/00-keel.mdc"

	// Create keel layer with instructions
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
instructions:
  - source: instructions.md
`))
	testutil.WriteFile(t, keelLayerDir, "instructions.md", []byte("# Keel instructions for Cursor"))

	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}

	// First compile — no pre-existing file, should write normally
	result1, err := Compile("cursor", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	assert.Empty(t, result1.Governance.InstructionConflicts)

	instrPath := filepath.Join(tmpDir, instrOutput)
	assert.True(t, testutil.FileExists(instrPath), "instruction file should be created")

	// Verify content is MDC-wrapped
	content1, err := os.ReadFile(instrPath)
	require.NoError(t, err)
	assert.Contains(t, string(content1), "Keel instructions for Cursor")

	// Recompile unchanged — should overwrite silently (hash matches)
	result2, err := Compile("cursor", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	assert.Empty(t, result2.Governance.InstructionConflicts, "no conflict when file unchanged")

	// Edit the file (simulate user edit)
	userEdit := "---\ndescription: My custom rules\n---\n\n# User edited rules"
	require.NoError(t, os.WriteFile(instrPath, []byte(userEdit), 0644))

	// Recompile WITHOUT --force — should skip with conflict
	result3, err := Compile("cursor", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, result3.Governance.InstructionConflicts, "conflict detected after user edit")
	assert.Contains(t, result3.Governance.InstructionConflicts, instrOutput)

	// Verify user edit preserved
	content3, err := os.ReadFile(instrPath)
	require.NoError(t, err)
	assert.Equal(t, userEdit, string(content3), "user edit should be preserved")

	// Recompile WITH --force — should overwrite
	result4, err := Compile("cursor", layers, tmpDir, CompileOptions{ForceInstructions: true})
	require.NoError(t, err)
	assert.Empty(t, result4.Governance.InstructionConflicts)

	content4, err := os.ReadFile(instrPath)
	require.NoError(t, err)
	assert.Contains(t, string(content4), "Keel instructions for Cursor")
	assert.NotContains(t, string(content4), "User edited rules")
}
