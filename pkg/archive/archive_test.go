package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// ---------------------------------------------------------------------------
// tar.gz tests
// ---------------------------------------------------------------------------

func TestCreateTarGz_RoundTrip(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	// Create a few files with known content.
	testutil.WriteFile(t, srcDir, "hello.txt", []byte("hello world"))
	testutil.WriteFile(t, srcDir, filepath.Join("sub", "nested.txt"), []byte("nested content"))

	destPath := filepath.Join(t.TempDir(), "out.tar.gz")

	require.NoError(t, CreateTarGz(srcDir, destPath))

	// Read back the archive and verify entries.
	entries := readTarGzEntries(t, destPath)

	want := []string{"hello.txt", "sub/nested.txt"}
	sort.Strings(entries)
	sort.Strings(want)

	require.Equal(t, want, entries)

	// Verify content round-trip by extracting to a temp dir.
	extractDir := t.TempDir()
	extractTarGzTo(t, destPath, extractDir)

	testutil.AssertFileContent(t, filepath.Join(extractDir, "hello.txt"), "hello world")
	testutil.AssertFileContent(t, filepath.Join(extractDir, "sub", "nested.txt"), "nested content")
}

func TestCreateTarGz_SkipsSymlinks(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	testutil.WriteFile(t, srcDir, "real.txt", []byte("real"))

	// Create a symlink.
	symPath := filepath.Join(srcDir, "link.txt")
	if err := os.Symlink(filepath.Join(srcDir, "real.txt"), symPath); err != nil {
		t.Skipf("cannot create symlinks on this platform: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "out.tar.gz")

	require.NoError(t, CreateTarGz(srcDir, destPath))

	entries := readTarGzEntries(t, destPath)

	// Only real.txt should be in the archive.
	require.Len(t, entries, 1, "symlink should be skipped")
	assert.Equal(t, "real.txt", entries[0])
}

func TestCreateTarGz_RelativePaths(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	testutil.WriteFile(t, srcDir, "a.txt", []byte("a"))
	testutil.WriteFile(t, srcDir, filepath.Join("deep", "b.txt"), []byte("b"))

	destPath := filepath.Join(t.TempDir(), "out.tar.gz")

	require.NoError(t, CreateTarGz(srcDir, destPath))

	// Verify no entry starts with / or contains the srcDir path.
	entries := readTarGzEntries(t, destPath)
	for _, e := range entries {
		assert.False(t, filepath.IsAbs(e), "entry %q is absolute, want relative", e)
		assert.False(t, len(e) > 0 && e[0] == '/', "entry %q starts with /, want relative path", e)
	}
}

func TestCreateTarGz_EmptyDir(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	destPath := filepath.Join(t.TempDir(), "out.tar.gz")

	require.NoError(t, CreateTarGz(srcDir, destPath))

	// Should produce a valid archive with zero entries.
	entries := readTarGzEntries(t, destPath)
	assert.Empty(t, entries, "expected 0 entries for empty source dir")

	// File should exist and be non-empty (gzip header at minimum).
	info, err := os.Stat(destPath)
	require.NoError(t, err)
	assert.NotZero(t, info.Size(), "archive file is empty")
}

func TestCreateTarGz_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	testutil.WriteFile(t, srcDir, "file.txt", []byte("content"))

	// destPath has a parent that does not exist yet.
	destPath := filepath.Join(t.TempDir(), "deep", "nested", "out.tar.gz")

	require.NoError(t, CreateTarGz(srcDir, destPath))

	_, err := os.Stat(destPath)
	require.NoError(t, err, "archive not created at %q", destPath)
}

// ---------------------------------------------------------------------------
// zip tests
// ---------------------------------------------------------------------------

func TestCreateZip_RoundTrip(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	testutil.WriteFile(t, srcDir, "hello.txt", []byte("hello world"))
	testutil.WriteFile(t, srcDir, filepath.Join("sub", "nested.txt"), []byte("nested content"))

	destPath := filepath.Join(t.TempDir(), "out.zip")

	require.NoError(t, CreateZip(srcDir, destPath))

	entries := readZipEntries(t, destPath)

	want := []string{"hello.txt", "sub/nested.txt"}
	sort.Strings(entries)
	sort.Strings(want)

	require.Equal(t, want, entries)

	// Verify content round-trip.
	extractDir := t.TempDir()
	extractZipTo(t, destPath, extractDir)

	testutil.AssertFileContent(t, filepath.Join(extractDir, "hello.txt"), "hello world")
	testutil.AssertFileContent(t, filepath.Join(extractDir, "sub", "nested.txt"), "nested content")
}

func TestCreateZip_NestedDirs(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	testutil.WriteFile(t, srcDir, filepath.Join("a", "b", "c", "deep.txt"), []byte("deep"))
	testutil.WriteFile(t, srcDir, filepath.Join("a", "sibling.txt"), []byte("sibling"))

	destPath := filepath.Join(t.TempDir(), "out.zip")

	require.NoError(t, CreateZip(srcDir, destPath))

	entries := readZipEntries(t, destPath)
	sort.Strings(entries)

	want := []string{"a/b/c/deep.txt", "a/sibling.txt"}
	require.Equal(t, want, entries)
}

func TestCreateZip_SkipsSymlinks(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	testutil.WriteFile(t, srcDir, "real.txt", []byte("real"))

	symPath := filepath.Join(srcDir, "link.txt")
	if err := os.Symlink(filepath.Join(srcDir, "real.txt"), symPath); err != nil {
		t.Skipf("cannot create symlinks on this platform: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "out.zip")

	require.NoError(t, CreateZip(srcDir, destPath))

	entries := readZipEntries(t, destPath)
	require.Len(t, entries, 1, "symlink should be skipped")
	assert.Equal(t, "real.txt", entries[0])
}

func TestCreateZip_EmptyDir(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	destPath := filepath.Join(t.TempDir(), "out.zip")

	require.NoError(t, CreateZip(srcDir, destPath))

	entries := readZipEntries(t, destPath)
	assert.Empty(t, entries, "expected 0 entries for empty source dir")
}

func TestCreateZip_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	testutil.WriteFile(t, srcDir, "file.txt", []byte("content"))

	destPath := filepath.Join(t.TempDir(), "deep", "nested", "out.zip")

	require.NoError(t, CreateZip(srcDir, destPath))

	_, err := os.Stat(destPath)
	require.NoError(t, err, "archive not created at %q", destPath)
}

// ---------------------------------------------------------------------------
// Error and edge case tests
// ---------------------------------------------------------------------------

func TestCreateTarGz_NonexistentSrcDir(t *testing.T) {
	t.Parallel()
	destPath := filepath.Join(t.TempDir(), "out.tar.gz")
	err := CreateTarGz("/nonexistent/dir", destPath)
	require.Error(t, err, "expected error for nonexistent source directory")
}

func TestCreateZip_NonexistentSrcDir(t *testing.T) {
	t.Parallel()
	destPath := filepath.Join(t.TempDir(), "out.zip")
	err := CreateZip("/nonexistent/dir", destPath)
	require.Error(t, err, "expected error for nonexistent source directory")
}

func TestCreateTarGz_EmptyFiles(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	testutil.WriteFile(t, srcDir, "empty.txt", []byte(""))
	testutil.WriteFile(t, srcDir, "notempty.txt", []byte("content"))

	destPath := filepath.Join(t.TempDir(), "out.tar.gz")
	require.NoError(t, CreateTarGz(srcDir, destPath))

	extractDir := t.TempDir()
	extractTarGzTo(t, destPath, extractDir)
	testutil.AssertFileContent(t, filepath.Join(extractDir, "empty.txt"), "")
	testutil.AssertFileContent(t, filepath.Join(extractDir, "notempty.txt"), "content")
}

func TestCreateZip_EmptyFiles(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	testutil.WriteFile(t, srcDir, "empty.txt", []byte(""))
	testutil.WriteFile(t, srcDir, "notempty.txt", []byte("content"))

	destPath := filepath.Join(t.TempDir(), "out.zip")
	require.NoError(t, CreateZip(srcDir, destPath))

	extractDir := t.TempDir()
	extractZipTo(t, destPath, extractDir)
	testutil.AssertFileContent(t, filepath.Join(extractDir, "empty.txt"), "")
	testutil.AssertFileContent(t, filepath.Join(extractDir, "notempty.txt"), "content")
}

func TestCreateTarGz_DeeplyNested(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	// 30 levels deep
	deep := srcDir
	for i := range 30 {
		deep = filepath.Join(deep, fmt.Sprintf("d%d", i))
	}
	testutil.WriteFile(t, deep, "leaf.txt", []byte("deep leaf"))

	destPath := filepath.Join(t.TempDir(), "out.tar.gz")
	require.NoError(t, CreateTarGz(srcDir, destPath))

	entries := readTarGzEntries(t, destPath)
	require.Len(t, entries, 1)

	extractDir := t.TempDir()
	extractTarGzTo(t, destPath, extractDir)
	testutil.AssertFileContent(t, filepath.Join(extractDir, entries[0]), "deep leaf")
}

func TestCreateZip_DeeplyNested(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	deep := srcDir
	for i := range 30 {
		deep = filepath.Join(deep, fmt.Sprintf("d%d", i))
	}
	testutil.WriteFile(t, deep, "leaf.txt", []byte("deep leaf"))

	destPath := filepath.Join(t.TempDir(), "out.zip")
	require.NoError(t, CreateZip(srcDir, destPath))

	entries := readZipEntries(t, destPath)
	require.Len(t, entries, 1)

	extractDir := t.TempDir()
	extractZipTo(t, destPath, extractDir)
	testutil.AssertFileContent(t, filepath.Join(extractDir, entries[0]), "deep leaf")
}

func TestCreateTarGz_UnreadableDestDir(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	testutil.WriteFile(t, srcDir, "file.txt", []byte("content"))

	// Use a path with an invalid parent that can't be created
	destPath := filepath.Join("/dev/null", "impossible", "out.tar.gz")
	err := CreateTarGz(srcDir, destPath)
	require.Error(t, err, "expected error for unwritable destination")
}

func TestCreateZip_UnreadableDestDir(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	testutil.WriteFile(t, srcDir, "file.txt", []byte("content"))

	destPath := filepath.Join("/dev/null", "impossible", "out.zip")
	err := CreateZip(srcDir, destPath)
	require.Error(t, err, "expected error for unwritable destination")
}

func TestCreateTarGz_ManyFiles(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	for i := range 50 {
		testutil.WriteFile(t, srcDir, fmt.Sprintf("file_%03d.txt", i), []byte(fmt.Sprintf("content %d", i)))
	}

	destPath := filepath.Join(t.TempDir(), "out.tar.gz")
	require.NoError(t, CreateTarGz(srcDir, destPath))

	entries := readTarGzEntries(t, destPath)
	require.Len(t, entries, 50)
}

func TestCreateZip_ManyFiles(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	for i := range 50 {
		testutil.WriteFile(t, srcDir, fmt.Sprintf("file_%03d.txt", i), []byte(fmt.Sprintf("content %d", i)))
	}

	destPath := filepath.Join(t.TempDir(), "out.zip")
	require.NoError(t, CreateZip(srcDir, destPath))

	entries := readZipEntries(t, destPath)
	require.Len(t, entries, 50)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// readTarGzEntries opens a tar.gz archive and returns all entry names (files only).
func readTarGzEntries(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err, "opening %q", path)
	defer f.Close()

	gz, err := gzip.NewReader(f)
	require.NoError(t, err, "gzip reader for %q", path)
	defer gz.Close()

	tr := tar.NewReader(gz)
	var entries []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "reading tar entry")
		if hdr.Typeflag == tar.TypeReg {
			entries = append(entries, hdr.Name)
		}
	}
	return entries
}

