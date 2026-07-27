package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// executeDownload downloads a binary or archive using the hardened HTTP
// client (SSRF validation, 120s timeout, redirect safety, DNS rebinding
// protection), verifies its SHA256 if specified, extracts archives using
// the guarded extraction functions (size/count/path-traversal limits), and
// places the executable in targetDir.
func executeDownload(name string, dep schema.DepSpec, targetDir string, requireChecksums bool) error {
	// SHA256 checksum validation.
	if dep.SHA256 == "" {
		msg := fmt.Sprintf("dependency %q has download URL but no sha256 checksum — consider adding one for integrity verification", name)
		if requireChecksums {
			return fmt.Errorf("%s (--require-checksums is set)", msg)
		}
		slog.Warn(msg, "dep", name)
	}

	// URL template expansion.
	downloadURL := expandURLTemplate(dep.Download)

	slog.Info("downloading dep", "dep", name, "url", downloadURL)

	// Pre-request SSRF validation on the URL.
	if err := remote.ValidateURL(downloadURL); err != nil {
		return fmt.Errorf("URL validation for dep %q: %w", name, err)
	}

	// Download to a temp file using the hardened HTTP client.
	tmpFile, err := os.CreateTemp("", "keel-dep-download-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	client := remote.HardenedClient()
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("creating request for %s: %w", downloadURL, err)
	}
	req.Header.Set("User-Agent", "keel")

	resp, err := client.Do(req)
	if err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("downloading %s: %w", downloadURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_ = tmpFile.Close()
		return fmt.Errorf("downloading %s: HTTP %d", downloadURL, resp.StatusCode)
	}

	// Enforce download size limit (same as remote package).
	limitedBody := io.LimitReader(resp.Body, remote.MaxTotalSize+1)
	written, err := io.Copy(tmpFile, limitedBody)
	if err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing download to temp file: %w", err)
	}
	if written > remote.MaxTotalSize {
		_ = tmpFile.Close()
		return fmt.Errorf("download exceeds %d MB size limit", remote.MaxTotalSize/(1024*1024))
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// SHA256 verification.
	if dep.SHA256 != "" {
		actual, err := fileSHA256(tmpPath)
		if err != nil {
			return fmt.Errorf("computing sha256 for dep %q: %w", name, err)
		}
		if actual != dep.SHA256 {
			return fmt.Errorf("sha256 mismatch for dep %q: expected %s, got %s", name, dep.SHA256, actual)
		}
	}

	// Archive detection and extraction.
	format := remote.DetectArchiveFormat(downloadURL)
	if format != "" {
		return extractAndPlace(name, dep, tmpPath, format, targetDir)
	}

	// Raw binary: stream to targetDir/name with 0755.
	dstPath := filepath.Join(targetDir, name)
	if err := streamCopyFile(tmpPath, dstPath, 0755); err != nil {
		return fmt.Errorf("copying binary to %s: %w", dstPath, err)
	}

	slog.Info("dep downloaded (raw binary)", "dep", name)
	return nil
}

// extractAndPlace extracts an archive using the hardened extraction functions
// from pkg/remote (with size/count/path-traversal limits), locates the
// executable, and copies it to targetDir/name with 0755 permissions.
func extractAndPlace(name string, dep schema.DepSpec, archivePath, format, targetDir string) error {
	tmpExtractDir, err := os.MkdirTemp("", "keel-dep-extract-*")
	if err != nil {
		return fmt.Errorf("creating temp extract dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpExtractDir) }()

	switch format {
	case "tar.gz":
		if err := remote.ExtractTarGz(archivePath, tmpExtractDir); err != nil {
			return fmt.Errorf("extracting tar.gz: %w", err)
		}
	case "zip":
		if err := remote.ExtractZip(archivePath, tmpExtractDir); err != nil {
			return fmt.Errorf("extracting zip: %w", err)
		}
	}

	// Locate the executable.
	execPath, err := locateExecutable(tmpExtractDir, name, dep.Executable)
	if err != nil {
		return err
	}

	// Stream to targetDir/name.
	dstPath := filepath.Join(targetDir, name)
	if err := streamCopyFile(execPath, dstPath, 0755); err != nil {
		return fmt.Errorf("copying executable to %s: %w", dstPath, err)
	}

	slog.Info("dep downloaded (archive)", "dep", name)
	return nil
}

// locateExecutable finds the executable in an extracted archive directory.
// If executableField is set, looks for that relative path. Otherwise looks for
// a file matching the dep name. Returns an error if not found or ambiguous.
func locateExecutable(extractDir, depName, executableField string) (string, error) {
	if executableField != "" {
		candidate := filepath.Join(extractDir, filepath.Clean(executableField))
		if !pathutil.ContainsPath(extractDir, candidate) {
			return "", fmt.Errorf("executable path %q escapes extract directory", executableField)
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		return "", fmt.Errorf("executable %q not found in archive", executableField)
	}

	// Search for a file matching the dep name.
	var matches []string
	err := filepath.WalkDir(extractDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == depName {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("searching for executable in archive: %w", err)
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no file named %q found in archive; set 'executable' to specify the path", depName)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple files named %q found in archive; set 'executable' to specify which one", depName)
	}
}

// streamCopyFile copies a file from src to dst using streaming io.Copy to
// avoid loading the entire file into memory. The destination file is created
// with the specified permissions.
func streamCopyFile(src, dst string, perm os.FileMode) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source %s: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("creating destination %s: %w", dst, err)
	}
	defer func() {
		if cerr := dstFile.Close(); err == nil {
			err = cerr
		}
	}()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying data: %w", err)
	}
	return nil
}

// expandURLTemplate replaces ${os} and ${arch} in a download URL with the
// current GOOS and GOARCH values.
func expandURLTemplate(url string) string {
	url = strings.ReplaceAll(url, "${os}", runtime.GOOS)
	url = strings.ReplaceAll(url, "${arch}", runtime.GOARCH)
	return url
}

// fileSHA256 computes the hex-encoded SHA256 hash of a file.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
