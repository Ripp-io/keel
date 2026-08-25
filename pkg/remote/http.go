package remote

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// Size and count limits for archive extraction. Exported so that callers
// outside pkg/remote (e.g., pkg/compiler/deps.go) use the same limits.
const (
	MaxTotalSize = 500 * 1024 * 1024 // 500 MB total extracted size
	MaxFileSize  = 50 * 1024 * 1024  // 50 MB per individual file
	MaxFileCount = 10_000            // Maximum number of files in an archive
)

// httpCacheDirs holds the directory paths used by the HTTP cache for a given source.
type httpCacheDirs struct {
	hashDir    string
	contentDir string
	metaPath   string
}

// FetchHTTP resolves an HTTP archive source to a local directory path.
// On first access: downloads and extracts the archive.
// On subsequent access: uses conditional GET (If-None-Match / If-Modified-Since).
// Returns the local path to the extracted layer directory.
func FetchHTTP(src ParsedSource, cacheBase string, offline bool) (string, error) {
	// SSRF protection: validate the initial request URL against loopback,
	// private, and link-local IP addresses before any network call.
	// skipSSRFValidation allows tests using httptest.NewServer (which binds
	// to 127.0.0.1) to bypass only the initial URL check; redirect
	// validation in CheckRedirect remains active.
	if !skipSSRFValidation.Load() {
		if err := validateHost(src.RawURL); err != nil {
			return "", err
		}
	}

	dirs, meta, err := setupHTTPCache(src, cacheBase)
	if err != nil {
		return "", err
	}

	lockFile, err := AcquireLock(dirs.hashDir)
	if err != nil {
		return "", fmt.Errorf("locking cache for %s: %w", src.RawURL, err)
	}
	defer ReleaseLock(lockFile)

	if offline {
		return checkHTTPCache(src, dirs)
	}

	return fetchAndExtract(src, dirs, meta)
}