// extractTarGzTo extracts a tar.gz archive into destDir for verification.
func extractTarGzTo(t *testing.T, archivePath, destDir string) {
	t.Helper()
	f, err := os.Open(archivePath)
	require.NoError(t, err, "opening %q", archivePath)
	defer f.Close()

	gz, err := gzip.NewReader(f)
	require.NoError(t, err, "gzip reader")
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "reading tar entry")
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		outPath := filepath.Join(destDir, hdr.Name)
		require.NoError(t, os.MkdirAll(filepath.Dir(outPath), 0755), "creating dir")
		outFile, err := os.Create(outPath)
		require.NoError(t, err, "creating file")
		_, err = io.Copy(outFile, tr)
		outFile.Close()
		require.NoError(t, err, "writing file")
	}
}

// readZipEntries opens a zip archive and returns all entry names (files only).
func readZipEntries(t *testing.T, path string) []string {
	t.Helper()
	r, err := zip.OpenReader(path)
	require.NoError(t, err, "opening zip %q", path)
	defer r.Close()

	var entries []string
	for _, f := range r.File {
		if !f.FileInfo().IsDir() {
			entries = append(entries, f.Name)
		}
	}
	return entries
}

// extractZipTo extracts a zip archive into destDir for verification.
func extractZipTo(t *testing.T, archivePath, destDir string) {
	t.Helper()
	r, err := zip.OpenReader(archivePath)
	require.NoError(t, err, "opening zip %q", archivePath)
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		outPath := filepath.Join(destDir, f.Name)
		require.NoError(t, os.MkdirAll(filepath.Dir(outPath), 0755), "creating dir")
		rc, err := f.Open()
		require.NoError(t, err, "opening zip entry")
		outFile, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			require.NoError(t, err, "creating file")
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		require.NoError(t, err, "writing file")
	}
}
