package remote

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FetchFile downloads a single file from a URL and caches it locally.
// Returns the path to the cached file. Uses conditional GET for freshness.
// Only HTTPS URLs are accepted. Archives are not extracted — the response
// body is saved as-is.
func FetchFile(rawURL, cacheBase string, offline bool) (string, error) {
	// Require HTTPS. The skipSSRFValidation flag allows tests using
	// httptest.NewServer (which binds to http://127.0.0.1) to bypass this.
	if !skipSSRFValidation.Load() {
		lower := strings.ToLower(rawURL)
		if !strings.HasPrefix(lower, "https://") {
			return "", fmt.Errorf("insecure URL not allowed: %q (use https://)", rawURL)
		}
	}

	// SSRF protection: validate the host before any network call.
	if err := validateHost(rawURL); err != nil {
		return "", err
	}

	dirs, meta, err := setupFileCache(rawURL, cacheBase)
	if err != nil {
		return "", err
	}

	lockFile, err := AcquireLock(dirs.hashDir)
	if err != nil {
		return "", fmt.Errorf("locking cache for %s: %w", rawURL, err)
	}
	defer ReleaseLock(lockFile)

	filename := filenameFromURL(rawURL)
	cachedPath := filepath.Join(dirs.contentDir, filename)

	if offline {
		return checkFileCache(rawURL, cachedPath)
	}

	return fetchAndSaveFile(rawURL, dirs, meta, cachedPath)
}

// filenameFromURL extracts the last path segment from a URL as the filename.
// Falls back to "download" if the URL path has no usable segment.
func filenameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "download"
	}
	base := filepath.Base(parsed.Path)
	if base == "" || base == "." || base == "/" {
		return "download"
	}
	return base
}

// setupFileCache computes the cache key, creates directory structure, and reads
// existing metadata. Returns the cache directories and any existing metadata.
func setupFileCache(rawURL, cacheBase string) (httpCacheDirs, HTTPMeta, error) {
	hash := CacheKey(rawURL)

	hashDir, err := EnsureCacheDir(cacheBase, "files", hash)
	if err != nil {
		return httpCacheDirs{}, HTTPMeta{}, fmt.Errorf("setting up file cache for %s: %w", rawURL, err)
	}

	dirs := httpCacheDirs{
		hashDir:    hashDir,
		contentDir: filepath.Join(hashDir, "content"),
		metaPath:   filepath.Join(hashDir, "meta.json"),
	}

	// Read existing meta.json if it exists (ignore error for missing file).
	meta, _ := ReadHTTPMeta(dirs.metaPath)

	return dirs, meta, nil
}

// checkFileCache returns the cached file path in offline mode, or an error if
// no cached data is available.
func checkFileCache(rawURL, cachedPath string) (string, error) {
	if _, err := os.Stat(cachedPath); err == nil {
		return cachedPath, nil
	}
	return "", fmt.Errorf("no cached data for %s, run without --offline first", rawURL)
}

// fetchAndSaveFile performs the HTTP conditional GET, downloads the file if
// needed, and updates the cache metadata. Returns the path to the cached file.
func fetchAndSaveFile(rawURL string, dirs httpCacheDirs, meta HTTPMeta, cachedPath string) (string, error) {
	resp, err := doConditionalGet(rawURL, meta)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNotModified:
		meta.LastFetched = time.Now().UTC()
		_ = WriteMeta(dirs.metaPath, meta)
		return cachedPath, nil

	case http.StatusOK:
		return downloadFile(rawURL, dirs, resp, cachedPath)

	default:
		return "", fmt.Errorf("fetching %s: HTTP %d", rawURL, resp.StatusCode)
	}
}

// downloadFile saves the response body to the cached file path.
func downloadFile(rawURL string, dirs httpCacheDirs, resp *http.Response, cachedPath string) (string, error) {
	// Write to a temp file first, then rename for atomicity.
	tmpFile, err := os.CreateTemp(dirs.hashDir, "download-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file for %s: %w", rawURL, err)
	}
	tmpPath := tmpFile.Name()

	limitedBody := io.LimitReader(resp.Body, MaxFileSize+1) // +1 to detect overflow
	size, err := io.Copy(tmpFile, limitedBody)
	if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("downloading %s: %w", rawURL, err)
	}
	if size > MaxFileSize {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("file %s exceeds %d MB size limit", rawURL, MaxFileSize/(1024*1024))
	}

	// Ensure the content directory exists.
	if err := os.MkdirAll(dirs.contentDir, 0755); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("creating content dir for %s: %w", rawURL, err)
	}

	// Move temp file to final location.
	if err := os.Rename(tmpPath, cachedPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("moving downloaded file for %s: %w", rawURL, err)
	}

	// Write metadata.
	meta := HTTPMeta{
		URL:              rawURL,
		LastFetched:      time.Now().UTC(),
		ETag:             resp.Header.Get("ETag"),
		LastModified:     resp.Header.Get("Last-Modified"),
		ContentSizeBytes: size,
	}
	if err := WriteMeta(dirs.metaPath, meta); err != nil {
		return "", fmt.Errorf("writing meta for %s: %w", rawURL, err)
	}

	return cachedPath, nil
}
