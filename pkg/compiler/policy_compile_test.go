package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Phase 6: MCP policy compilation tests ---

func TestCompileClaude_MCPPolicyDenied(t *testing.T) {
	t.Parallel()
	// Company layer sets mcp.policy.denied and mcp.policy.enableAll.
	// Claude settings.json should contain deniedMcpServers and enableAllProjectMcpServers.
	layers := buildTestLayers(t, "layers-mcp-policy", "company")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings), "parsing settings.json")

	// Verify deniedMcpServers
	denied, ok := settings["deniedMcpServers"]
	require.True(t, ok, "expected deniedMcpServers key in settings.json")
	deniedArr, ok := denied.([]any)
	require.True(t, ok, "expected deniedMcpServers to be an array, got %T", denied)
	assert.Equal(t, []any{"untrusted-server"}, deniedArr)

	// Verify enableAllProjectMcpServers
	enableAll, ok := settings["enableAllProjectMcpServers"]
	require.True(t, ok, "expected enableAllProjectMcpServers key in settings.json")
	assert.Equal(t, false, enableAll)

	// allowedMcpServers should NOT be present (not set in this layer)
	_, ok = settings["allowedMcpServers"]
	assert.False(t, ok, "expected allowedMcpServers to NOT be present when not set")
}

func TestCompileClaude_MCPPolicyAllowed(t *testing.T) {
	t.Parallel()
	// Project layer sets mcp.policy.allowed.
	// Claude settings.json should contain allowedMcpServers.
	layers := buildTestLayers(t, "layers-mcp-policy", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings), "parsing settings.json")

	// Verify allowedMcpServers
	allowed, ok := settings["allowedMcpServers"]
	require.True(t, ok, "expected allowedMcpServers key in settings.json")
	allowedArr, ok := allowed.([]any)
	require.True(t, ok, "expected allowedMcpServers to be an array, got %T", allowed)
	require.Len(t, allowedArr, 2)
	assert.Equal(t, "sonarqube", allowedArr[0])
	assert.Equal(t, "jira", allowedArr[1])
}

func TestCompileClaude_MCPPolicyMergedAcrossLayers(t *testing.T) {
	t.Parallel()
	// Company sets denied + enableAll, project sets allowed.
	// After deep-merge, settings.json should have all three keys.
	layers := buildTestLayers(t, "layers-mcp-policy", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings), "parsing settings.json")

	// All three keys should be present
	assert.Contains(t, settings, "deniedMcpServers", "expected deniedMcpServers from company layer")
	assert.Contains(t, settings, "enableAllProjectMcpServers", "expected enableAllProjectMcpServers from company layer")
	assert.Contains(t, settings, "allowedMcpServers", "expected allowedMcpServers from project layer")
}

func TestCompileCursor_MCPPolicyWarnsNoOutput(t *testing.T) {
	t.Parallel()
	// Cursor does not support MCP policy. Compilation should succeed
	// without producing MCP governance keys in cli-config.json.
	layers := buildTestLayers(t, "layers-mcp-policy", "company")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	cliConfigPath := filepath.Join(outputDir, ".cursor", "cli-config.json")
	if _, err := os.Stat(cliConfigPath); os.IsNotExist(err) {
		// Cursor with no permissions layers may not produce cli-config.json.
		// That's fine -- the point is no MCP policy keys leak.
		return
	}

	data, err := os.ReadFile(cliConfigPath)
	require.NoError(t, err, "reading cli-config.json")

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config), "parsing cli-config.json")

	// None of the MCP governance keys should be present
	for _, key := range []string{"allowedMcpServers", "deniedMcpServers", "enableAllProjectMcpServers"} {
		assert.NotContains(t, config, key, "Cursor cli-config.json should not contain %q (MCP policy is Claude-only)", key)
	}
}

// --- Phase 7: Permissions policy compilation tests ---

func TestCompileClaude_PermissionsPolicyAdditive(t *testing.T) {
	t.Parallel()
	// Company layer sets permissions.policy.merge=additive.
	// Project layer adds more permissions.
	// Result: permissions arrays should be UNIONED, not replaced.
	layers := buildTestLayers(t, "layers-perm-policy", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings), "parsing settings.json")

	perms, ok := settings["permissions"].(map[string]any)
	require.True(t, ok, "expected permissions key in settings.json")

	allow, ok := perms["allow"].([]any)
	require.True(t, ok, "expected permissions.allow to be an array")

	// With additive merge: company [Read, Edit] + project [Write, Bash(git:*)]
	// = [Read, Edit, Write, Bash(git:*)]
	assert.Len(t, allow, 4, "expected 4 permissions (additive union)")

	// Check that both company and project permissions are present
	permSet := make(map[string]bool)
	for _, p := range allow {
		permSet[p.(string)] = true
	}
	for _, expected := range []string{"Read", "Edit", "Write", "Bash(git:*)"} {
		assert.True(t, permSet[expected], "expected permission %q in additive union", expected)
	}
}

func TestCompileCursor_PermissionsPolicyAdditive(t *testing.T) {
	t.Parallel()
	// Same test but for Cursor: additive merge should union permission arrays.
	layers := buildTestLayers(t, "layers-perm-policy", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")

	cliConfigPath := filepath.Join(outputDir, ".cursor", "cli-config.json")
	data, err := os.ReadFile(cliConfigPath)
	require.NoError(t, err, "reading cli-config.json")

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config), "parsing cli-config.json")

	// Cursor uses "permissions.allow" as a flat key
	allow, ok := config["permissions.allow"].([]any)
	require.True(t, ok, "expected permissions.allow key in cli-config.json")

	// With additive merge: company [Read(*), Edit(*)] + project [Write(*), Bash(git:*)]
	// = [Read(*), Edit(*), Write(*), Bash(git:*)]  (Cursor format adds (*))
	assert.Len(t, allow, 4, "expected 4 permissions (additive union)")
}
