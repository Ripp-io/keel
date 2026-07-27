package remote

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// ---------------------------------------------------------------------------
// Test helpers: create test archives
// ---------------------------------------------------------------------------

// createTestTarGzWithSymlink creates a .tar.gz that includes a symlink entry.
func createTestTarGzWithSymlink(t *testing.T, files map[string]string, linkName, linkTarget string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-symlink-*.tar.gz")
	require.NoError(t, err, "creating temp tar.gz")
	defer tmpFile.Close()

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}), "writing tar header for %q", name)
		_, err := tw.Write([]byte(content))
		require.NoError(t, err, "writing tar content for %q", name)
	}

	// Add symlink entry.
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     linkName,
		Linkname: linkTarget,
		Typeflag: tar.TypeSymlink,
		Mode:     0777,
	}), "writing symlink header")

	require.NoError(t, tw.Close(), "closing tar writer")
	require.NoError(t, gw.Close(), "closing gzip writer")

	return tmpFile.Name()
}

// createTestTarGzWithZipSlip creates a .tar.gz with a path traversal entry.
func createTestTarGzWithZipSlip(t *testing.T) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-zipslip-*.tar.gz")
	require.NoError(t, err, "creating temp tar.gz")
	defer tmpFile.Close()

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	// Normal file first.
	content := "normal"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "normal.txt",
		Size:     int64(len(content)),
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(content))
	require.NoError(t, err)

	// Malicious path traversal entry.
	evil := "evil content"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "../../etc/passwd",
		Size:     int64(len(evil)),
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(evil))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	return tmpFile.Name()
}

// createTestTarGzLargeFileCount creates a .tar.gz with more than MaxFileCount files.
func createTestTarGzLargeFileCount(t *testing.T, count int) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-filecount-*.tar.gz")
	require.NoError(t, err, "creating temp tar.gz")
	defer tmpFile.Close()

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	for i := 0; i <= count; i++ {
		name := fmt.Sprintf("file_%05d.txt", i)
		content := "x"
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}), "writing header %d", i)
		_, err := tw.Write([]byte(content))
		require.NoError(t, err, "writing content %d", i)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	return tmpFile.Name()
}

// createTestTarGzSingleRoot creates a .tar.gz where all files are under a
// single root directory (e.g., "policy-v2/...").
func createTestTarGzSingleRoot(t *testing.T, rootDir string, files map[string]string) string {
	t.Helper()
	prefixed := make(map[string]string, len(files))
	for name, content := range files {
		prefixed[rootDir+"/"+name] = content
	}
	return testutil.CreateTestTarGz(t, t.TempDir(), prefixed)
}

// createTestZipWithZipSlip creates a .zip with a path traversal entry.
func createTestZipWithZipSlip(t *testing.T) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-zipslip-*.zip")
	require.NoError(t, err, "creating temp zip")
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)

	// Normal file.
	fw, err := zw.Create("normal.txt")
	require.NoError(t, err)
	_, err = fw.Write([]byte("normal"))
	require.NoError(t, err)

	// Malicious path traversal entry -- use CreateHeader to set exact Name.
	header := &zip.FileHeader{
		Name:   "../../etc/passwd",
		Method: zip.Deflate,
	}
	ew, err := zw.CreateHeader(header)
	require.NoError(t, err, "creating evil entry")
	_, err = ew.Write([]byte("evil"))
	require.NoError(t, err)

	require.NoError(t, zw.Close())

	return tmpFile.Name()
}

