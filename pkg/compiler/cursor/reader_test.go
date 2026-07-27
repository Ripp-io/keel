package cursor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// testdataDir returns the path to the parent compiler package's testdata directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "unable to determine test file location")
	return filepath.Join(filepath.Dir(filename), "..", "testdata")
}

func cursorReaderTestdataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(testdataDir(t), "reader-cursor")
}

func TestReader_Full(t *testing.T) {
	t.Parallel()
	reader := &Reader{}
	layerPath := filepath.Join(cursorReaderTestdataDir(t), "full")
	ir, err := reader.Read(layerPath)
	require.NoError(t, err, "Read()")
	assert.Equal(t, "cursor", ir.SourceAgent, "SourceAgent")
	require.Len(t, ir.Instructions, 1, "instruction blocks")
	assert.Equal(t, "instructions.md", ir.Instructions[0].Filename, "instruction filename")
	assert.Contains(t, ir.Instructions[0].Content, "Test Cursor Project", "instruction content")
	require.NotNil(t, ir.Permissions, "Permissions")
	allowRaw, ok := ir.Permissions["allow"]
	require.True(t, ok, "Permissions should have allow key")
	allowSlice, ok := allowRaw.([]any)
	require.True(t, ok, "allow should be []any, got %T", allowRaw)
	expectedAllow := map[string]bool{"Read": true, "Edit": true, "Bash(git:*)": true}
	for _, perm := range allowSlice {
		permStr, ok := perm.(string)
		require.True(t, ok, "permission should be string, got %T", perm)
		assert.True(t, expectedAllow[permStr], "unexpected allow permission %q", permStr)
		delete(expectedAllow, permStr)
	}
	for perm := range expectedAllow {
		assert.Fail(t, "missing allow permission", perm)
	}
	denyRaw, ok := ir.Permissions["deny"]
	require.True(t, ok, "Permissions should have deny key")
	denySlice, ok := denyRaw.([]any)
	require.True(t, ok, "deny should be []any, got %T", denyRaw)
	require.Len(t, denySlice, 1, "deny entries")
	denyStr, ok := denySlice[0].(string)
	require.True(t, ok, "deny[0] should be string")
	assert.Equal(t, "Bash(rm:*)", denyStr, "deny permissions")
	require.NotNil(t, ir.Settings, "Settings")
	model, ok := ir.Settings["model"]
	require.True(t, ok, "Settings should have model key")
	assert.Equal(t, "gpt-4o", model, "model setting")
	require.NotNil(t, ir.AgentSpecific, "AgentSpecific")
	cursorSpecific, ok := ir.AgentSpecific["cursor"]
	require.True(t, ok, "AgentSpecific should have cursor key")
	assert.Contains(t, cursorSpecific, "editor.vimMode", "AgentSpecific[cursor] keys")
	require.NotNil(t, ir.Hooks, "Hooks")
	assert.Contains(t, ir.Hooks, "PreToolUse", "Hooks keys")
	assert.NotContains(t, ir.Hooks, "preToolUse", "camelCase key should be absent")
	require.NotNil(t, ir.MCP, "MCP")
	assert.Contains(t, ir.MCP, "mcpServers", "MCP keys")
	require.NotNil(t, ir.ConfigBlocks["environment"], "Environment")
	nodeEnv, ok := ir.ConfigBlocks["environment"]["NODE_ENV"]
	require.True(t, ok, "Environment should have NODE_ENV key")
	assert.Equal(t, "development", nodeEnv, "NODE_ENV value")
	require.NotNil(t, ir.Categories, "Categories")
	commands, ok := ir.Categories["commands"]
	require.True(t, ok, "Categories should have commands key")
	assert.Len(t, commands, 1, "command files")
	foundDeploy := false
	for _, f := range commands {
		if f.RelPath == "deploy.md" {
			foundDeploy = true
		}
	}
	assert.True(t, foundDeploy, "commands should include deploy.md")
	rules, ok := ir.Categories["rules"]
	require.True(t, ok, "Categories should have rules key")
	assert.Len(t, rules, 1, "rule files")
	foundSecurity := false
	for _, f := range rules {
		if f.RelPath == "security.mdc" {
			foundSecurity = true
		}
	}
	assert.True(t, foundSecurity, "rules should include security.mdc")
}

