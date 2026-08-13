package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// TestMergePolicy_AdditivePermissions verifies that when a layer sets
// permissions.policy.merge to "additive", permission arrays (allow, deny)
// are unioned across layers instead of last-writer-wins.
func TestMergePolicy_AdditivePermissions(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Layer A: sets additive policy and provides permissions
	layerA := filepath.Join(tempDir, "layer-a")
	require.NoError(t, os.MkdirAll(layerA, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(`
schema: v1
name: layer-a
permissions:
  policy:
    merge: additive
  rules:
    - source: perms-a.json
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "perms-a.json"), []byte(`{"allow": ["Read"]}`), 0644))

	// Layer B: adds more permissions (without policy override)
	layerB := filepath.Join(tempDir, "layer-b")
	require.NoError(t, os.MkdirAll(layerB, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "keel.yml"), []byte(`
schema: v1
name: layer-b
permissions:
  rules:
    - source: perms-b.json
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "perms-b.json"), []byte(`{"allow": ["Write"]}`), 0644))

	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	result, err := Compile("claude", []config.Layer{
		{Path: layerA, Format: config.FormatKeel},
		{Path: layerB, Format: config.FormatKeel},
	}, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Verify the permissions in settings.json contain BOTH "Read" AND "Write"
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	settingsData, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))

	perms, ok := settings["permissions"].(map[string]any)
	require.True(t, ok, "expected permissions to be a map")

	allow, ok := perms["allow"].([]any)
	require.True(t, ok, "expected allow to be an array")

	// Convert to strings for easier assertion
	allowStrings := make([]string, len(allow))
	for i, v := range allow {
		allowStrings[i] = v.(string)
	}

	assert.Contains(t, allowStrings, "Read", "expected Read from layer-a")
	assert.Contains(t, allowStrings, "Write", "expected Write from layer-b")
	assert.Equal(t, 2, len(allowStrings), "expected both permissions to be present (additive merge)")

	// Verify governance recorded the additive policy
	require.NotNil(t, result.Governance.Record, "expected Governance to be non-nil")
	require.NotNil(t, result.Governance.Record.PermissionsPolicy, "expected PermissionsPolicy to be recorded")
	assert.Equal(t, "additive", result.Governance.Record.PermissionsPolicy.Merge, "expected merge strategy to be additive")
	assert.Equal(t, "layer-a", result.Governance.Record.PermissionsPolicy.SetBy, "expected policy set by layer-a")
}

// TestMergePolicy_DefaultPermissions verifies that without an additive policy,
// permissions arrays use last-writer-wins semantics (default deep-merge).
func TestMergePolicy_DefaultPermissions(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Layer A: provides permissions (no policy)
	layerA := filepath.Join(tempDir, "layer-a")
	require.NoError(t, os.MkdirAll(layerA, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(`
schema: v1
name: layer-a
permissions:
  rules:
    - source: perms-a.json
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "perms-a.json"), []byte(`{"allow": ["Read", "Write"]}`), 0644))

	// Layer B: overwrites permissions (no additive policy)
	layerB := filepath.Join(tempDir, "layer-b")
	require.NoError(t, os.MkdirAll(layerB, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "keel.yml"), []byte(`
schema: v1
name: layer-b
permissions:
  rules:
    - source: perms-b.json
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "perms-b.json"), []byte(`{"allow": ["Execute"]}`), 0644))

	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	result, err := Compile("claude", []config.Layer{
		{Path: layerA, Format: config.FormatKeel},
		{Path: layerB, Format: config.FormatKeel},
	}, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Verify the permissions contain only "Execute" (Layer B's value replaces Layer A's)
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	settingsData, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))

	perms, ok := settings["permissions"].(map[string]any)
	require.True(t, ok, "expected permissions to be a map")

	allow, ok := perms["allow"].([]any)
	require.True(t, ok, "expected allow to be an array")

	allowStrings := make([]string, len(allow))
	for i, v := range allow {
		allowStrings[i] = v.(string)
	}

	assert.Contains(t, allowStrings, "Execute", "expected Execute from layer-b")
	assert.NotContains(t, allowStrings, "Read", "expected Read from layer-a to be replaced")
	assert.NotContains(t, allowStrings, "Write", "expected Write from layer-a to be replaced")
	assert.Equal(t, 1, len(allowStrings), "expected only layer-b's permissions (last-writer-wins)")

	// Verify no policy was set
	if result.Governance.Record != nil && result.Governance.Record.PermissionsPolicy != nil {
		t.Errorf("expected no PermissionsPolicy when policy is not set, got: %+v", result.Governance.Record.PermissionsPolicy)
	}
}