// serveArchive returns an httptest handler that serves the given archive file
// with the specified ETag and Last-Modified headers, and supports conditional
// GET (304 responses).
func serveArchive(t *testing.T, archivePath, etag, lastModified string) http.HandlerFunc {
	t.Helper()
	data, err := os.ReadFile(archivePath)
	require.NoError(t, err, "reading archive for server")

	return func(w http.ResponseWriter, r *http.Request) {
		// Check conditional GET.
		if etag != "" && r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if lastModified != "" && r.Header.Get("If-Modified-Since") == lastModified {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		if lastModified != "" {
			w.Header().Set("Last-Modified", lastModified)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

// ---------------------------------------------------------------------------
// FetchHTTP tests
// ---------------------------------------------------------------------------

func TestFetchHTTP_TarGz(t *testing.T) {
	disableSSRFForTest(t)
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"keel.yml":         "version: 1",
		"instructions.md":  "# Hello",
		"settings/mcp.yml": "tools: []",
	})

	ts := httptest.NewServer(serveArchive(t, archivePath, `"test-etag"`, "Wed, 07 May 2026 10:00:00 GMT"))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/layers.tar.gz",
		Original: ts.URL + "/layers.tar.gz",
	}

	result, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)

	// Verify extracted files exist.
	for _, name := range []string{"keel.yml", "instructions.md", "settings/mcp.yml"} {
		assert.FileExists(t, filepath.Join(result, name))
	}

	// Verify keel.yml content.
	data, err := os.ReadFile(filepath.Join(result, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "version: 1", string(data))

	// Verify meta.json was written.
	hash := CacheKey(src.RawURL)
	metaPath := filepath.Join(cacheBase, "http", hash, "meta.json")
	meta, err := ReadHTTPMeta(metaPath)
	require.NoError(t, err)
	assert.Equal(t, `"test-etag"`, meta.ETag)
	assert.Equal(t, "tar.gz", meta.ArchiveFormat)
}

func TestFetchHTTP_Zip(t *testing.T) {
	disableSSRFForTest(t)
	archivePath := testutil.CreateTestZip(t, t.TempDir(), map[string]string{
		"keel.yml":    "version: 2",
		"settings.md": "# Config",
	})

	ts := httptest.NewServer(serveArchive(t, archivePath, `"zip-etag"`, ""))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/layers.zip",
		Original: ts.URL + "/layers.zip",
	}

	result, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(result, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "version: 2", string(data))
}

func TestFetchHTTP_ConditionalGet_304(t *testing.T) {
	disableSSRFForTest(t)
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"keel.yml": "original",
	})
	etag := `"stable-etag"`

	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		serveArchive(t, archivePath, etag, "")(w, r)
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/layers.tar.gz",
		Original: ts.URL + "/layers.tar.gz",
	}

	// First fetch -- should download and extract.
	result1, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)

	data, err := os.ReadFile(filepath.Join(result1, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(data))

	// Second fetch -- should get 304, no re-extraction.
	result2, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)
	require.Equal(t, 2, requestCount)

	assert.Equal(t, result1, result2, "same content directory expected")

	data, err = os.ReadFile(filepath.Join(result2, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(data))
}

func TestFetchHTTP_ConditionalGet_200(t *testing.T) {
	disableSSRFForTest(t)
	archive1 := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{"keel.yml": "version: 1"})
	archive2 := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{"keel.yml": "version: 2"})

	currentArchive := archive1
	currentETag := `"etag-v1"`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := os.ReadFile(currentArchive)
		if r.Header.Get("If-None-Match") == currentETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", currentETag)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/layers.tar.gz",
		Original: ts.URL + "/layers.tar.gz",
	}

	// First fetch.
	result, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(result, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "version: 1", string(data))

	// Change the archive on the server.
	currentArchive = archive2
	currentETag = `"etag-v2"`

	// Second fetch -- should get 200 with new content.
	result, err = FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)
	data, err = os.ReadFile(filepath.Join(result, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "version: 2", string(data))

	// Verify meta was updated.
	hash := CacheKey(src.RawURL)
	metaPath := filepath.Join(cacheBase, "http", hash, "meta.json")
	meta, err := ReadHTTPMeta(metaPath)
	require.NoError(t, err)
	assert.Equal(t, `"etag-v2"`, meta.ETag)
}

func TestFetchHTTP_Offline_CacheHit(t *testing.T) {
	disableSSRFForTest(t)
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"keel.yml": "cached content",
	})

	ts := httptest.NewServer(serveArchive(t, archivePath, "", ""))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/layers.tar.gz",
		Original: ts.URL + "/layers.tar.gz",
	}

	// First fetch online to populate cache.
	_, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)

	// Now fetch offline -- should succeed from cache.
	result, err := FetchHTTP(src, cacheBase, true)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(result, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "cached content", string(data))
}

func TestFetchHTTP_Offline_CacheMiss(t *testing.T) {
	disableSSRFForTest(t)
	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   "https://example.com/nonexistent.tar.gz",
		Original: "https://example.com/nonexistent.tar.gz",
	}

	_, err := FetchHTTP(src, cacheBase, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no cached data")
	assert.Contains(t, err.Error(), "--offline")
}