// setupHTTPCache computes the cache key, creates directory structure, and reads
// existing metadata. Returns the cache directories and any existing metadata.
func setupHTTPCache(src ParsedSource, cacheBase string) (httpCacheDirs, HTTPMeta, error) {
	hash := CacheKey(src.RawURL)

	hashDir, err := EnsureCacheDir(cacheBase, "http", hash)
	if err != nil {
		return httpCacheDirs{}, HTTPMeta{}, fmt.Errorf("setting up cache for %s: %w", src.RawURL, err)
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

// checkHTTPCache returns the cached content directory in offline mode, or an
// error if no cached data is available. Performs integrity checking.
func checkHTTPCache(src ParsedSource, dirs httpCacheDirs) (string, error) {
	if !hasContent(dirs.contentDir) {
		return "", fmt.Errorf("no cached data for %s, run without --offline first", src.RawURL)
	}

	// Read meta for integrity checking.
	meta, err := ReadHTTPMeta(dirs.metaPath)
	if err != nil {
		slog.Warn("cache meta missing in offline mode, using cached content without verification", "url", src.RawURL)
		return dirs.contentDir, nil
	}

	// Verify cache integrity.
	if !verifyHTTPCacheIntegrity(dirs, meta) {
		return "", fmt.Errorf("cache integrity check failed for %s (run without --offline to re-fetch)", src.RawURL)
	}

	// Warn if cache is expired in offline mode.
	if isCacheExpired(meta, defaultCacheTTL) {
		slog.Warn("using expired cache entry in offline mode", "url", src.RawURL,
			"age", time.Since(meta.LastFetched).Round(time.Hour))
	}

	return dirs.contentDir, nil
}

// fetchAndExtract performs the HTTP conditional GET, downloads the archive if
// needed, extracts it, and updates the cache metadata. Returns the content directory.
// Checks cache integrity before using cached content.
func fetchAndExtract(src ParsedSource, dirs httpCacheDirs, meta HTTPMeta) (string, error) {
	// Check cache integrity before using or updating.
	if hasContent(dirs.contentDir) && !verifyHTTPCacheIntegrity(dirs, meta) {
		slog.Info("cache integrity check failed, re-fetching", "url", src.RawURL)
		// Force re-download by clearing meta ETag/LastModified to prevent conditional GET.
		meta.ETag = ""
		meta.LastModified = ""
	}

	resp, err := doConditionalGet(src.RawURL, meta)
	if err != nil {
		// Network error in online mode: use stale cache if available and valid.
		if hasContent(dirs.contentDir) && meta.ContentHash != "" && verifyHTTPCacheIntegrity(dirs, meta) {
			slog.Warn("network fetch failed, using stale cache", "url", src.RawURL, "error", err)
			return dirs.contentDir, nil
		}
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNotModified:
		meta.LastFetched = time.Now().UTC()
		_ = WriteMeta(dirs.metaPath, meta)
		return dirs.contentDir, nil

	case http.StatusOK:
		return downloadAndExtract(src, dirs, resp)

	default:
		return "", fmt.Errorf("fetching %s: HTTP %d", src.RawURL, resp.StatusCode)
	}
}

// doConditionalGet builds and executes an HTTP GET with conditional headers
// (If-None-Match, If-Modified-Since) based on cached metadata. Uses the
// shared HardenedClient which validates redirect targets for scheme safety
// and SSRF, and includes DNS rebinding protection.
func doConditionalGet(rawURL string, meta HTTPMeta) (*http.Response, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "keel")
	if meta.ETag != "" {
		req.Header.Set("If-None-Match", meta.ETag)
	}
	if meta.LastModified != "" {
		req.Header.Set("If-Modified-Since", meta.LastModified)
	}

	client := HardenedClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	return resp, nil
}

// downloadAndExtract saves the response body to a temp file, detects the archive
// format, extracts it, and swaps the content into the cache directory.
func downloadAndExtract(src ParsedSource, dirs httpCacheDirs, resp *http.Response) (string, error) {
	archivePath, size, err := downloadArchive(src.RawURL, dirs.hashDir, resp)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(archivePath) }()

	format := DetectArchiveFormat(src.RawURL)
	if format == "" {
		return "", fmt.Errorf("unsupported archive format for URL: %s", src.RawURL)
	}

	if err := extractToCache(src.RawURL, archivePath, format, dirs); err != nil {
		return "", err
	}

	// Compute content hash for integrity checking.
	contentHash, err := hashDirectory(dirs.contentDir)
	if err != nil {
		return "", fmt.Errorf("hashing content for %s: %w", src.RawURL, err)
	}

	meta := HTTPMeta{
		URL:              src.RawURL,
		LastFetched:      time.Now().UTC(),
		ETag:             resp.Header.Get("ETag"),
		LastModified:     resp.Header.Get("Last-Modified"),
		ContentSizeBytes: size,
		ArchiveFormat:    format,
		ContentHash:      contentHash,
	}
	if err := WriteMeta(dirs.metaPath, meta); err != nil {
		return "", fmt.Errorf("writing meta for %s: %w", src.RawURL, err)
	}

	return dirs.contentDir, nil
}

// downloadArchive saves the response body to a temporary file, enforcing the
// maximum download size. Returns the temp file path and the number of bytes written.
func downloadArchive(rawURL, hashDir string, resp *http.Response) (string, int64, error) {
	tmpFile, err := os.CreateTemp(hashDir, "download-*")
	if err != nil {
		return "", 0, fmt.Errorf("creating temp file for %s: %w", rawURL, err)
	}
	tmpPath := tmpFile.Name()

	limitedBody := io.LimitReader(resp.Body, MaxTotalSize+1) // +1 to detect overflow
	size, err := io.Copy(tmpFile, limitedBody)
	if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, fmt.Errorf("downloading %s: %w", rawURL, err)
	}
	if size > MaxTotalSize {
		_ = os.Remove(tmpPath)
		return "", 0, fmt.Errorf("download exceeds %d MB size limit", MaxTotalSize/(1024*1024))
	}

	return tmpPath, size, nil
}

