package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// --- forceRegistry unit tests ---

func TestForceRegistry_LockAndIsLocked(t *testing.T) {
	t.Parallel()
	r := newForceRegistry()
	r.lock("mcp", "sonarqube", "company")

	locked, by := r.isLocked("mcp", "sonarqube")
	assert.True(t, locked, "expected sonarqube to be locked")
	assert.Equal(t, "company", by)
}

func TestForceRegistry_UnlockedItem(t *testing.T) {
	t.Parallel()
	r := newForceRegistry()
	r.lock("mcp", "sonarqube", "company")

	locked, _ := r.isLocked("mcp", "gitlab")
	assert.False(t, locked, "expected gitlab to be unlocked")
}

func TestForceRegistry_DifferentCategories(t *testing.T) {
	t.Parallel()
	r := newForceRegistry()
	r.lock("mcp", "sonarqube", "company")

	// Same key but different category should not be locked
	locked, _ := r.isLocked("command", "sonarqube")
	assert.False(t, locked, "expected different category to be unlocked")
}

func TestForceRegistry_MultipleLocks(t *testing.T) {
	t.Parallel()
	r := newForceRegistry()
	r.lock("mcp", "sonarqube", "company")
	r.lock("command", "deploy.md", "team")
	r.lock("hook", "PreToolUse", "company")

	tests := []struct {
		category string
		key      string
		locked   bool
		by       string
	}{
		{"mcp", "sonarqube", true, "company"},
		{"command", "deploy.md", true, "team"},
		{"hook", "PreToolUse", true, "company"},
		{"mcp", "gitlab", false, ""},
		{"command", "build.md", false, ""},
	}

	for _, tt := range tests {
		locked, by := r.isLocked(tt.category, tt.key)
		assert.Equal(t, tt.locked, locked, "IsLocked(%s, %s) locked", tt.category, tt.key)
		assert.Equal(t, tt.by, by, "IsLocked(%s, %s) by", tt.category, tt.key)
	}
}

func TestForceRegistry_Locks(t *testing.T) {
	t.Parallel()
	r := newForceRegistry()
	r.lock("mcp", "sonarqube", "company")
	r.lock("command", "deploy.md", "team")

	records := r.locks()
	require.Len(t, records, 2)

	// Sort for deterministic comparison (map iteration order is random)
	sort.Slice(records, func(i, j int) bool {
		return records[i].Category+":"+records[i].Key < records[j].Category+":"+records[j].Key
	})

	assert.Equal(t, "command", records[0].Category)
	assert.Equal(t, "deploy.md", records[0].Key)
	assert.Equal(t, "team", records[0].LockedBy)
	assert.Equal(t, "mcp", records[1].Category)
	assert.Equal(t, "sonarqube", records[1].Key)
	assert.Equal(t, "company", records[1].LockedBy)
}

// TestForceRegistry_ConcurrentAccess was removed because forceRegistry is
// intentionally not thread-safe (compilation is single-threaded).

func TestForceRegistry_ColonInKey_NoCollision(t *testing.T) {
	t.Parallel()
	// A key containing ":" must not collide with a different category:key pair.
	r := newForceRegistry()
	r.lock("mcp", "server:name", "layer-a")

	// Different category + key that would collide with the old ":" separator
	locked, _ := r.isLocked("mcp:server", "name")
	assert.False(t, locked, "key with colon should not collide with different category+key split")

	// The original entry should still be retrievable
	locked, by := r.isLocked("mcp", "server:name")
	assert.True(t, locked, "expected mcp/server:name to be locked")
	assert.Equal(t, "layer-a", by)

	// Verify locks() round-trips correctly
	records := r.locks()
	require.Len(t, records, 1)
	assert.Equal(t, "mcp", records[0].Category)
	assert.Equal(t, "server:name", records[0].Key)
}

// --- Pure merge function unit tests ---