func TestFetchHTTP_HTTPError_404(t *testing.T) {
	disableSSRFForTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/missing.tar.gz",
		Original: ts.URL + "/missing.tar.gz",
	}

	_, err := FetchHTTP(src, cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestFetchHTTP_SingleRootUnwrap(t *testing.T) {
	disableSSRFForTest(t)
	archivePath := createTestTarGzSingleRoot(t, "policy-v2", map[string]string{
		"keel.yml":    "version: 3",
		"settings.md": "# Settings",
	})

	ts := httptest.NewServer(serveArchive(t, archivePath, "", ""))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/policy-v2.tar.gz",
		Original: ts.URL + "/policy-v2.tar.gz",
	}

	result, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)

	// Files should be directly in content/, not content/policy-v2/.
	data, err := os.ReadFile(filepath.Join(result, "keel.yml"))
	require.NoError(t, err, "keel.yml should be at root of content")
	assert.Equal(t, "version: 3", string(data))

	// The wrapper directory should not exist.
	assert.NoDirExists(t, filepath.Join(result, "policy-v2"),
		"wrapper directory should not exist after unwrapping")
}

// ---------------------------------------------------------------------------
// extractTarGz tests
// ---------------------------------------------------------------------------

func TestExtractTarGz_ZipSlip(t *testing.T) {
	t.Parallel()
	archivePath := createTestTarGzWithZipSlip(t)
	targetDir := t.TempDir()

	err := ExtractTarGz(archivePath, targetDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path escapes target directory")
}

func TestExtractTarGz_SizeLimit(t *testing.T) {
	t.Parallel()
	// Create an archive with a file larger than MaxFileSize.
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-large-*.tar.gz")
	require.NoError(t, err)
	defer tmpFile.Close()

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	largeSize := int64(MaxFileSize + 1)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "large.bin",
		Size:     largeSize,
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}))

	chunk := make([]byte, 1024)
	var written int64
	for written < largeSize {
		n := int64(len(chunk))
		if written+n > largeSize {
			n = largeSize - written
		}
		tw.Write(chunk[:n])
		written += n
	}

	tw.Close()
	gw.Close()

	targetDir := t.TempDir()
	err = ExtractTarGz(tmpFile.Name(), targetDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum file size")
}

func TestExtractTarGz_FileCountLimit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping large archive test in short mode")
	}
	archivePath := createTestTarGzLargeFileCount(t, MaxFileCount)
	targetDir := t.TempDir()

	err := ExtractTarGz(archivePath, targetDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum file count")
}

func TestExtractTarGz_SymlinksSkipped(t *testing.T) {
	t.Parallel()
	archivePath := createTestTarGzWithSymlink(t,
		map[string]string{"real.txt": "real content"},
		"link.txt", "real.txt",
	)
	targetDir := t.TempDir()

	require.NoError(t, ExtractTarGz(archivePath, targetDir))

	assert.FileExists(t, filepath.Join(targetDir, "real.txt"))

	// Symlink should NOT exist (silently skipped).
	_, err := os.Lstat(filepath.Join(targetDir, "link.txt"))
	assert.Error(t, err, "symlink should have been skipped")
}

func TestExtractTarGz_TotalSizeLimit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping large archive test in short mode")
	}
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-totalsize-*.tar.gz")
	require.NoError(t, err)
	defer tmpFile.Close()

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	fileSize := int64(MaxFileSize)
	numFiles := (int64(MaxTotalSize) / fileSize) + 2
	chunk := make([]byte, 1024)

	for i := int64(0); i < numFiles; i++ {
		name := fmt.Sprintf("chunk_%03d.bin", i)
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     fileSize,
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}))
		var written int64
		for written < fileSize {
			n := int64(len(chunk))
			if written+n > fileSize {
				n = fileSize - written
			}
			tw.Write(chunk[:n])
			written += n
		}
	}

	tw.Close()
	gw.Close()

	targetDir := t.TempDir()
	err = ExtractTarGz(tmpFile.Name(), targetDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum total extracted size")
}

// ---------------------------------------------------------------------------
// extractZip tests
// ---------------------------------------------------------------------------

