package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadVarFile_Basic(t *testing.T) {
	content := "environment=staging\napi_url=https://staging.example.com\n"
	path := writeVarFile(t, content)

	vars, err := LoadVarFile(path)
	require.NoError(t, err)

	want := map[string]string{
		"environment": "staging",
		"api_url":     "https://staging.example.com",
	}
	assertVarsEqual(t, vars, want)
}

func TestLoadVarFile_CommentsAndBlankLines(t *testing.T) {
	content := `# This is a comment
environment=staging

# Another comment
api_url=https://staging.example.com

`
	path := writeVarFile(t, content)

	vars, err := LoadVarFile(path)
	require.NoError(t, err)

	want := map[string]string{
		"environment": "staging",
		"api_url":     "https://staging.example.com",
	}
	assertVarsEqual(t, vars, want)
}

func TestLoadVarFile_ValueWithEquals(t *testing.T) {
	content := "key_with_equals=value=with=equals\n"
	path := writeVarFile(t, content)

	vars, err := LoadVarFile(path)
	require.NoError(t, err)

	assert.Equal(t, "value=with=equals", vars["key_with_equals"])
}

func TestLoadVarFile_EmptyValue(t *testing.T) {
	content := "empty_key=\n"
	path := writeVarFile(t, content)

	vars, err := LoadVarFile(path)
	require.NoError(t, err)

	assert.Contains(t, vars, "empty_key")
	assert.Equal(t, "", vars["empty_key"])
}

func TestLoadVarFile_EmptyFile(t *testing.T) {
	path := writeVarFile(t, "")

	vars, err := LoadVarFile(path)
	require.NoError(t, err)

	assert.Empty(t, vars)
}

func TestLoadVarFile_OnlyComments(t *testing.T) {
	content := "# comment 1\n# comment 2\n"
	path := writeVarFile(t, content)

	vars, err := LoadVarFile(path)
	require.NoError(t, err)

	assert.Empty(t, vars)
}

func TestLoadVarFile_MissingFile(t *testing.T) {
	_, err := LoadVarFile("/nonexistent/path/vars.env")
	assert.Error(t, err)
}

func TestLoadVarFile_InvalidLine_NoEquals(t *testing.T) {
	content := "valid=good\ninvalid_no_equals\n"
	path := writeVarFile(t, content)

	_, err := LoadVarFile(path)
	assert.Error(t, err)
}

func TestLoadVarFile_InvalidLine_EmptyKey(t *testing.T) {
	content := "=value_without_key\n"
	path := writeVarFile(t, content)

	_, err := LoadVarFile(path)
	assert.Error(t, err)
}

func TestLoadVarFile_WhitespaceHandling(t *testing.T) {
	content := "  key_with_spaces  =value\n"
	path := writeVarFile(t, content)

	vars, err := LoadVarFile(path)
	require.NoError(t, err)

	assert.Equal(t, "value", vars["key_with_spaces"])
}

// --- helpers ---

func writeVarFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vars.env")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644), "writing var file")
	return path
}

func assertVarsEqual(t *testing.T, got, want map[string]string) {
	t.Helper()
	require.Len(t, got, len(want))
	for k, wv := range want {
		assert.Containsf(t, got, k, "missing key %q", k)
		assert.Equalf(t, wv, got[k], "key %q", k)
	}
}
