package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestDriftCmd_NoDrift(t *testing.T) {
	dir := t.TempDir()

	// Create and compile a layer.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("drift test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Check drift — should be clean.
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{dir})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No drift detected")
}

func TestDriftCmd_DriftDetected(t *testing.T) {
	dir := t.TempDir()

	// Create and compile a layer.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("drift test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Modify CLAUDE.md.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("modified"), 0644))

	// Check drift — should detect.
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{dir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift detected")
	assert.Contains(t, buf.String(), "modified")
	assert.Contains(t, buf.String(), "CLAUDE.md")
}

func TestDriftCmd_NoManifest(t *testing.T) {
	dir := t.TempDir()

	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{dir})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No compiled output found")
}

func TestDriftCmd_Policy_NoDrift(t *testing.T) {
	dir := t.TempDir()

	// Create a layer with force-locked instructions via files: map.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(`schema: v1
instructions:
  - source: instructions.md
files:
  "CLAUDE.md":
    force: true
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("policy test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Check policy drift — should be clean.
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--policy", dir})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No policy drift detected")
}

func TestDriftCmd_Policy_ViolationDetected(t *testing.T) {
	dir := t.TempDir()

	// Create a layer with force-locked instructions via files: map.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(`schema: v1
instructions:
  - source: instructions.md
files:
  "CLAUDE.md":
    force: true
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("policy test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Modify the force-locked CLAUDE.md.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("modified"), 0644))

	// Check policy drift — should detect violation.
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--policy", dir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift detected")
	output := buf.String()
	assert.Contains(t, output, "[policy]")
	assert.Contains(t, output, "modified")
	assert.Contains(t, output, "CLAUDE.md")
	assert.Contains(t, output, "Policy summary:")
	assert.Contains(t, output, "Force-locked files: 1")
	assert.Contains(t, output, "Policy violations: 1")
}

func TestDriftCmd_Policy_FilterNonForceLocked(t *testing.T) {
	dir := t.TempDir()

	// Create a layer with both force-locked and non-force-locked content.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(`schema: v1
instructions:
  - source: instructions.md
files:
  "CLAUDE.md":
    force: true
settings:
  model: claude-sonnet-4-20250514
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("policy test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Modify only the non-force-locked settings.json.
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(settingsPath, append(data, []byte("\n")...), 0644))

	// Check policy drift — should report no policy drift (settings.json is not force-locked).
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--policy", dir})

	err = cmd.Execute()
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "No policy drift detected")
	assert.NotContains(t, output, "settings.json")
}

// TestRenderSemanticDiff_EmptyDiff tests that renderSemanticDiff handles an empty diff gracefully.
func TestRenderSemanticDiff_EmptyDiff(t *testing.T) {
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Nil diff should be a no-op.
	renderSemanticDiff(cmd, nil)
	assert.Empty(t, buf.String())

	// Empty diff with no changes should also produce no output.
	emptyDiff := &manifest.EffectiveStateDiff{}
	renderSemanticDiff(cmd, emptyDiff)
	assert.Empty(t, buf.String())
}

// TestRenderSemanticDiff_PermissionsChanges tests permission change rendering.
func TestRenderSemanticDiff_PermissionsChanges(t *testing.T) {
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	diff := &manifest.EffectiveStateDiff{
		Permissions: &manifest.PermissionsDiff{
			AllowAdded:   []string{"Read", "Write"},
			AllowRemoved: []string{"Bash"},
			DenyAdded:    []string{"Delete"},
			DenyRemoved:  []string{"Admin"},
		},
	}

	renderSemanticDiff(cmd, diff)
	output := buf.String()

	assert.Contains(t, output, "permissions:")
	assert.Contains(t, output, "+ Read (added to allow)")
	assert.Contains(t, output, "+ Write (added to allow)")
	assert.Contains(t, output, "- Bash (removed from allow)")
	assert.Contains(t, output, "+ Delete (added to deny)")
	assert.Contains(t, output, "- Admin (removed from deny)")
}