func TestExtractZip_ZipSlip(t *testing.T) {
	t.Parallel()
	archivePath := createTestZipWithZipSlip(t)
	targetDir := t.TempDir()

	err := ExtractZip(archivePath, targetDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path escapes target directory")
}

func TestExtractZip_Basic(t *testing.T) {
	t.Parallel()
	archivePath := testutil.CreateTestZip(t, t.TempDir(), map[string]string{
		"keel.yml":       "version: 1",
		"sub/nested.txt": "nested content",
	})
	targetDir := t.TempDir()

	require.NoError(t, ExtractZip(archivePath, targetDir))

	data, err := os.ReadFile(filepath.Join(targetDir, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "version: 1", string(data))

	data, err = os.ReadFile(filepath.Join(targetDir, "sub", "nested.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested content", string(data))
}

// ---------------------------------------------------------------------------
// unwrapSingleRoot tests
// ---------------------------------------------------------------------------

func TestUnwrapSingleRoot_Unwraps(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	wrapper := filepath.Join(dir, "wrapper")
	require.NoError(t, os.MkdirAll(wrapper, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(wrapper, "file.txt"), []byte("hello"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(wrapper, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(wrapper, "subdir", "nested.txt"), []byte("nested"), 0644))

	require.NoError(t, unwrapSingleRoot(dir))

	// file.txt should be at root.
	data, err := os.ReadFile(filepath.Join(dir, "file.txt"))
	require.NoError(t, err, "file.txt should be at root")
	assert.Equal(t, "hello", string(data))

	// subdir/nested.txt should be at root level.
	data, err = os.ReadFile(filepath.Join(dir, "subdir", "nested.txt"))
	require.NoError(t, err, "subdir/nested.txt should be at root")
	assert.Equal(t, "nested", string(data))

	// Wrapper should be removed.
	assert.NoDirExists(t, wrapper, "wrapper directory should have been removed")
}

func TestUnwrapSingleRoot_NoUnwrap_MultipleEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("b"), 0644))

	require.NoError(t, unwrapSingleRoot(dir))

	assert.FileExists(t, filepath.Join(dir, "file1.txt"))
	assert.FileExists(t, filepath.Join(dir, "file2.txt"))
}

func TestUnwrapSingleRoot_NoUnwrap_SingleFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("a"), 0644))

	require.NoError(t, unwrapSingleRoot(dir))

	assert.FileExists(t, filepath.Join(dir, "file.txt"))
}

// ---------------------------------------------------------------------------
// detectArchiveFormat tests
// ---------------------------------------------------------------------------

func TestDetectArchiveFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/layers.tar.gz", "tar.gz"},
		{"https://example.com/layers.tgz", "tar.gz"},
		{"https://example.com/layers.TGZ", "tar.gz"},
		{"https://example.com/layers.TAR.GZ", "tar.gz"},
		{"https://example.com/layers.zip", "zip"},
		{"https://example.com/layers.ZIP", "zip"},
		{"https://example.com/layers.txt", ""},
		{"https://example.com/layers", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.url, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, DetectArchiveFormat(tt.url))
		})
	}
}

// ---------------------------------------------------------------------------
// hasContent tests
// ---------------------------------------------------------------------------

func TestHasContent(t *testing.T) {
	t.Parallel()
	t.Run("empty directory", func(t *testing.T) {
		t.Parallel()
		assert.False(t, hasContent(t.TempDir()))
	})

	t.Run("directory with file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
		assert.True(t, hasContent(dir))
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		t.Parallel()
		assert.False(t, hasContent("/nonexistent/path"))
	})
}

// ---------------------------------------------------------------------------
// File permissions tests
// ---------------------------------------------------------------------------

