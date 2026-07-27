// Package testutil provides shared test helpers used across keel's test suite.
// This is a regular (non-test) package so it can be imported by any _test.go file.
// The Go linker strips it from production binaries since no non-test code imports it.
package testutil

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertFileContent reads a file and asserts its content matches expected.
func AssertFileContent(t testing.TB, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading %s", path)
	assert.Equal(t, expected, string(data), "content mismatch at %s", path)
}

// WriteFile creates parent directories and writes content to dir/rel.
func WriteFile(t testing.TB, dir, rel string, content []byte) {
	t.Helper()
	p := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755), "mkdir for %s", rel)
	require.NoError(t, os.WriteFile(p, content, 0644), "writing %s", rel)
}

// WriteJSON marshals v to indented JSON and writes it to path, creating
// parent directories as needed.
func WriteJSON(t testing.TB, path string, v any) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	data, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
}

// ReadJSONMap reads a JSON file and returns it as a map[string]any.
func ReadJSONMap(t testing.TB, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading %s", path)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m), "parsing %s", path)
	return m
}

// FileExists returns whether a file or directory exists at the given path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// AssertNoKeelKeys recursively checks that no "_keel" keys exist in data.
// Use this to verify that keel metadata has been stripped from compiled output.
func AssertNoKeelKeys(t testing.TB, data map[string]any) {
	t.Helper()
	assertNoKeelKeysRecursive(t, data, "")
}

func assertNoKeelKeysRecursive(t testing.TB, data map[string]any, path string) {
	t.Helper()
	for key, value := range data {
		fullPath := key
		if path != "" {
			fullPath = path + "." + key
		}
		assert.NotEqual(t, "_keel", key, "found _keel key at path %q -- should have been stripped", path)
		if nested, ok := value.(map[string]any); ok {
			assertNoKeelKeysRecursive(t, nested, fullPath)
		}
	}
}

// CreateTestTarGz creates a .tar.gz archive in dir containing the given files
// (key = path within archive, value = file content). Returns the path to the
// created archive file.
func CreateTestTarGz(t testing.TB, dir string, files map[string]string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(dir, "test-*.tar.gz")
	require.NoError(t, err, "creating temp tar.gz")
	defer func() { _ = tmpFile.Close() }()

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		// Ensure parent directory entries exist.
		d := filepath.Dir(name)
		if d != "." && d != "" {
			parts := strings.Split(d, "/")
			for i := range parts {
				dirPath := strings.Join(parts[:i+1], "/") + "/"
				_ = tw.WriteHeader(&tar.Header{
					Name:     dirPath,
					Typeflag: tar.TypeDir,
					Mode:     0755,
				})
			}
		}

		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}), "writing tar header for %q", name)
		_, err := tw.Write([]byte(content))
		require.NoError(t, err, "writing tar content for %q", name)
	}

	require.NoError(t, tw.Close(), "closing tar writer")
	require.NoError(t, gw.Close(), "closing gzip writer")

	return tmpFile.Name()
}

// CreateTestZip creates a .zip archive in dir containing the given files
// (key = path within archive, value = file content). Returns the path to the
// created archive file.
func CreateTestZip(t testing.TB, dir string, files map[string]string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(dir, "test-*.zip")
	require.NoError(t, err, "creating temp zip")
	defer func() { _ = tmpFile.Close() }()

	zw := zip.NewWriter(tmpFile)

	for name, content := range files {
		fw, err := zw.Create(name)
		require.NoError(t, err, "creating zip entry %q", name)
		_, err = fw.Write([]byte(content))
		require.NoError(t, err, "writing zip content for %q", name)
	}

	require.NoError(t, zw.Close(), "closing zip writer")

	return tmpFile.Name()
}