func TestCleanContainerForceLocked(t *testing.T) {
	t.Parallel()
	force := newForceRegistry()
	force.lock("mcp", "sonarqube", "company")

	srcMap := map[string]any{
		"mcpServers": map[string]any{
			"sonarqube": map[string]any{
				"command": "npx",
				"args":    []any{"override-sonar"},
			},
			"gitlab": map[string]any{
				"command": "npx",
				"args":    []any{"gitlab-mcp"},
				"force":   true,
			},
		},
	}

	cleaned, policies, err := cleanContainerForceLocked(srcMap, "mcpServers", "mcp", force)
	require.NoError(t, err)

	servers, ok := cleaned["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in cleaned result")

	// sonarqube should be removed (force-locked)
	assert.NotContains(t, servers, "sonarqube",
		"expected force-locked sonarqube to be removed from cleaned result")

	// gitlab should remain but without "force" field
	gl, ok := servers["gitlab"].(map[string]any)
	require.True(t, ok, "expected gitlab to remain")
	assert.NotContains(t, gl, "force", "expected force field to be stripped from gitlab")

	// Policies should include gitlab's force
	glPolicy, ok := policies["mcpServers.gitlab"]
	require.True(t, ok, "expected policy for mcpServers.gitlab")
	require.NotNil(t, glPolicy.Force)
	assert.True(t, *glPolicy.Force)
}

func TestDeepMergeMaps_EmptySrc(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"existing": "value"}
	result := merge.DeepMergeMapsWithPolicies(dst, nil, nil)
	assert.Equal(t, "value", result["existing"],
		"expected dst to be returned unchanged when src is empty")
}

func TestDeepMergeMaps_BasicMerge(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"a": "1"}
	src := map[string]any{"b": "2"}
	result := merge.DeepMergeMapsWithPolicies(dst, src, nil)
	assert.Equal(t, "1", result["a"])
	assert.Equal(t, "2", result["b"])
}

// --- Force-lock MCP integration tests ---

func TestForceLock_MCPServer(t *testing.T) {
	t.Parallel()
	// Company force-locks sonarqube MCP server.
	// Project tries to override sonarqube with different args.
	// Result: company's sonarqube survives, project's gitlab is added.
	layers := buildTestLayers(t, "layers-force", "company", "project")
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "reading .mcp.json")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp))

	servers := mcp["mcpServers"].(map[string]any)

	// sonarqube should use company's config (force-locked)
	sonarqube, ok := servers["sonarqube"]
	require.True(t, ok, "expected sonarqube server to exist")
	sq := sonarqube.(map[string]any)
	args := sq["args"].([]any)
	require.Len(t, args, 1)
	assert.Equal(t, "sonarqube-mcp-server", args[0])

	// gitlab should be added by project (not force-locked)
	assert.Contains(t, servers, "gitlab", "expected gitlab server from project layer")

	// Verify force-lock manifest annotation
	found := false
	for _, fl := range result.Governance.ForceLocked {
		if fl.Category == "mcp" && fl.Key == "sonarqube" && fl.LockedBy == "company" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected force-lock manifest entry for mcp:sonarqube")
}

func TestForceLock_Command(t *testing.T) {
	t.Parallel()
	// Company force-locks security-scan.md command.
	// Project provides a different security-scan.md.
	// Result: company's content survives, project's build.md is added.
	layers := buildTestLayers(t, "layers-force", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// security-scan.md should have company's content
	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "security-scan.md"))
	require.NoError(t, err, "reading security-scan.md")
	assert.Contains(t, string(content), "force-locked, cannot be overridden",
		"expected company's force-locked security-scan.md content")
	assert.NotContains(t, string(content), "should be blocked by force-lock",
		"project's security-scan.md should NOT override force-locked version")

	// build.md should be added by project
	buildPath := filepath.Join(outputDir, ".claude", "commands", "build.md")
	_, err = os.Stat(buildPath)
	assert.NoError(t, err, "expected build.md from project layer to exist")
}

func TestForceLock_Hook(t *testing.T) {
	t.Parallel()
	// Company force-locks PreToolUse hook.
	// Project provides a different PreToolUse hook.
	// Result: company's PreToolUse survives, project's is skipped.
	layers := buildTestLayers(t, "layers-force", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))

	hooks := settings["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)
	require.Len(t, preToolUse, 1, "expected 1 PreToolUse hook (company's force-locked)")

	hook := preToolUse[0].(map[string]any)
	assert.Equal(t, "Write(**/secrets/**)", hook["matcher"],
		"expected company's PreToolUse matcher")

	// Check the hook command is company's "Blocked" version
	hookActions := hook["hooks"].([]any)
	action := hookActions[0].(map[string]any)
	assert.Contains(t, action["command"].(string), "Blocked",
		"expected company's hook command with 'Blocked'")
}