// extractToCache extracts an archive into a temp directory, unwraps single-root
// archives, and swaps the result into the cache content directory.
func extractToCache(rawURL, archivePath, format string, dirs httpCacheDirs) error {
	tmpExtractDir, err := os.MkdirTemp(dirs.hashDir, "extract-*")
	if err != nil {
		return fmt.Errorf("creating temp extract dir for %s: %w", rawURL, err)
	}
	defer func() { _ = os.RemoveAll(tmpExtractDir) }()

	switch format {
	case "tar.gz":
		err = ExtractTarGz(archivePath, tmpExtractDir)
	case "zip":
		err = ExtractZip(archivePath, tmpExtractDir)
	}
	if err != nil {
		return fmt.Errorf("extracting %s: %w", rawURL, err)
	}

	if err := unwrapSingleRoot(tmpExtractDir); err != nil {
		return fmt.Errorf("unwrapping single root for %s: %w", rawURL, err)
	}

	if err := replaceContent(dirs.contentDir, tmpExtractDir); err != nil {
		return fmt.Errorf("replacing content for %s: %w", rawURL, err)
	}

	return nil
}

// hasContent returns true if the directory exists and contains at least one entry.
func hasContent(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// DetectArchiveFormat returns the archive format based on URL extension.
// Returns "tar.gz", "zip", or "" if unsupported.
func DetectArchiveFormat(rawURL string) string {
	lower := strings.ToLower(rawURL)
	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	default:
		return ""
	}
}

// ExtractTarGz extracts a .tar.gz or .tgz archive to targetDir with
// size/count/path-traversal protections enforced by extractionGuard.
func ExtractTarGz(archivePath, targetDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	guard := &extractionGuard{}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		// Skip symlinks silently.
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			continue
		}

		// Zip-slip protection.
		absPath, err := guard.checkPath(header.Name, targetDir)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return fmt.Errorf("creating directory %q: %w", header.Name, err)
			}

		case tar.TypeReg:
			if err := guard.checkFile(header.Name, header.Size); err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
				return fmt.Errorf("creating parent dir for %q: %w", header.Name, err)
			}

			outFile, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("creating file %q: %w", header.Name, err)
			}

			written, err := io.Copy(outFile, io.LimitReader(tr, MaxFileSize+1))
			if closeErr := outFile.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("writing file %q: %w", header.Name, err)
			}
			if written > MaxFileSize {
				return fmt.Errorf("file %q exceeds maximum file size (%d bytes)", header.Name, MaxFileSize)
			}

		default:
			continue
		}
	}

	return nil
}

// ExtractZip extracts a .zip archive to targetDir with size/count/path-traversal
// protections enforced by extractionGuard.
func ExtractZip(archivePath, targetDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("opening zip archive: %w", err)
	}
	defer func() { _ = r.Close() }()

	guard := &extractionGuard{}

	for _, f := range r.File {
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			continue
		}

		absPath, err := guard.checkPath(f.Name, targetDir)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return fmt.Errorf("creating directory %q: %w", f.Name, err)
			}
			continue
		}

		if err := guard.checkFile(f.Name, int64(f.UncompressedSize64)); err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return fmt.Errorf("creating parent dir for %q: %w", f.Name, err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening zip entry %q: %w", f.Name, err)
		}

		outFile, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("creating file %q: %w", f.Name, err)
		}

		written, err := io.Copy(outFile, io.LimitReader(rc, MaxFileSize+1))
		if cerr := outFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
		_ = rc.Close()
		if err != nil {
			return fmt.Errorf("writing file %q: %w", f.Name, err)
		}
		if written > MaxFileSize {
			return fmt.Errorf("file %q exceeds maximum file size (%d bytes)", f.Name, MaxFileSize)
		}
	}

	return nil
}

// unwrapSingleRoot checks if dir contains exactly one entry and it is a
// directory. If so, it moves the contents of that single directory up one level
// and removes the now-empty wrapper directory.
func unwrapSingleRoot(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}

	wrapperDir := filepath.Join(dir, entries[0].Name())
	innerEntries, err := os.ReadDir(wrapperDir)
	if err != nil {
		return fmt.Errorf("reading wrapper directory: %w", err)
	}

	for _, inner := range innerEntries {
		src := filepath.Join(wrapperDir, inner.Name())
		dst := filepath.Join(dir, inner.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("moving %q up: %w", inner.Name(), err)
		}
	}

	if err := os.Remove(wrapperDir); err != nil {
		return fmt.Errorf("removing wrapper directory: %w", err)
	}

	return nil
}