func TestReader_Minimal(t *testing.T) {
	t.Parallel()
	reader := &Reader{}
	layerPath := filepath.Join(cursorReaderTestdataDir(t), "minimal")
	ir, err := reader.Read(layerPath)
	require.NoError(t, err, "Read()")
	assert.Equal(t, "cursor", ir.SourceAgent, "SourceAgent")
	require.Len(t, ir.Instructions, 1, "instruction blocks")
	assert.Contains(t, ir.Instructions[0].Content, "Minimal Cursor Project", "instruction content")
	assert.Nil(t, ir.Permissions, "Permissions")
	assert.Nil(t, ir.Hooks, "Hooks")
	assert.Nil(t, ir.Settings, "Settings")
	assert.Nil(t, ir.MCP, "MCP")
	assert.Nil(t, ir.ConfigBlocks["environment"], "Environment")
	assert.Empty(t, ir.Categories, "Categories")
	assert.Nil(t, ir.AgentSpecific, "AgentSpecific")
}

func TestReader_Empty(t *testing.T) {
	t.Parallel()
	reader := &Reader{}
	layerPath := filepath.Join(cursorReaderTestdataDir(t), "empty")
	ir, err := reader.Read(layerPath)
	require.NoError(t, err, "Read()")
	assert.Equal(t, "cursor", ir.SourceAgent, "SourceAgent")
	assert.Empty(t, ir.Instructions, "Instructions")
	assert.Nil(t, ir.Permissions, "Permissions")
	assert.Nil(t, ir.Hooks, "Hooks")
	assert.Nil(t, ir.Settings, "Settings")
	assert.Nil(t, ir.MCP, "MCP")
	assert.Nil(t, ir.ConfigBlocks["environment"], "Environment")
	assert.Empty(t, ir.Categories, "Categories")
	assert.Nil(t, ir.AgentSpecific, "AgentSpecific")
}

func TestReader_PermissionStripping(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cursorDir := filepath.Join(dir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))
	cliConfig := map[string]any{
		"version":           float64(1),
		"permissions.allow": []string{"Read(*)", "Bash(git:*)", "Write(*)"},
		"permissions.deny":  []string{"Bash(rm:*)"},
	}
	data, err := json.Marshal(cliConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "cli-config.json"), data, 0644))
	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err, "Read()")
	require.NotNil(t, ir.Permissions, "Permissions")
	allowRaw, ok := ir.Permissions["allow"]
	require.True(t, ok, "Permissions should have allow key")
	allowSlice, ok := allowRaw.([]any)
	require.True(t, ok, "allow should be []any, got %T", allowRaw)
	expectedAllow := []string{"Read", "Bash(git:*)", "Write"}
	require.Len(t, allowSlice, len(expectedAllow), "allow entries")
	for i, want := range expectedAllow {
		got, ok := allowSlice[i].(string)
		require.True(t, ok, "allow[%d] should be string, got %T", i, allowSlice[i])
		assert.Equal(t, want, got, "allow[%d]", i)
	}
	denyRaw, ok := ir.Permissions["deny"]
	require.True(t, ok, "Permissions should have deny key")
	denySlice, ok := denyRaw.([]any)
	require.True(t, ok, "deny should be []any, got %T", denyRaw)
	require.Len(t, denySlice, 1, "deny entries")
	denyStr, ok := denySlice[0].(string)
	require.True(t, ok, "deny[0] should be string")
	assert.Equal(t, "Bash(rm:*)", denyStr, "deny permissions")
}

func TestReader_HookKeyTransform(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hooks := map[string]any{
		"preToolUse":  []any{map[string]any{"matcher": "Write", "hooks": []string{"echo pre"}}},
		"postToolUse": []any{map[string]any{"matcher": "Read", "hooks": []string{"echo post"}}},
	}
	data, err := json.Marshal(hooks)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hooks.json"), data, 0644))
	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err, "Read()")
	require.NotNil(t, ir.Hooks, "Hooks")
	assert.Contains(t, ir.Hooks, "PreToolUse", "Hooks keys")
	assert.Contains(t, ir.Hooks, "PostToolUse", "Hooks keys")
	assert.NotContains(t, ir.Hooks, "preToolUse", "camelCase keys should be absent")
	assert.NotContains(t, ir.Hooks, "postToolUse", "camelCase keys should be absent")
}

