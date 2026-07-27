package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestForceLock_AlreadyLockedItem(t *testing.T) {
	t.Parallel()

	// Layer A force-locks "mcp/gitlab", Layer B (higher priority) tries to write to "mcp/gitlab".
	// Expected: B's write is silently blocked, A's config survives.

	dir := t.TempDir()

	// Layer A: force-locks gitlab MCP server.
	layerA := filepath.Join(dir, "layer-a")
	require.NoError(t, os.MkdirAll(layerA, 0755))

	keelA := `schema: v1
name: layer-a
mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(keelA), 0644))

	mcpA := `{
  "mcpServers": {
    "gitlab": {
      "command": "layer-a-command",
      "force": true
    }
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "mcp.json"), []byte(mcpA), 0644))

	// Layer B: tries to override gitlab (should be blocked).
	layerB := filepath.Join(dir, "layer-b")
	require.NoError(t, os.MkdirAll(layerB, 0755))

	keelB := `schema: v1
name: layer-b
mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "keel.yml"), []byte(keelB), 0644))

	mcpB := `{
  "mcpServers": {
    "gitlab": {
      "command": "layer-b-command-should-be-blocked"
    },
    "other-server": {
      "command": "layer-b-other"
    }
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "mcp.json"), []byte(mcpB), 0644))

	layers := []config.Layer{
		{Path: layerA, Format: config.FormatKeel, Type: "layer-a"},
		{Path: layerB, Format: config.FormatKeel, Type: "layer-b"},
	}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Verify gitlab server uses layer-a's command (force-locked).
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err)
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp))

	servers, ok := mcp["mcpServers"].(map[string]any)
	require.True(t, ok)

	gitlab, ok := servers["gitlab"].(map[string]any)
	require.True(t, ok, "gitlab server should exist")
	assert.Equal(t, "layer-a-command", gitlab["command"], "gitlab should use layer-a's command (force-locked)")

	// Layer B's other-server should be added (not force-locked).
	otherSrv, ok := servers["other-server"].(map[string]any)
	require.True(t, ok, "other-server from layer-b should exist")
	assert.Equal(t, "layer-b-other", otherSrv["command"])
}

func TestForceLock_LocksPermanentWithinCompilation(t *testing.T) {
	t.Parallel()

	// Verify that force-locks are permanent within a compilation (no unlock mechanism).
	// Layer A locks "mcp/server1", Layer B locks "mcp/server2", Layer C tries to override both.
	// Expected: Both locks hold, C's overrides are blocked.

	dir := t.TempDir()

	layerA := filepath.Join(dir, "layer-a")
	require.NoError(t, os.MkdirAll(layerA, 0755))
	keelA := `schema: v1
name: layer-a
mcp:
  servers:
    - source: mcp.json
`
	mcpA := `{"mcpServers": {"server1": {"command": "a", "force": true}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(keelA), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "mcp.json"), []byte(mcpA), 0644))

	layerB := filepath.Join(dir, "layer-b")
	require.NoError(t, os.MkdirAll(layerB, 0755))
	keelB := `schema: v1
name: layer-b
mcp:
  servers:
    - source: mcp.json
`
	mcpB := `{"mcpServers": {"server2": {"command": "b", "force": true}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "keel.yml"), []byte(keelB), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "mcp.json"), []byte(mcpB), 0644))

	layerC := filepath.Join(dir, "layer-c")
	require.NoError(t, os.MkdirAll(layerC, 0755))
	keelC := `schema: v1
name: layer-c
mcp:
  servers:
    - source: mcp.json
`
	mcpC := `{"mcpServers": {"server1": {"command": "c1-blocked"}, "server2": {"command": "c2-blocked"}, "server3": {"command": "c3"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerC, "keel.yml"), []byte(keelC), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerC, "mcp.json"), []byte(mcpC), 0644))

	layers := []config.Layer{
		{Path: layerA, Format: config.FormatKeel, Type: "layer-a"},
		{Path: layerB, Format: config.FormatKeel, Type: "layer-b"},
		{Path: layerC, Format: config.FormatKeel, Type: "layer-c"},
	}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err)
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp))

	servers, ok := mcp["mcpServers"].(map[string]any)
	require.True(t, ok)

	// Both server1 and server2 should retain their original values (force-locked).
	s1, ok := servers["server1"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "a", s1["command"], "server1 should be locked by layer-a")

	s2, ok := servers["server2"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "b", s2["command"], "server2 should be locked by layer-b")

	// server3 should be added from layer-c (not locked).
	s3, ok := servers["server3"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "c3", s3["command"])
}

func TestForceLock_FilePolicy_OwnLayerNotLocked(t *testing.T) {
	t.Parallel()

	// Verify that a layer's own files are written BEFORE its force-locks take effect
	// (so the layer doesn't lock itself out).

	dir := t.TempDir()

	layerA := filepath.Join(dir, "layer-a")
	require.NoError(t, os.MkdirAll(filepath.Join(layerA, "commands"), 0755))

	keelA := `schema: v1