// TestRenderSemanticDiff_MCPServerChanges tests MCP server change rendering.
func TestRenderSemanticDiff_MCPServerChanges(t *testing.T) {
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	diff := &manifest.EffectiveStateDiff{
		MCPServers: &manifest.ListDiff{
			Added:   []string{"gitlab", "jira"},
			Removed: []string{"legacy-server"},
		},
	}

	renderSemanticDiff(cmd, diff)
	output := buf.String()

	assert.Contains(t, output, "mcp_servers:")
	assert.Contains(t, output, "+ gitlab (added)")
	assert.Contains(t, output, "+ jira (added)")
	assert.Contains(t, output, "- legacy-server (removed)")
}

// TestRenderSemanticDiff_HookChanges tests hook change rendering.
func TestRenderSemanticDiff_HookChanges(t *testing.T) {
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	diff := &manifest.EffectiveStateDiff{
		Hooks: &manifest.ListDiff{
			Added:   []string{"PreToolUse", "PostToolUse"},
			Removed: []string{"Deprecated"},
		},
	}

	renderSemanticDiff(cmd, diff)
	output := buf.String()

	assert.Contains(t, output, "hooks:")
	assert.Contains(t, output, "+ PreToolUse (added)")
	assert.Contains(t, output, "+ PostToolUse (added)")
	assert.Contains(t, output, "- Deprecated (removed)")
}

// TestRenderSemanticDiff_SettingsChanges tests settings change rendering.
func TestRenderSemanticDiff_SettingsChanges(t *testing.T) {
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	diff := &manifest.EffectiveStateDiff{
		Settings: []manifest.SettingChange{
			{Key: "model", From: "opus", To: "sonnet"},
			{Key: "new_setting", From: nil, To: "added"},
			{Key: "removed_setting", From: "removed", To: nil},
		},
	}

	renderSemanticDiff(cmd, diff)
	output := buf.String()

	assert.Contains(t, output, "settings:")
	assert.Contains(t, output, "~ model: opus → sonnet")
	assert.Contains(t, output, "+ new_setting: added (added)")
	assert.Contains(t, output, "- removed_setting: removed (removed)")
}

// TestRenderSemanticDiff_InstructionsChanges tests instructions delta rendering.
func TestRenderSemanticDiff_InstructionsChanges(t *testing.T) {
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Test positive deltas.
	diff := &manifest.EffectiveStateDiff{
		Instructions: &manifest.InstructionsDiff{
			TokenDelta: 150,
			ByteDelta:  500,
		},
	}

	renderSemanticDiff(cmd, diff)
	output := buf.String()

	assert.Contains(t, output, "instructions:")
	assert.Contains(t, output, "+150 tokens")
	assert.Contains(t, output, "+500 bytes")

	// Test negative deltas.
	buf.Reset()
	diff.Instructions.TokenDelta = -75
	diff.Instructions.ByteDelta = -250

	renderSemanticDiff(cmd, diff)
	output = buf.String()

	assert.Contains(t, output, "-75 tokens")
	assert.Contains(t, output, "-250 bytes")
}

// TestRenderSemanticDiff_CategoryChanges tests category file count delta rendering.
func TestRenderSemanticDiff_CategoryChanges(t *testing.T) {
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	diff := &manifest.EffectiveStateDiff{
		Categories: []manifest.CategoryDiff{
			{Name: "commands", CountDelta: 3},
			{Name: "agents", CountDelta: -1},
		},
	}

	renderSemanticDiff(cmd, diff)
	output := buf.String()

	assert.Contains(t, output, "categories:")
	assert.Contains(t, output, "commands: +3 files")
	assert.Contains(t, output, "agents: -1 files")
}