func TestForceLock_ManifestAnnotation(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-force", "company", "project")
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Governance.ForceLocked, "expected force-locked entries in compile result")

	// Should have entries for: mcp:sonarqube, command:security-scan.md, hook:PreToolUse
	expectedLocks := map[string]string{
		"mcp:sonarqube":            "company",
		"command:security-scan.md": "company",
		"hook:PreToolUse":          "company",
	}

	for _, fl := range result.Governance.ForceLocked {
		key := fl.Category + ":" + fl.Key
		expectedLayer, ok := expectedLocks[key]
		if ok {
			assert.Equal(t, expectedLayer, fl.LockedBy, "force-lock %s locked_by", key)
			delete(expectedLocks, key)
		}
	}

	for key := range expectedLocks {
		assert.Failf(t, "missing force-lock entry", "expected force-lock for %s", key)
	}

	// Verify .keel.json contains force_locked section
	manifestData, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
	require.NoError(t, err)

	manifestText := string(manifestData)
	assert.Contains(t, manifestText, `"force_locked"`)
	assert.Contains(t, manifestText, `"category": "mcp"`)
}

func TestForceLock_CursorCompiler(t *testing.T) {
	t.Parallel()
	// Force-lock should work for Cursor compiler too.
	layers := buildTestLayers(t, "layers-force", "company", "project")
	outputDir := t.TempDir()

	result, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// MCP: sonarqube should use company's config
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "mcp.json"))
	require.NoError(t, err, "reading mcp.json")
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp))
	servers := mcp["mcpServers"].(map[string]any)
	sq := servers["sonarqube"].(map[string]any)
	args := sq["args"].([]any)
	assert.Equal(t, "sonarqube-mcp-server", args[0])

	// Command: security-scan.md should be company's
	content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "commands", "security-scan.md"))
	require.NoError(t, err, "reading security-scan.md")
	assert.Contains(t, string(content), "force-locked, cannot be overridden",
		"expected company's force-locked content in Cursor output")

	// Hooks: preToolUse (camelCase) should use company's version
	hooksData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "hooks.json"))
	require.NoError(t, err, "reading hooks.json")
	var hooks map[string]any
	require.NoError(t, json.Unmarshal(hooksData, &hooks))
	preToolUse := hooks["preToolUse"].([]any)
	require.Len(t, preToolUse, 1)
	hook := preToolUse[0].(map[string]any)
	assert.Equal(t, "Write(**/secrets/**)", hook["matcher"],
		"expected company's hook matcher")

	// Force-lock manifest entries should exist
	assert.NotEmpty(t, result.Governance.ForceLocked, "expected force-locked entries for Cursor compiler")
}

func TestForceLock_MixedLayers(t *testing.T) {
	t.Parallel()
	// Layer 1 (DSL): force-locks sonarqube MCP server and security-scan.md command
	// Layer 2 (native): has mcp.json and commands/ trying to override
	// Result: DSL force-locks win; native layer can't override them

	td := testdataDir(t)

	dslLayer := config.Layer{
		Path:   filepath.Join(td, "layers-force", "company"),
		Type:   "company",
		Format: config.FormatKeel,
	}

	// Create a temporary native layer that tries to override
	tmpDir := t.TempDir()
	nativeLayer := filepath.Join(tmpDir, "native_override")
	require.NoError(t, os.MkdirAll(filepath.Join(nativeLayer, "commands"), 0755))

	// Native layer tries to override security-scan.md
	require.NoError(t, os.WriteFile(
		filepath.Join(nativeLayer, "commands", "security-scan.md"),
		[]byte("Native override — should be blocked"),
		0644,
	))

	// Native layer adds a new command
	require.NoError(t, os.WriteFile(
		filepath.Join(nativeLayer, "commands", "native-cmd.md"),
		[]byte("Native command — should be added"),
		0644,
	))

	layers := []config.Layer{
		dslLayer,
		{Path: nativeLayer, Type: "project", Format: config.FormatKeel},
	}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// security-scan.md should have company's content (force-locked)
	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "security-scan.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "force-locked, cannot be overridden",
		"expected company's force-locked security-scan.md in mixed layer test")
	assert.NotContains(t, string(content), "Native override",
		"native layer should NOT override force-locked command")

	// native-cmd.md should be added (not force-locked)
	nativeCmdPath := filepath.Join(outputDir, ".claude", "commands", "native-cmd.md")
	_, err = os.Stat(nativeCmdPath)
	assert.NoError(t, err, "expected native-cmd.md to be added from native layer")
}

// --- splitCompositeKey unit tests ---
