package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// --- readExternalSettingsJSON tests ---

func TestReadExternalSettingsJSON_EmptyRelPath(t *testing.T) {
	t.Parallel()
	result := readExternalSettingsJSON(t.TempDir(), "")
	assert.Nil(t, result, "expected nil for empty relPath")
}

func TestReadExternalSettingsJSON_ValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, "settings.claude.json")
	require.NoError(t, os.WriteFile(settingsFile, []byte(`{"key": "value", "num": 42}`), 0644))

	result := readExternalSettingsJSON(dir, "settings.claude.json")
	require.NotNil(t, result, "expected non-nil result for valid JSON file")
	assert.Equal(t, "value", result["key"])
	// JSON numbers are float64 after unmarshal.
	assert.Equal(t, float64(42), result["num"])
}

func TestReadExternalSettingsJSON_FileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result := readExternalSettingsJSON(dir, "nonexistent.json")
	assert.Nil(t, result, "expected nil for nonexistent file")
}

func TestReadExternalSettingsJSON_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(badFile, []byte(`{invalid`), 0644))

	result := readExternalSettingsJSON(dir, "bad.json")
	assert.Nil(t, result, "expected nil for invalid JSON")
}

func TestReadExternalSettingsJSON_PathEscapesLayer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a file outside the layer.
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.json")
	require.NoError(t, os.WriteFile(outsideFile, []byte(`{"secret": true}`), 0644))

	// Try to escape via relative path.
	result := readExternalSettingsJSON(dir, "../../etc/passwd")
	assert.Nil(t, result, "expected nil for path that escapes layer boundary")
}

func TestReadExternalSettingsJSON_NestedPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0755))
	settingsFile := filepath.Join(dir, "config", "agent.json")
	require.NoError(t, os.WriteFile(settingsFile, []byte(`{"nested": true}`), 0644))

	result := readExternalSettingsJSON(dir, "config/agent.json")
	require.NotNil(t, result, "expected non-nil result for nested valid JSON file")
	assert.Equal(t, true, result["nested"])
}

// --- readAndMergePermissions tests ---

func TestReadAndMergePermissions_EmptyRefs(t *testing.T) {
	t.Parallel()
	merged, policies := readAndMergePermissions(t.TempDir(), nil)
	assert.Empty(t, merged, "expected empty merged map for nil refs")
	assert.Empty(t, policies, "expected empty policies for nil refs")
}

func TestReadAndMergePermissions_EmptySliceRefs(t *testing.T) {
	t.Parallel()
	merged, policies := readAndMergePermissions(t.TempDir(), []schema.FileRef{})
	assert.Empty(t, merged, "expected empty merged map for empty refs")
	assert.Empty(t, policies, "expected empty policies for empty refs")
}

func TestReadAndMergePermissions_ValidSingleFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	permFile := filepath.Join(dir, "permissions.json")
	require.NoError(t, os.WriteFile(permFile, []byte(`{"allow": ["Read", "Write"]}`), 0644))

	refs := []schema.FileRef{{Source: "permissions.json"}}
	merged, _ := readAndMergePermissions(dir, refs)

	allow, ok := merged["allow"]
	require.True(t, ok, "expected 'allow' key in merged permissions")
	arr, ok := allow.([]any)
	require.True(t, ok, "expected 'allow' to be []any, got %T", allow)
	assert.Len(t, arr, 2)
}

func TestReadAndMergePermissions_PathEscapesBoundary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	refs := []schema.FileRef{{Source: "../../escape/permissions.json"}}
	merged, policies := readAndMergePermissions(dir, refs)

	assert.Empty(t, merged, "expected empty merged map for escaped path")
	assert.Empty(t, policies, "expected empty policies for escaped path")
}

func TestReadAndMergePermissions_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	badFile := filepath.Join(dir, "perms.json")
	require.NoError(t, os.WriteFile(badFile, []byte(`not json at all`), 0644))

	refs := []schema.FileRef{{Source: "perms.json"}}
	merged, policies := readAndMergePermissions(dir, refs)

	assert.Empty(t, merged, "expected empty merged map for invalid JSON")
	assert.Empty(t, policies, "expected empty policies for invalid JSON")
}