func TestReader_HookKeyTransformRename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hooks := map[string]any{
		"beforeSubmitPrompt": []any{map[string]any{"matcher": "*", "hooks": []string{"echo submit"}}},
		"preToolUse":         []any{map[string]any{"matcher": "Write", "hooks": []string{"echo pre"}}},
	}
	data, err := json.Marshal(hooks)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hooks.json"), data, 0644))
	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err, "Read()")
	require.NotNil(t, ir.Hooks, "Hooks")
	assert.Contains(t, ir.Hooks, "UserPromptSubmit", "should be renamed from beforeSubmitPrompt")
	assert.NotContains(t, ir.Hooks, "beforeSubmitPrompt", "Cursor-native key should be absent in IR")
	assert.NotContains(t, ir.Hooks, "BeforeSubmitPrompt", "naive PascalCase should be absent")
	assert.Contains(t, ir.Hooks, "PreToolUse", "standard PascalCase transform")
}

func TestCursorSandboxToCanonical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		expected func(*testing.T, *schema.SandboxConfig)
	}{
		{
			name: "all cursor fields",
			input: map[string]any{
				"type":                     "docker",
				"additionalReadwritePaths": []any{"/project", "/cache"},
				"additionalReadonlyPaths":  []any{"/usr/lib", "/etc"},
				"disableTmpWrite":          true,
				"enableSharedBuildCache":   true,
			},
			expected: func(t *testing.T, sb *schema.SandboxConfig) {
				assert.Equal(t, "docker", sb.Type)
				assert.Equal(t, []string{"/project", "/cache"}, sb.AdditionalReadwritePaths)
				assert.Equal(t, []string{"/usr/lib", "/etc"}, sb.AdditionalReadonlyPaths)
				require.NotNil(t, sb.DisableTmpWrite)
				assert.True(t, *sb.DisableTmpWrite)
				require.NotNil(t, sb.EnableSharedBuildCache)
				assert.True(t, *sb.EnableSharedBuildCache)
			},
		},
		{
			name: "network policy",
			input: map[string]any{
				"networkPolicy": map[string]any{
					"default": "deny",
					"allow":   []any{"example.com", "api.github.com"},
					"deny":    []any{"malicious.com"},
				},
			},
			expected: func(t *testing.T, sb *schema.SandboxConfig) {
				require.NotNil(t, sb.Network)
				assert.Equal(t, "deny", sb.Network.DefaultPolicy)
				assert.Equal(t, []string{"example.com", "api.github.com"}, sb.Network.AllowedDomains)
				assert.Equal(t, []string{"malicious.com"}, sb.Network.DeniedDomains)
			},
		},
		{
			name: "mixed fields",
			input: map[string]any{
				"type":                     "vm",
				"additionalReadwritePaths": []any{"/workspace"},
				"networkPolicy": map[string]any{
					"default": "allow",
					"deny":    []any{"*.evil.com"},
				},
			},
			expected: func(t *testing.T, sb *schema.SandboxConfig) {
				assert.Equal(t, "vm", sb.Type)
				assert.Equal(t, []string{"/workspace"}, sb.AdditionalReadwritePaths)
				require.NotNil(t, sb.Network)
				assert.Equal(t, "allow", sb.Network.DefaultPolicy)
				assert.Equal(t, []string{"*.evil.com"}, sb.Network.DeniedDomains)
			},
		},
		{
			name:  "empty input",
			input: map[string]any{},
			expected: func(t *testing.T, sb *schema.SandboxConfig) {
				assert.Empty(t, sb.Type)
				assert.Nil(t, sb.AdditionalReadwritePaths)
				assert.Nil(t, sb.AdditionalReadonlyPaths)
				assert.Nil(t, sb.DisableTmpWrite)
				assert.Nil(t, sb.EnableSharedBuildCache)
				assert.Nil(t, sb.Network)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cursorSandboxToCanonical(tt.input)
			require.NotNil(t, result)
			tt.expected(t, result)
		})
	}
}