name: layer-a
commands:
  - source: commands/deploy.md
files:
  "commands/deploy.md":
    force: true
`
	deployContent := "# Deploy\n\nLayer A's deployment command."
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(keelA), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "commands", "deploy.md"), []byte(deployContent), 0644))

	layers := []config.Layer{
		{Path: layerA, Format: config.FormatKeel, Type: "layer-a"},
	}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// deploy.md should exist with layer-a's content (not locked out).
	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "deploy.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Layer A's deployment command", "layer-a's deploy.md should be written")

	// Now add layer B that tries to override deploy.md (should be blocked).
	layerB := filepath.Join(dir, "layer-b")
	require.NoError(t, os.MkdirAll(filepath.Join(layerB, "commands"), 0755))

	keelB := `schema: v1
name: layer-b
commands:
  - source: commands/deploy.md
`
	deployB := "# Deploy\n\nLayer B's override (should be blocked)."
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "keel.yml"), []byte(keelB), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "commands", "deploy.md"), []byte(deployB), 0644))

	layersAB := []config.Layer{
		{Path: layerA, Format: config.FormatKeel, Type: "layer-a"},
		{Path: layerB, Format: config.FormatKeel, Type: "layer-b"},
	}

	outputDir2 := t.TempDir()
	_, err = Compile("claude", layersAB, outputDir2, CompileOptions{})
	require.NoError(t, err)

	// deploy.md should still have layer-a's content (force-locked).
	content2, err := os.ReadFile(filepath.Join(outputDir2, ".claude", "commands", "deploy.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content2), "Layer A's deployment command", "deploy.md should be force-locked")
	assert.NotContains(t, string(content2), "Layer B's override", "layer-b should not override force-locked file")
}

func TestForceLock_IsCategoryLocked(t *testing.T) {
	t.Parallel()

	// Test the isCategoryLocked method.
	r := newForceRegistry()

	// Initially, no category is locked.
	locked, _ := r.isCategoryLocked("settings")
	assert.False(t, locked, "settings should not be locked initially")

	// Lock a key in the settings category.
	r.lock("settings", "model", "layer-a")

	locked, source := r.isCategoryLocked("settings")
	assert.True(t, locked, "settings category should be locked")
	assert.Contains(t, source, "layer-a", "source should identify the locking layer")
	assert.Contains(t, source, "model", "source should identify the locked key")

	// Other categories should not be locked.
	locked, _ = r.isCategoryLocked("mcp")
	assert.False(t, locked, "mcp category should not be locked")

	// Lock another key in a different category.
	r.lock("mcp", "server1", "layer-b")

	locked, source = r.isCategoryLocked("mcp")
	assert.True(t, locked, "mcp category should be locked")
	assert.Contains(t, source, "layer-b")
	assert.Contains(t, source, "server1")
}

func TestForceLock_FilePolicyRoundTrip(t *testing.T) {
	t.Parallel()

	// Test that file policies with force: true are registered and recorded in manifest.

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(filepath.Join(layerDir, "commands"), 0755))

	keel := `schema: v1
name: layer
commands:
  - source: commands/
files:
  "commands/deploy.md":
    force: true
  "commands/build.md":
    force: false
`
	deployContent := "Deploy command"
	buildContent := "Build command"
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "commands", "deploy.md"), []byte(deployContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "commands", "build.md"), []byte(buildContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	outputDir := t.TempDir()
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Verify force-lock manifest entry for deploy.md (but not build.md).
	foundDeploy := false
	for _, fl := range result.Governance.ForceLocked {
		if fl.Category == "command" && fl.Key == "deploy.md" {
			foundDeploy = true
			assert.Equal(t, "layer", fl.LockedBy)
		}
		// build.md should NOT be in force-locked list.
		if fl.Key == "build.md" {
			assert.Fail(t, "build.md should not be force-locked (force: false)")
		}
	}
	assert.True(t, foundDeploy, "deploy.md should be in force-locked manifest")

	// Load manifest and verify force_locked section.
	mf, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)
	require.NotNil(t, mf)

	foundInManifest := false
	for _, fl := range mf.ForceLocked {
		if fl.Category == "command" && fl.Key == "deploy.md" {
			foundInManifest = true
		}
	}
	assert.True(t, foundInManifest, "deploy.md force-lock should be in .keel.json manifest")
}

// Helper functions testdataDir, buildTestLayers are defined in testhelpers_test.go.
// manifest.LoadKeelState is the exported function in manifest.go.
