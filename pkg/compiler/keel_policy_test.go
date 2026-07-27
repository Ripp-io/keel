package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// --- extractMCPServerName unit tests ---

func TestExtractMCPServerName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		expected string
	}{
		{"mcpServers.gitlab", "gitlab"},
		{"mcpServers.sonarqube", "sonarqube"},
		{"mcpServers.nested.deep", "nested.deep"},
		{"other.path", ""},
		{"mcpServers", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, extractMCPServerName(tt.path),
				"extractMCPServerName(%q)", tt.path)
		})
	}
}

// --- Claude compiler: _keel policy integration tests ---

func TestCompileClaude_McpKeelPolicyStripped(t *testing.T) {
	t.Parallel()
	// Company layer has mcp.json with _keel on sonarqube (force: true).
	// Project layer has mcp.json trying to override sonarqube.
	// Result: sonarqube from company (force-locked), gitlab from project. No _keel in output.
	layers := buildTestLayers(t, "layers-keel-policy", "company", "project")
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "reading .mcp.json")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp), "parsing .mcp.json")

	servers := mcp["mcpServers"].(map[string]any)

	// sonarqube should use company's config (force-locked by _keel)
	sonarqube, ok := servers["sonarqube"]
	require.True(t, ok, "sonarqube server should exist")
	sq := sonarqube.(map[string]any)
	args := sq["args"].([]any)
	assert.Equal(t, 1, len(args), "sonarqube args length")
	assert.Equal(t, "sonarqube-mcp-server", args[0], "sonarqube args[0] (company's value)")

	// _keel should be stripped from sonarqube
	assert.NotContains(t, sq, "_keel", "sonarqube should not contain _keel")

	// gitlab should be added by project (not force-locked)
	assert.Contains(t, servers, "gitlab", "gitlab server from project layer")

	// Verify _keel is not anywhere in the output
	testutil.AssertNoKeelKeys(t, mcp)

	// Verify force-lock manifest annotation for mcp:sonarqube
	found := false
	for _, fl := range result.Governance.ForceLocked {
		if fl.Category == "mcp" && fl.Key == "sonarqube" && fl.LockedBy == "company" {
			found = true
			break
		}
	}
	assert.True(t, found, "force-lock manifest entry for mcp:sonarqube")
}

func TestCompileClaude_SettingsKeelPolicyStripped(t *testing.T) {
	t.Parallel()
	// Company layer has settings.json with _keel on mcpServers.jira (force: true).
	// Project layer has settings.json trying to override jira and adding context7.
	// Result: jira from company (force-locked), context7 from project. No _keel in output.
	layers := buildTestLayers(t, "layers-keel-policy", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings), "parsing settings.json")

	// _keel should not appear anywhere in settings
	testutil.AssertNoKeelKeys(t, settings)

	// mcpServers should exist (from both layers merged)
	mcpServers, ok := settings["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers in settings.json")

	// jira should exist (from company, force-locked by _keel)
	jira, ok := mcpServers["jira"].(map[string]any)
	require.True(t, ok, "jira in mcpServers")

	// _keel should be stripped from jira
	assert.NotContains(t, jira, "_keel", "jira should not contain _keel in settings.json")
}

func TestCompileClaude_KeelPolicyNoKeelInOutput(t *testing.T) {
	t.Parallel()
	// Single layer with _keel -- verify clean output
	layers := buildTestLayers(t, "layers-keel-policy", "company")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	// Check .mcp.json
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "reading .mcp.json")
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp), "parsing .mcp.json")
	testutil.AssertNoKeelKeys(t, mcp)

	// Check settings.json
	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")
	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings), "parsing settings.json")
	testutil.AssertNoKeelKeys(t, settings)
}

// --- Cursor compiler: _keel policy integration tests ---

func TestCompileCursor_McpKeelPolicyStripped(t *testing.T) {
	t.Parallel()
	// Company layer has mcp.json with _keel on sonarqube (force: true).
	// Project layer has mcp.json trying to override sonarqube.
	// Result: sonarqube from company (force-locked), gitlab from project. No _keel in output.
	layers := buildTestLayers(t, "layers-keel-policy", "company", "project")
	outputDir := t.TempDir()

	result, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "mcp.json"))
	require.NoError(t, err, "reading mcp.json")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp), "parsing mcp.json")

	servers := mcp["mcpServers"].(map[string]any)

	// sonarqube should use company's config (force-locked by _keel)
	sonarqube := servers["sonarqube"].(map[string]any)
	args := sonarqube["args"].([]any)
	assert.Equal(t, "sonarqube-mcp-server", args[0], "company's sonarqube args")

	// _keel should not be in output
	testutil.AssertNoKeelKeys(t, mcp)

	// gitlab should be added by project
	assert.Contains(t, servers, "gitlab", "gitlab server from project layer")

	// Force-lock entry should exist
	found := false
	for _, fl := range result.Governance.ForceLocked {
		if fl.Category == "mcp" && fl.Key == "sonarqube" {
			found = true
			break
		}
	}
	assert.True(t, found, "force-lock for mcp:sonarqube in cursor output")
}