// TestRenderSemanticDiff_MixedChanges tests rendering when multiple change types are present.
func TestRenderSemanticDiff_MixedChanges(t *testing.T) {
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	diff := &manifest.EffectiveStateDiff{
		Permissions: &manifest.PermissionsDiff{
			AllowAdded: []string{"Read"},
		},
		MCPServers: &manifest.ListDiff{
			Added: []string{"gitlab"},
		},
		Hooks: &manifest.ListDiff{
			Removed: []string{"OldHook"},
		},
		Settings: []manifest.SettingChange{
			{Key: "model", From: "opus", To: "sonnet"},
		},
		Instructions: &manifest.InstructionsDiff{
			TokenDelta: 100,
			ByteDelta:  300,
		},
		Categories: []manifest.CategoryDiff{
			{Name: "commands", CountDelta: 2},
		},
	}

	renderSemanticDiff(cmd, diff)
	output := buf.String()

	// Verify all sections are present.
	assert.Contains(t, output, "permissions:")
	assert.Contains(t, output, "+ Read (added to allow)")
	assert.Contains(t, output, "mcp_servers:")
	assert.Contains(t, output, "+ gitlab (added)")
	assert.Contains(t, output, "hooks:")
	assert.Contains(t, output, "- OldHook (removed)")
	assert.Contains(t, output, "settings:")
	assert.Contains(t, output, "~ model: opus → sonnet")
	assert.Contains(t, output, "instructions:")
	assert.Contains(t, output, "+100 tokens")
	assert.Contains(t, output, "+300 bytes")
	assert.Contains(t, output, "categories:")
	assert.Contains(t, output, "commands: +2 files")
}

