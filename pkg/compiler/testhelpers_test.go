package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// testdataDir returns the absolute path to the testdata directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "unable to determine test file location")
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// allLayers is the canonical 7-layer enterprise stack order.
var allLayers = []string{"base", "company", "org", "solution", "team", "project", "user"}

// buildTestLayers constructs a layer slice from named subdirectories under
// testdata/<subdir>, auto-detecting format via config.DetectLayerFormat.
func buildTestLayers(t *testing.T, subdir string, names ...string) []config.Layer {
	t.Helper()
	td := testdataDir(t)
	var layers []config.Layer
	for _, name := range names {
		p := filepath.Join(td, subdir, name)
		_, err := os.Stat(p)
		require.NoError(t, err, "layer %q not found at %s", name, p)
		format, version := config.DetectLayerFormat(p)
		layers = append(layers, config.Layer{
			Path:    p,
			Type:    name,
			Format:  format,
			Version: version,
		})
	}
	return layers
}

// assertFileMatchesExpected compares an output file against an expected file.
func assertFileMatchesExpected(t *testing.T, outputDir, relPath, expectedPath string) {
	t.Helper()
	gotPath := filepath.Join(outputDir, relPath)
	got, err := os.ReadFile(gotPath)
	require.NoError(t, err, "reading output %q", relPath)

	expected, err := os.ReadFile(expectedPath)
	require.NoError(t, err, "reading expected %q", expectedPath)

	assert.Equal(t, string(expected), string(got), "%s content mismatch", relPath)
}

// assertJSONMatchesExpected compares JSON output files by parsing and re-marshaling
// to normalize formatting differences.
func assertJSONMatchesExpected(t *testing.T, outputDir, relPath, expectedPath string) {
	t.Helper()
	gotPath := filepath.Join(outputDir, relPath)

	gotData, err := os.ReadFile(gotPath)
	require.NoError(t, err, "reading output %q", relPath)

	expectedData, err := os.ReadFile(expectedPath)
	require.NoError(t, err, "reading expected %q", expectedPath)

	var gotMap, expectedMap map[string]any
	err = json.Unmarshal(gotData, &gotMap)
	require.NoError(t, err, "parsing output JSON %q: content: %s", relPath, string(gotData))
	err = json.Unmarshal(expectedData, &expectedMap)
	require.NoError(t, err, "parsing expected JSON %q", expectedPath)

	gotNorm, _ := json.MarshalIndent(gotMap, "", "  ")
	expectedNorm, _ := json.MarshalIndent(expectedMap, "", "  ")

	assert.Equal(t, string(expectedNorm), string(gotNorm), "%s JSON mismatch", relPath)
}