func TestExtractTarGz_FilePermissions(t *testing.T) {
	t.Parallel()
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"file.txt": "content",
	})
	targetDir := t.TempDir()

	require.NoError(t, ExtractTarGz(archivePath, targetDir))

	info, err := os.Stat(filepath.Join(targetDir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestExtractTarGz_DirPermissions(t *testing.T) {
	t.Parallel()
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"subdir/file.txt": "content",
	})
	targetDir := t.TempDir()

	require.NoError(t, ExtractTarGz(archivePath, targetDir))

	info, err := os.Stat(filepath.Join(targetDir, "subdir"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

// ---------------------------------------------------------------------------
// FetchHTTP with User-Agent header test
// ---------------------------------------------------------------------------

func TestFetchHTTP_UserAgent(t *testing.T) {
	disableSSRFForTest(t)
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{"keel.yml": "v1"})
	var receivedUA string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		data, _ := os.ReadFile(archivePath)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/layers.tar.gz",
		Original: ts.URL + "/layers.tar.gz",
	}

	_, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)
	assert.Equal(t, "keel", receivedUA)
}

// ---------------------------------------------------------------------------
// extractZip limit tests
// ---------------------------------------------------------------------------

// createTestZipLargeFileCount creates a .zip with more than MaxFileCount files.
func createTestZipLargeFileCount(t *testing.T, count int) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-filecount-*.zip")
	require.NoError(t, err, "creating temp zip")
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)

	for i := 0; i <= count; i++ {
		name := fmt.Sprintf("file_%05d.txt", i)
		fw, err := zw.Create(name)
		require.NoError(t, err, "creating zip entry %d", i)
		_, err = fw.Write([]byte("x"))
		require.NoError(t, err, "writing zip content %d", i)
	}

	require.NoError(t, zw.Close())

	return tmpFile.Name()
}

// createTestZipLargeFile creates a .zip with a single file whose
// uncompressed size exceeds MaxFileSize.
func createTestZipLargeFile(t *testing.T) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-largefile-*.zip")
	require.NoError(t, err, "creating temp zip")
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)

	header := &zip.FileHeader{
		Name:               "large.bin",
		Method:             zip.Store,
		UncompressedSize64: uint64(MaxFileSize + 1),
	}
	fw, err := zw.CreateHeader(header)
	require.NoError(t, err, "creating zip header")

	chunk := make([]byte, 1024)
	var written int64
	target := int64(MaxFileSize + 1)
	for written < target {
		n := int64(len(chunk))
		if written+n > target {
			n = target - written
		}
		fw.Write(chunk[:n])
		written += n
	}

	require.NoError(t, zw.Close())

	return tmpFile.Name()
}

// createTestZipTotalSizeExceeded creates a .zip with many files whose
// total uncompressed size exceeds MaxTotalSize.
func createTestZipTotalSizeExceeded(t *testing.T) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-totalsize-*.zip")
	require.NoError(t, err, "creating temp zip")
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)

	fileSize := int64(MaxFileSize)
	numFiles := (int64(MaxTotalSize) / fileSize) + 2

	chunk := make([]byte, 1024)
	for i := int64(0); i < numFiles; i++ {
		header := &zip.FileHeader{
			Name:               fmt.Sprintf("chunk_%03d.bin", i),
			Method:             zip.Store,
			UncompressedSize64: uint64(fileSize),
		}
		fw, err := zw.CreateHeader(header)
		require.NoError(t, err, "creating zip entry %d", i)
		var written int64
		for written < fileSize {
			n := int64(len(chunk))
			if written+n > fileSize {
				n = fileSize - written
			}
			fw.Write(chunk[:n])
			written += n
		}
	}

	require.NoError(t, zw.Close())

	return tmpFile.Name()
}

func TestExtractZip_FileCountLimit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping large archive test in short mode")
	}
	archivePath := createTestZipLargeFileCount(t, MaxFileCount)
	targetDir := t.TempDir()

	err := ExtractZip(archivePath, targetDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum file count")
}

func TestExtractZip_FileSizeLimit(t *testing.T) {
	t.Parallel()
	archivePath := createTestZipLargeFile(t)
	targetDir := t.TempDir()

	err := ExtractZip(archivePath, targetDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum file size")
}

func TestExtractZip_TotalSizeLimit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping large archive test in short mode")
	}
	archivePath := createTestZipTotalSizeExceeded(t)
	targetDir := t.TempDir()

	err := ExtractZip(archivePath, targetDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum total extracted size")
}

// ---------------------------------------------------------------------------
// Security: HTTP download size limit
// ---------------------------------------------------------------------------