// skipSSRFValidation disables SSRF host validation for test servers that bind
// to 127.0.0.1. Only set from test code via disableSSRFForTest.
// Uses atomic.Bool to prevent data races if tests ever run in parallel.
var skipSSRFValidation atomic.Bool

// validateHost parses the hostname from rawURL and rejects loopback, private,
// and link-local IP addresses to prevent SSRF attacks. It does not enforce
// the URL scheme; scheme validation is handled by Parse() (which rejects
// http:// at the entry point) and by validateRedirect (for redirect targets).
//
// This pre-request check catches IP-literal URLs early. DNS-name hosts are
// validated at connect time by ssrfSafeDialContext in client.go, which
// resolves DNS and validates all resolved IPs before connecting — providing
// race-free DNS rebinding protection.
func validateHost(rawURL string) error {
	if skipSSRFValidation.Load() {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	host := parsed.Hostname()
	if host == "" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return fmt.Errorf("request to private/loopback address rejected: %s", host)
		}
	}
	return nil
}

// validateRedirect checks a redirect target URL for both scheme safety
// (HTTPS required) and SSRF (private/loopback/link-local IPs).
// Unlike validateHost, this always runs even when skipSSRFValidation is set,
// because redirect targets must always be validated regardless of test mode.
//
// Like validateHost, this checks IP-literal hostnames. DNS-name hosts
// are validated at connect time by ssrfSafeDialContext (see client.go).
func validateRedirect(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid redirect URL %q: %w", rawURL, err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("redirect to insecure URL rejected: %s", rawURL)
	}
	host := parsed.Hostname()
	if host == "" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return fmt.Errorf("redirect to private/loopback address rejected: %s", host)
		}
	}
	return nil
}

// replaceContent replaces the contents of contentDir with the contents of srcDir.
func replaceContent(contentDir, srcDir string) error {
	entries, err := os.ReadDir(contentDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading content dir: %w", err)
	}
	for _, entry := range entries {
		p := filepath.Join(contentDir, entry.Name())
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("removing old content %q: %w", entry.Name(), err)
		}
	}

	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return fmt.Errorf("creating content dir: %w", err)
	}

	newEntries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("reading extract dir: %w", err)
	}
	for _, entry := range newEntries {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(contentDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("moving %q to content: %w", entry.Name(), err)
		}
	}

	return nil
}

// hashDirectory computes a deterministic SHA-256 hash of a directory's content.
// Delegates to manifest.HashLayerDir for consistent hashing across the codebase.
func hashDirectory(dirPath string) (string, error) {
	return manifest.HashLayerDir(dirPath)
}

// defaultCacheTTL is the default time-to-live for cache entries.
// After TTL expires, a re-fetch is attempted. If offline, stale entries are used with a warning.
const defaultCacheTTL = 7 * 24 * time.Hour // 7 days

// verifyHTTPCacheIntegrity checks the integrity of a cached HTTP layer by comparing
// the stored content hash against the actual content. If the hash mismatches, logs
// a warning and returns false (caller should re-fetch). If the meta is missing the
// content_hash field (older cache entry), verification is skipped and returns true.
func verifyHTTPCacheIntegrity(dirs httpCacheDirs, meta HTTPMeta) bool {
	// Skip verification for older cache entries without content_hash.
	if meta.ContentHash == "" {
		return true
	}

	actualHash, err := hashDirectory(dirs.contentDir)
	if err != nil {
		slog.Warn("cache integrity check failed: unable to hash content", "url", meta.URL, "error", err)
		return false
	}

	if actualHash != meta.ContentHash {
		slog.Warn("cache integrity check failed: content hash mismatch (corruption or tampering detected)",
			"url", meta.URL, "expected", meta.ContentHash, "actual", actualHash)
		return false
	}

	return true
}

// isCacheExpired returns true if the cache entry has exceeded the TTL.
// TTL is measured from LastFetched. Expired entries trigger a re-fetch.
func isCacheExpired(meta HTTPMeta, ttl time.Duration) bool {
	if meta.LastFetched.IsZero() {
		return true
	}
	return time.Since(meta.LastFetched) > ttl
}