func TestReadAndMergePermissions_MultipleValidFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	file1 := filepath.Join(dir, "perms1.json")
	require.NoError(t, os.WriteFile(file1, []byte(`{"allow": ["Read"]}`), 0644))
	file2 := filepath.Join(dir, "perms2.json")
	require.NoError(t, os.WriteFile(file2, []byte(`{"deny": ["Exec"]}`), 0644))

	refs := []schema.FileRef{
		{Source: "perms1.json"},
		{Source: "perms2.json"},
	}
	merged, _ := readAndMergePermissions(dir, refs)

	assert.Contains(t, merged, "allow", "expected 'allow' key from first file")
	assert.Contains(t, merged, "deny", "expected 'deny' key from second file")
}

func TestReadAndMergePermissions_FileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	refs := []schema.FileRef{{Source: "nonexistent.json"}}
	merged, policies := readAndMergePermissions(dir, refs)

	assert.Empty(t, merged, "expected empty merged map for missing file")
	assert.Empty(t, policies, "expected empty policies for missing file")
}

func TestReadAndMergePermissions_PathRef(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	permFile := filepath.Join(dir, "alt-perms.json")
	require.NoError(t, os.WriteFile(permFile, []byte(`{"allow": ["Read"]}`), 0644))

	refs := []schema.FileRef{{Source: "alt-perms.json"}}
	merged, _ := readAndMergePermissions(dir, refs)

	assert.Contains(t, merged, "allow", "expected 'allow' key when using Source ref")
}

func TestReadAndMergePermissions_EmptySourceAndPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	refs := []schema.FileRef{{}} // both Source and Path are empty
	merged, policies := readAndMergePermissions(dir, refs)

	assert.Empty(t, merged, "expected empty map for ref with no source/path")
	assert.Empty(t, policies, "expected empty policies for ref with no source/path")
}

// --- readHooksJSON tests ---

func TestReadHooksJSON_FileNotFound(t *testing.T) {
	t.Parallel()
	result, err := readHooksJSON(filepath.Join(t.TempDir(), "nonexistent.json"))
	assert.NoError(t, err, "expected nil error for missing file")
	assert.Nil(t, result, "expected nil result for missing file")
}

func TestReadHooksJSON_ValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hooksFile := filepath.Join(dir, "hooks.json")
	content := `{
		"PreToolUse": [{"matcher": "Write(**)", "hooks": [{"command": "echo blocked"}]}],
		"PostToolUse": [{"matcher": "Read(**)", "hooks": [{"command": "echo ok"}]}]
	}`
	require.NoError(t, os.WriteFile(hooksFile, []byte(content), 0644))

	result, err := readHooksJSON(hooksFile)
	require.NoError(t, err)
	require.NotNil(t, result, "expected non-nil result for valid hooks JSON")

	assert.Contains(t, result, "PreToolUse")
	assert.Contains(t, result, "PostToolUse")
}

func TestReadHooksJSON_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hooksFile := filepath.Join(dir, "hooks.json")
	require.NoError(t, os.WriteFile(hooksFile, []byte(`not valid json`), 0644))

	result, err := readHooksJSON(hooksFile)
	require.Error(t, err, "expected error for invalid JSON")
	assert.Nil(t, result, "expected nil result for invalid JSON")
}

func TestReadHooksJSON_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hooksFile := filepath.Join(dir, "hooks.json")
	require.NoError(t, os.WriteFile(hooksFile, []byte(""), 0644))

	result, err := readHooksJSON(hooksFile)
	require.Error(t, err, "expected error for empty file (not valid JSON)")
	assert.Nil(t, result, "expected nil result for empty file")
}

func TestReadHooksJSON_UnreadableFile(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("test not reliable when running as root")
	}

	dir := t.TempDir()
	hooksFile := filepath.Join(dir, "hooks.json")
	require.NoError(t, os.WriteFile(hooksFile, []byte(`{"PreToolUse": []}`), 0000))
	t.Cleanup(func() { os.Chmod(hooksFile, 0644) })

	result, err := readHooksJSON(hooksFile)
	require.Error(t, err, "expected error for unreadable hooks file")
	assert.Nil(t, result, "expected nil result for unreadable file")
}

func TestReadHooksJSON_EmptyObject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hooksFile := filepath.Join(dir, "hooks.json")
	require.NoError(t, os.WriteFile(hooksFile, []byte(`{}`), 0644))

	result, err := readHooksJSON(hooksFile)
	require.NoError(t, err, "unexpected error for empty object")
	require.NotNil(t, result, "expected non-nil result for valid empty object")
	assert.Empty(t, result)
}