func TestFetchHTTP_DownloadSizeLimit(t *testing.T) {
	disableSSRFForTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		chunk := make([]byte, 1024*1024)
		var written int64
		target := int64(MaxTotalSize + 2)
		for written < target {
			n := int64(len(chunk))
			if written+n > target {
				n = target - written
			}
			w.Write(chunk[:n])
			written += n
		}
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/huge.tar.gz",
		Original: ts.URL + "/huge.tar.gz",
	}

	_, err := FetchHTTP(src, cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "size limit")
}

// ---------------------------------------------------------------------------
// Security: CheckRedirect - HTTPS enforcement and SSRF
// ---------------------------------------------------------------------------

func TestFetchHTTP_RedirectSecurity(t *testing.T) {
	disableSSRFForTest(t)

	tests := []struct {
		name       string
		redirectTo string
		errMsg     string
	}{
		{
			name:       "redirect to HTTP rejected",
			redirectTo: "http://evil.example.com/payload.tar.gz",
			errMsg:     "insecure",
		},
		{
			name:       "redirect to private IP rejected",
			redirectTo: "https://127.0.0.1/payload.tar.gz",
			errMsg:     "private/loopback",
		},
		{
			name:       "redirect to link-local IP rejected",
			redirectTo: "https://169.254.169.254/metadata",
			errMsg:     "private/loopback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, tt.redirectTo, http.StatusFound)
			}))
			defer ts.Close()

			cacheBase := t.TempDir()
			src := ParsedSource{
				Type:     SourceArchive,
				RawURL:   ts.URL + "/redirect.tar.gz",
				Original: ts.URL + "/redirect.tar.gz",
			}

			_, err := FetchHTTP(src, cacheBase, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// ---------------------------------------------------------------------------
// Security: Unsupported archive format
// ---------------------------------------------------------------------------

func TestFetchHTTP_UnsupportedArchiveFormat(t *testing.T) {
	disableSSRFForTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not an archive"))
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/layers.rar",
		Original: ts.URL + "/layers.rar",
	}

	_, err := FetchHTTP(src, cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported archive format")
}

// ---------------------------------------------------------------------------
// replaceContent tests
// ---------------------------------------------------------------------------

func TestReplaceContent_Basic(t *testing.T) {
	t.Parallel()
	contentDir := filepath.Join(t.TempDir(), "content")
	srcDir := filepath.Join(t.TempDir(), "src")

	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "new.txt"), []byte("new"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "subdir", "nested.txt"), []byte("nested"), 0644))

	require.NoError(t, replaceContent(contentDir, srcDir))

	data, err := os.ReadFile(filepath.Join(contentDir, "new.txt"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
}

func TestReplaceContent_OverwritesExisting(t *testing.T) {
	t.Parallel()
	contentDir := filepath.Join(t.TempDir(), "content")
	srcDir := filepath.Join(t.TempDir(), "src")

	require.NoError(t, os.MkdirAll(contentDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(contentDir, "old.txt"), []byte("old"), 0644))

	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "new.txt"), []byte("new"), 0644))

	require.NoError(t, replaceContent(contentDir, srcDir))

	assert.NoFileExists(t, filepath.Join(contentDir, "old.txt"), "old content should be gone")
	assert.FileExists(t, filepath.Join(contentDir, "new.txt"))
}

// ---------------------------------------------------------------------------
// extractTarGz/extractZip error edge cases
// ---------------------------------------------------------------------------

func TestExtractTarGz_ErrorCases(t *testing.T) {
	t.Parallel()
	t.Run("corrupted archive", func(t *testing.T) {
		t.Parallel()
		tmpFile := filepath.Join(t.TempDir(), "corrupt.tar.gz")
		require.NoError(t, os.WriteFile(tmpFile, []byte("this is not a tar.gz"), 0644))

		err := ExtractTarGz(tmpFile, t.TempDir())
		assert.Error(t, err)
	})

	t.Run("nonexistent file", func(t *testing.T) {
		t.Parallel()
		err := ExtractTarGz("/nonexistent/path/archive.tar.gz", t.TempDir())
		assert.Error(t, err)
	})
}