func TestBuildCursorSandboxFromCanonical(t *testing.T) {
	t.Parallel()

	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		input    *schema.SandboxConfig
		expected map[string]any
	}{
		{
			name: "all cursor fields",
			input: &schema.SandboxConfig{
				Type:                     "docker",
				AdditionalReadwritePaths: []string{"/project", "/cache"},
				AdditionalReadonlyPaths:  []string{"/usr/lib", "/etc"},
				DisableTmpWrite:          &trueVal,
				EnableSharedBuildCache:   &falseVal,
			},
			expected: map[string]any{
				"type":                     "docker",
				"additionalReadwritePaths": []string{"/project", "/cache"},
				"additionalReadonlyPaths":  []string{"/usr/lib", "/etc"},
				"disableTmpWrite":          true,
				"enableSharedBuildCache":   false,
			},
		},
		{
			name: "network policy",
			input: &schema.SandboxConfig{
				Network: &schema.SandboxNetConfig{
					DefaultPolicy:  "deny",
					AllowedDomains: []string{"example.com", "api.github.com"},
					DeniedDomains:  []string{"malicious.com"},
				},
			},
			expected: map[string]any{
				"networkPolicy": map[string]any{
					"default": "deny",
					"allow":   []string{"example.com", "api.github.com"},
					"deny":    []string{"malicious.com"},
				},
			},
		},
		{
			name: "portable filesystem merged with cursor paths",
			input: &schema.SandboxConfig{
				AdditionalReadwritePaths: []string{"/cursor/rw"},
				AdditionalReadonlyPaths:  []string{"/cursor/ro"},
				Filesystem: &schema.SandboxFSConfig{
					AllowWrite: []string{"/portable/write"},
					AllowRead:  []string{"/portable/read"},
				},
			},
			expected: map[string]any{
				"additionalReadwritePaths": []string{"/cursor/rw", "/portable/write"},
				"additionalReadonlyPaths":  []string{"/cursor/ro", "/portable/read"},
			},
		},
		{
			name:     "empty input",
			input:    &schema.SandboxConfig{},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCursorSandboxFromCanonical(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCursorSandboxRoundTrip(t *testing.T) {
	t.Parallel()

	trueVal := true
	original := map[string]any{
		"type":                     "docker",
		"additionalReadwritePaths": []any{"/project", "/cache"},
		"additionalReadonlyPaths":  []any{"/usr/lib"},
		"disableTmpWrite":          true,
		"enableSharedBuildCache":   true,
		"networkPolicy": map[string]any{
			"default": "deny",
			"allow":   []any{"example.com"},
			"deny":    []any{"malicious.com"},
		},
	}

	// Cursor → Canonical
	canonical := cursorSandboxToCanonical(original)
	require.NotNil(t, canonical)

	// Canonical → Cursor
	roundtripped := buildCursorSandboxFromCanonical(canonical)

	// Verify equivalence
	assert.Equal(t, "docker", roundtripped["type"])
	assert.Equal(t, []string{"/project", "/cache"}, roundtripped["additionalReadwritePaths"])
	assert.Equal(t, []string{"/usr/lib"}, roundtripped["additionalReadonlyPaths"])
	assert.Equal(t, true, roundtripped["disableTmpWrite"])
	assert.Equal(t, true, roundtripped["enableSharedBuildCache"])

	np, ok := roundtripped["networkPolicy"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "deny", np["default"])
	assert.Equal(t, []string{"example.com"}, np["allow"])
	assert.Equal(t, []string{"malicious.com"}, np["deny"])

	// Also test the reverse: Canonical → Cursor → Canonical
	canonicalInput := &schema.SandboxConfig{
		Type:                     "vm",
		AdditionalReadwritePaths: []string{"/workspace"},
		DisableTmpWrite:          &trueVal,
		Network: &schema.SandboxNetConfig{
			DefaultPolicy: "allow",
			DeniedDomains: []string{"*.evil.com"},
		},
	}

	cursorFormat := buildCursorSandboxFromCanonical(canonicalInput)

	// Simulate real-world usage: marshal to JSON and unmarshal back
	// (this is what happens when writing to disk and reading back)
	cursorJSON, err := json.Marshal(cursorFormat)
	require.NoError(t, err)
	var cursorFromJSON map[string]any
	err = json.Unmarshal(cursorJSON, &cursorFromJSON)
	require.NoError(t, err)

	roundtrippedCanonical := cursorSandboxToCanonical(cursorFromJSON)

	assert.Equal(t, canonicalInput.Type, roundtrippedCanonical.Type)
	assert.Equal(t, canonicalInput.AdditionalReadwritePaths, roundtrippedCanonical.AdditionalReadwritePaths)
	assert.Equal(t, canonicalInput.DisableTmpWrite, roundtrippedCanonical.DisableTmpWrite)
	require.NotNil(t, roundtrippedCanonical.Network)
	assert.Equal(t, canonicalInput.Network.DefaultPolicy, roundtrippedCanonical.Network.DefaultPolicy)
	assert.Equal(t, canonicalInput.Network.DeniedDomains, roundtrippedCanonical.Network.DeniedDomains)
}