// TestDriftCmd_CI_ImpliesJSON tests that --ci flag implies JSON output.
func TestDriftCmd_CI_ImpliesJSON(t *testing.T) {
	dir := t.TempDir()

	// Create and compile a layer.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("drift test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Modify CLAUDE.md to create drift.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("modified"), 0644))

	// Check drift with --ci flag.
	cmd := newDriftCmd()
	var bufOut bytes.Buffer
	var bufErr bytes.Buffer
	cmd.SetOut(&bufOut)
	cmd.SetErr(&bufErr)
	cmd.SetArgs([]string{"--ci", dir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift detected")

	// Output should be valid JSON (only stdout, not stderr).
	output := bufOut.String()
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")

	// Should not contain text-mode decorations.
	assert.NotContains(t, output, "modified  CLAUDE.md")
	assert.NotContains(t, output, "Drift detected")
}

// TestDriftCmd_CI_ExplicitFormatWins tests that explicit --format overrides --ci.
func TestDriftCmd_CI_ExplicitFormatWins(t *testing.T) {
	dir := t.TempDir()

	// Create and compile a layer.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("drift test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Modify CLAUDE.md to create drift.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("modified"), 0644))

	// Check drift with --ci and --format text.
	cmd := newDriftCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--ci", "--format", "text", dir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift detected")

	// Output should be text format (explicit --format wins).
	output := buf.String()
	assert.Contains(t, output, "modified")
	assert.Contains(t, output, "CLAUDE.md")
}

// TestDriftCmd_CI_WithPolicy tests --ci flag with --policy.
func TestDriftCmd_CI_WithPolicy(t *testing.T) {
	dir := t.TempDir()

	// Create a layer with force-locked instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(`schema: v1
instructions:
  - source: instructions.md
files:
  "CLAUDE.md":
    force: true
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("policy test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Modify the force-locked CLAUDE.md.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("modified"), 0644))

	// Check policy drift with --ci flag.
	cmd := newDriftCmd()
	var bufOut bytes.Buffer
	var bufErr bytes.Buffer
	cmd.SetOut(&bufOut)
	cmd.SetErr(&bufErr)
	cmd.SetArgs([]string{"--ci", "--policy", dir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift detected")

	// Output should be valid JSON (only stdout).
	output := bufOut.String()
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")

	// Should not contain text-mode policy summary.
	assert.NotContains(t, output, "Policy summary:")
}

// TestDriftCmd_SARIF_Format tests SARIF output format.
func TestDriftCmd_SARIF_Format(t *testing.T) {
	dir := t.TempDir()

	// Create and compile a layer.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("drift test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Modify CLAUDE.md to create drift.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("modified"), 0644))

	// Check drift with --format sarif.
	cmd := newDriftCmd()
	var bufOut bytes.Buffer
	var bufErr bytes.Buffer
	cmd.SetOut(&bufOut)
	cmd.SetErr(&bufErr)
	cmd.SetArgs([]string{"--format", "sarif", dir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift detected")

	// Output should be valid SARIF JSON (only stdout).
	output := bufOut.String()
	var sarif map[string]interface{}
	err = json.Unmarshal([]byte(output), &sarif)
	require.NoError(t, err, "Output should be valid JSON")

	// Verify SARIF structure.
	assert.Equal(t, "https://json.schemastore.org/sarif-2.1.0.json", sarif["$schema"])
	assert.Equal(t, "2.1.0", sarif["version"])

	runs := sarif["runs"].([]interface{})
	require.Len(t, runs, 1)

	run := runs[0].(map[string]interface{})
	tool := run["tool"].(map[string]interface{})
	driver := tool["driver"].(map[string]interface{})
	assert.Equal(t, "keel", driver["name"])

	// Check rules are defined.
	rules := driver["rules"].([]interface{})
	require.Len(t, rules, 3)

	// Check results.
	results := run["results"].([]interface{})
	require.NotEmpty(t, results)

	// Find a result for the modified file.
	var foundModified bool
	for _, r := range results {
		result := r.(map[string]interface{})
		if result["ruleId"] == "keel/drift/modified" {
			foundModified = true
			assert.Equal(t, "warning", result["level"])
			locations := result["locations"].([]interface{})
			require.Len(t, locations, 1)
			loc := locations[0].(map[string]interface{})
			physicalLocation := loc["physicalLocation"].(map[string]interface{})
			artifactLocation := physicalLocation["artifactLocation"].(map[string]interface{})
			assert.Equal(t, "CLAUDE.md", artifactLocation["uri"])
		}
	}
	assert.True(t, foundModified, "Should have found a modified file result")
}

// TestDriftCmd_SARIF_WithPolicy tests SARIF output with --policy flag.
func TestDriftCmd_SARIF_WithPolicy(t *testing.T) {
	dir := t.TempDir()

	// Create a layer with force-locked instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(`schema: v1
instructions:
  - source: instructions.md
files:
  "CLAUDE.md":
    force: true
settings:
  model: claude-sonnet-4-20250514
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("policy test"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Modify both force-locked CLAUDE.md and non-force-locked settings.json.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("modified"), 0644))
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(settingsPath, append(data, []byte("\n")...), 0644))

	// Check policy drift with --format sarif --policy.
	cmd := newDriftCmd()
	var bufOut bytes.Buffer
	var bufErr bytes.Buffer
	cmd.SetOut(&bufOut)
	cmd.SetErr(&bufErr)
	cmd.SetArgs([]string{"--format", "sarif", "--policy", dir})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift detected")

	// Output should be valid SARIF JSON (only stdout).
	output := bufOut.String()
	var sarif map[string]interface{}
	err = json.Unmarshal([]byte(output), &sarif)
	require.NoError(t, err, "Output should be valid JSON")

	// Check results - should only have policy violations, not settings.json.
	runs := sarif["runs"].([]interface{})
	require.Len(t, runs, 1)
	run := runs[0].(map[string]interface{})
	results := run["results"].([]interface{})
	require.NotEmpty(t, results)

	// Should have policy violation for CLAUDE.md, but not settings.json.
	var foundPolicyViolation bool
	var foundSettingsJson bool
	for _, r := range results {
		result := r.(map[string]interface{})
		if result["ruleId"] == "keel/drift/policy-violation" {
			foundPolicyViolation = true
			assert.Equal(t, "error", result["level"])
			locations := result["locations"].([]interface{})
			require.Len(t, locations, 1)
			loc := locations[0].(map[string]interface{})
			physicalLocation := loc["physicalLocation"].(map[string]interface{})
			artifactLocation := physicalLocation["artifactLocation"].(map[string]interface{})
			uri := artifactLocation["uri"].(string)
			if uri == "settings.json" || uri == ".claude/settings.json" {
				foundSettingsJson = true
			}
		}
	}
	assert.True(t, foundPolicyViolation, "Should have found a policy violation")
	assert.False(t, foundSettingsJson, "Should not have settings.json in policy violations")
}