func TestExtractZip_ErrorCases(t *testing.T) {
	t.Parallel()
	t.Run("corrupted archive", func(t *testing.T) {
		t.Parallel()
		tmpFile := filepath.Join(t.TempDir(), "corrupt.zip")
		require.NoError(t, os.WriteFile(tmpFile, []byte("this is not a zip"), 0644))

		err := ExtractZip(tmpFile, t.TempDir())
		assert.Error(t, err)
	})

	t.Run("nonexistent file", func(t *testing.T) {
		t.Parallel()
		err := ExtractZip("/nonexistent/path/archive.zip", t.TempDir())
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// unwrapSingleRoot edge cases
// ---------------------------------------------------------------------------

func TestUnwrapSingleRoot_NonexistentDir(t *testing.T) {
	t.Parallel()
	err := unwrapSingleRoot("/nonexistent/path")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// validateHost: SSRF IP range tests
// ---------------------------------------------------------------------------

func TestValidateHost_IPRanges(t *testing.T) {
	skipSSRFValidation.Store(false)
	t.Cleanup(func() { skipSSRFValidation.Store(false) })

	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "IPv6 loopback ::1", rawURL: "https://[::1]/path", wantErr: true},
		{name: "RFC1918 10.x.x.x", rawURL: "https://10.0.0.1/path", wantErr: true},
		{name: "RFC1918 172.16.x.x", rawURL: "https://172.16.0.1/path", wantErr: true},
		{name: "RFC1918 192.168.x.x", rawURL: "https://192.168.1.1/path", wantErr: true},
		{name: "IPv6 link-local fe80::1", rawURL: "https://[fe80::1]/path", wantErr: true},
		{name: "valid public IP 8.8.8.8", rawURL: "https://8.8.8.8/path", wantErr: false},
		{name: "DNS hostname passes", rawURL: "https://example.com/path", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHost(tt.rawURL)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "private/loopback")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateRedirect: comprehensive scheme + SSRF tests
// ---------------------------------------------------------------------------

func TestValidateRedirect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
		errMsg  string
	}{
		{name: "HTTPS valid", rawURL: "https://example.com/path", wantErr: false},
		{name: "HTTP insecure", rawURL: "http://example.com/path", wantErr: true, errMsg: "insecure"},
		{name: "loopback 127.0.0.1", rawURL: "https://127.0.0.1/path", wantErr: true, errMsg: "private/loopback"},
		{name: "private 10.0.0.1", rawURL: "https://10.0.0.1/path", wantErr: true, errMsg: "private/loopback"},
		{name: "IPv6 loopback", rawURL: "https://[::1]/path", wantErr: true, errMsg: "private/loopback"},
		{name: "IPv6 link-local", rawURL: "https://[fe80::1]/path", wantErr: true, errMsg: "private/loopback"},
		{name: "link-local 169.254", rawURL: "https://169.254.169.254/metadata", wantErr: true, errMsg: "private/loopback"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateRedirect(tt.rawURL)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Conditional GET: Last-Modified only (no ETag)
// ---------------------------------------------------------------------------

func TestFetchHTTP_ConditionalGet_LastModifiedOnly(t *testing.T) {
	disableSSRFForTest(t)
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"keel.yml": "last-modified-content",
	})
	lastModified := "Thu, 07 May 2026 12:00:00 GMT"

	requestCount := 0
	var receivedIfModSince string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		receivedIfModSince = r.Header.Get("If-Modified-Since")
		serveArchive(t, archivePath, "", lastModified)(w, r)
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/layers.tar.gz",
		Original: ts.URL + "/layers.tar.gz",
	}

	// First fetch: should download (200), no conditional header sent.
	result1, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)
	assert.Empty(t, receivedIfModSince, "first request should not send If-Modified-Since")

	data, err := os.ReadFile(filepath.Join(result1, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "last-modified-content", string(data))

	// Verify meta.json has Last-Modified but no ETag.
	hash := CacheKey(src.RawURL)
	metaPath := filepath.Join(cacheBase, "http", hash, "meta.json")
	meta, err := ReadHTTPMeta(metaPath)
	require.NoError(t, err)
	assert.Empty(t, meta.ETag)
	assert.Equal(t, lastModified, meta.LastModified)

	// Second fetch: should send If-Modified-Since and get 304.
	result2, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)
	require.Equal(t, 2, requestCount)
	assert.Equal(t, lastModified, receivedIfModSince)

	assert.Equal(t, result1, result2, "same path expected after 304")

	data, err = os.ReadFile(filepath.Join(result2, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "last-modified-content", string(data))
}
