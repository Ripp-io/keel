package remote

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// FetchFile tests
// ---------------------------------------------------------------------------

func TestFetchFile_Downloads(t *testing.T) {
	disableSSRFForTest(t)

	content := "# Security Instructions\nDo not commit secrets."
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"file-etag-1"`)
		w.Header().Set("Content-Type", "text/markdown")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	url := ts.URL + "/configs/security.md"

	result, err := FetchFile(url, cacheBase, false)
	require.NoError(t, err)

	data, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
	assert.Equal(t, "security.md", filepath.Base(result))
}

func TestFetchFile_CachesResult(t *testing.T) {
	disableSSRFForTest(t)

	content := "cached content"
	etag := `"stable-file-etag"`
	requestCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	url := ts.URL + "/configs/mcp.json"

	// First fetch -- should download.
	result1, err := FetchFile(url, cacheBase, false)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)

	data, err := os.ReadFile(result1)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))

	// Second fetch -- should get 304, return cached path.
	result2, err := FetchFile(url, cacheBase, false)
	require.NoError(t, err)
	require.Equal(t, 2, requestCount)

	assert.Equal(t, result1, result2, "same path should be returned")

	data, err = os.ReadFile(result2)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestFetchFile_OfflineWithCache(t *testing.T) {
	disableSSRFForTest(t)

	content := "offline-available content"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	url := ts.URL + "/configs/instructions.md"

	// First fetch online to populate cache.
	_, err := FetchFile(url, cacheBase, false)
	require.NoError(t, err)

	// Now fetch offline -- should succeed from cache.
	result, err := FetchFile(url, cacheBase, true)
	require.NoError(t, err)

	data, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestFetchFile_OfflineNoCache(t *testing.T) {
	disableSSRFForTest(t)

	cacheBase := t.TempDir()
	url := "https://example.com/nonexistent/config.md"

	_, err := FetchFile(url, cacheBase, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no cached data")
	assert.Contains(t, err.Error(), "--offline")
}

func TestFetchFile_SSRFProtection(t *testing.T) {
	// Do NOT call disableSSRFForTest -- we want SSRF validation active.
	skipSSRFValidation.Store(false)
	t.Cleanup(func() { skipSSRFValidation.Store(false) })

	cacheBase := t.TempDir()

	tests := []struct {
		name string
		url  string
	}{
		{"loopback IPv4", "https://127.0.0.1/secret.md"},
		{"private 10.x", "https://10.0.0.1/secret.md"},
		{"private 192.168.x", "https://192.168.1.1/secret.md"},
		{"link-local", "https://169.254.169.254/metadata"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FetchFile(tt.url, cacheBase, false)
			require.Error(t, err, "expected SSRF error for %s", tt.url)
			assert.Contains(t, err.Error(), "private/loopback")
		})
	}
}

func TestFetchFile_RequiresHTTPS(t *testing.T) {
	skipSSRFValidation.Store(false)
	t.Cleanup(func() { skipSSRFValidation.Store(false) })

	cacheBase := t.TempDir()

	_, err := FetchFile("http://example.com/config.md", cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https://")
}

func TestFetchFile_HTTPError_404(t *testing.T) {
	disableSSRFForTest(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cacheBase := t.TempDir()

	_, err := FetchFile(ts.URL+"/missing.md", cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestFetchFile_ContentUpdated(t *testing.T) {
	disableSSRFForTest(t)

	currentContent := "version 1"
	currentETag := `"v1"`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == currentETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", currentETag)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(currentContent))
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	url := ts.URL + "/config.json"

	// First fetch.
	result, err := FetchFile(url, cacheBase, false)
	require.NoError(t, err)
	data, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, "version 1", string(data))

	// Update server content.
	currentContent = "version 2"
	currentETag = `"v2"`

	// Second fetch -- should get 200 with new content.
	result, err = FetchFile(url, cacheBase, false)
	require.NoError(t, err)
	data, err = os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, "version 2", string(data))
}

func TestFetchFile_UserAgent(t *testing.T) {
	disableSSRFForTest(t)

	var receivedUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("content"))
	}))
	defer ts.Close()

	cacheBase := t.TempDir()

	_, err := FetchFile(ts.URL+"/file.md", cacheBase, false)
	require.NoError(t, err)
	assert.Equal(t, "keel", receivedUA)
}

func TestFetchFile_FileSizeLimit(t *testing.T) {
	disableSSRFForTest(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		chunk := make([]byte, 1024*1024)
		var written int64
		target := int64(MaxFileSize + 2)
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

	_, err := FetchFile(ts.URL+"/huge.md", cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "size limit")
}