func TestCompileCursor_CliConfigKeelPolicyStripped(t *testing.T) {
	t.Parallel()
	// Company layer has cli-config.json with _keel (force: false).
	// _keel should be stripped but no force-lock registered.
	layers := buildTestLayers(t, "layers-keel-policy", "company")
	outputDir := t.TempDir()

	result, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	cliConfigData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err, "reading cli-config.json")

	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(cliConfigData, &cliConfig), "parsing cli-config.json")

	// _keel should be stripped
	testutil.AssertNoKeelKeys(t, cliConfig)

	// version and permissions.allow should survive
	assert.Equal(t, float64(1), cliConfig["version"], "version")

	// force: false should NOT produce a force-lock
	for _, fl := range result.Governance.ForceLocked {
		assert.NotEqual(t, "settings-key", fl.Category,
			"unexpected settings-key force-lock: %+v (force was false)", fl)
	}
}

// --- Cursor compiler: hooks _keel policy integration tests ---

func TestKeelPolicy_CursorHooksDeepMerge(t *testing.T) {
	t.Parallel()
	// Two layers: company has preToolUse, project has preToolUse + postToolUse
	// Deep-merge should combine them (project's preToolUse wins, postToolUse added)
	layers := buildTestLayers(t, "layers-keel-policy", "company", "project")
	outputDir := t.TempDir()

	result, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	hooksPath := filepath.Join(outputDir, ".cursor", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	require.NoError(t, err, "reading hooks.json")

	var hooks map[string]any
	require.NoError(t, json.Unmarshal(data, &hooks), "parsing hooks.json")

	// postToolUse should exist (from project layer)
	assert.Contains(t, hooks, "postToolUse", "deep-merged hooks.json")

	// Verify no _keel metadata leaked
	assert.NotContains(t, hooks, "_keel", "_keel should be stripped from hooks.json output")

	// Check strategy is deep-merge in file records
	for _, f := range result.Output.Files {
		if f.Output == ".cursor/hooks.json" {
			assert.Equal(t, "deep-merge", f.Strategy, "hooks.json strategy")
		}
	}
}

// --- Additive merge integration tests ---

func TestKeelPolicy_AdditivePermissions_Claude(t *testing.T) {
	t.Parallel()
	// Company layer: permissions.json has _keel.merge: additive on allow
	// Project layer: permissions.json adds "Write" to allow
	// Result: allow should be ["Read", "Edit", "Write"] (union, not replacement)
	layers := buildTestLayers(t, "layers-additive", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings), "parsing settings.json")

	// _keel should be stripped
	testutil.AssertNoKeelKeys(t, settings)

	// permissions.allow should be the union
	perms, ok := settings["permissions"].(map[string]any)
	require.True(t, ok, "permissions in settings.json, got %T", settings["permissions"])
	allow, ok := perms["allow"].([]any)
	require.True(t, ok, "allow should be []any, got %T", perms["allow"])

	// Verify union: company's ["Read", "Edit"] + project's ["Read", "Write"] = ["Read", "Edit", "Write"]
	require.Len(t, allow, 3, "permissions (union)")

	// Build a set for easy checking
	allowSet := make(map[string]bool)
	for _, v := range allow {
		s, ok := v.(string)
		require.True(t, ok, "expected string in allow array, got %T", v)
		allowSet[s] = true
	}

	for _, expected := range []string{"Read", "Edit", "Write"} {
		assert.True(t, allowSet[expected], "%q should be in allow array", expected)
	}
}

func TestKeelPolicy_CursorHooksForceLock(t *testing.T) {
	t.Parallel()
	// Company layer force-locks preToolUse via _keel policy.
	// Project layer tries to override preToolUse -- should be blocked.
	layers := buildTestLayers(t, "layers-keel-policy", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	hooksPath := filepath.Join(outputDir, ".cursor", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	require.NoError(t, err, "reading hooks.json")

	var hooks map[string]any
	require.NoError(t, json.Unmarshal(data, &hooks), "parsing hooks.json")

	// preToolUse should contain company's deny rule, not project's allow rule
	preToolUse, ok := hooks["preToolUse"]
	require.True(t, ok, "preToolUse in hooks.json")

	preArr, ok := preToolUse.([]any)
	require.True(t, ok && len(preArr) > 0, "preToolUse should be a non-empty array")

	firstHook, ok := preArr[0].(map[string]any)
	require.True(t, ok, "first hook should be an object")

	// Company's rule should win (force-locked)
	if action, ok := firstHook["action"]; ok {
		assert.Equal(t, "deny", action, "force-locked preToolUse action")
	}
	if matcher, ok := firstHook["matcher"]; ok {
		assert.Equal(t, "Write(**/secrets/**)", matcher, "force-locked preToolUse matcher (company's value)")
	}

	// postToolUse should still be present (not force-locked)
	assert.Contains(t, hooks, "postToolUse", "postToolUse should pass through (not force-locked)")
}