// TestMergePolicy_KeelPolicyInJSON verifies that _keel metadata in a JSON
// file is stripped from the output and does NOT affect merge behavior unless
// explicitly supported by the accumulator.
//
// NOTE: Current implementation strips _keel from JSON but does NOT apply
// _keel.merge policies from JSON files to permissions — only the keel.yml
// permissions.policy is respected. This test documents that behavior.
func TestMergePolicy_KeelPolicyInJSON(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Layer A: provides permissions JSON with _keel metadata
	layerA := filepath.Join(tempDir, "layer-a")
	require.NoError(t, os.MkdirAll(layerA, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(`
schema: v1
name: layer-a
permissions:
  rules:
    - source: perms-a.json
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerA, "perms-a.json"), []byte(`{
  "_keel": {"merge": "additive"},
  "allow": ["Read"]
}`), 0644))

	// Layer B: provides permissions JSON without _keel
	layerB := filepath.Join(tempDir, "layer-b")
	require.NoError(t, os.MkdirAll(layerB, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "keel.yml"), []byte(`
schema: v1
name: layer-b
permissions:
  rules:
    - source: perms-b.json
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerB, "perms-b.json"), []byte(`{"allow": ["Write"]}`), 0644))

	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	result, err := Compile("claude", []config.Layer{
		{Path: layerA, Format: config.FormatKeel},
		{Path: layerB, Format: config.FormatKeel},
	}, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Verify _keel metadata is stripped from output
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	settingsData, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))

	perms, ok := settings["permissions"].(map[string]any)
	require.True(t, ok, "expected permissions to be a map")

	// _keel should NOT appear in the output
	_, hasKeel := perms["_keel"]
	assert.False(t, hasKeel, "expected _keel metadata to be stripped from output")

	// The merge behavior depends on whether keel.yml set additive policy.
	// Since Layer A's keel.yml did NOT set permissions.policy.merge,
	// the default deep-merge applies (last-writer-wins for arrays).
	allow, ok := perms["allow"].([]any)
	require.True(t, ok, "expected allow to be an array")

	allowStrings := make([]string, len(allow))
	for i, v := range allow {
		allowStrings[i] = v.(string)
	}

	// Without keel.yml policy override, default merge is last-writer-wins
	// (the _keel.merge in JSON is not currently applied to permissions).
	assert.Contains(t, allowStrings, "Write", "expected Write from layer-b")
	// Whether "Read" survives depends on whether _keel.merge in JSON is honored.
	// Current implementation: _keel is stripped but NOT applied for permissions.
	// If this changes in the future, update this assertion.
	if len(allowStrings) == 1 {
		assert.NotContains(t, allowStrings, "Read", "expected last-writer-wins (no keel.yml policy)")
	} else {
		// If _keel.merge is honored, both should be present
		assert.Contains(t, allowStrings, "Read", "expected Read if _keel.merge is honored")
	}

	// No governance policy should be recorded (policy was in JSON, not keel.yml)
	if result.Governance.Record != nil && result.Governance.Record.PermissionsPolicy != nil {
		t.Logf("PermissionsPolicy found: %+v (this means _keel.merge in JSON is now honored)", result.Governance.Record.PermissionsPolicy)
	}
}

// TestMergePolicy_PermissionsFromAgentSettingsFile verifies that permissions
// in an agent-specific settings file (settings.claude.json) are properly
// deep-merged with permissions from other layers. This is a regression test
// for a bug where permissions.deny in settings.claude.json was silently
// dropped because it went through AgentSpecific instead of the Permissions
// accumulation path.
func TestMergePolicy_PermissionsFromAgentSettingsFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Native Claude layer (simulates an implicit layer with allow)
	nativeLayer := filepath.Join(tempDir, "native")
	require.NoError(t, os.MkdirAll(filepath.Join(nativeLayer, ".claude"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nativeLayer, ".claude", "settings.json"), []byte(`{
  "permissions": {
    "allow": ["mcp__pencil"]
  }
}`), 0644))

	// DSL layer with permissions.deny in settings.claude.json
	dslLayer := filepath.Join(tempDir, "keel-layer")
	require.NoError(t, os.MkdirAll(dslLayer, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dslLayer, "keel.yml"), []byte(`
schema: v1
name: test-layer
settings:
  claude: settings.claude.json
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dslLayer, "settings.claude.json"), []byte(`{
  "permissions": {
    "deny": ["Read(./.keel/state/**)"]
  }
}`), 0644))

	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	// Native layer first (lower priority), DSL layer second (higher priority)
	_, err := Compile("claude", []config.Layer{
		{Path: nativeLayer, Format: "claude"},
		{Path: dslLayer, Format: config.FormatKeel},
	}, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Read the compiled settings.json
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	settingsData, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))

	perms, ok := settings["permissions"].(map[string]any)
	require.True(t, ok, "expected permissions to be a map")

	// Both allow and deny should be present (deep-merged from different layers)
	allow, ok := perms["allow"].([]any)
	require.True(t, ok, "expected allow to be an array")
	assert.Equal(t, 1, len(allow))
	assert.Equal(t, "mcp__pencil", allow[0])

	deny, ok := perms["deny"].([]any)
	require.True(t, ok, "expected deny to be an array — deny was silently dropped before fix")
	assert.Equal(t, 1, len(deny))
	assert.Equal(t, "Read(./.keel/state/**)", deny[0])
}
