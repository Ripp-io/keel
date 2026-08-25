package pathutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// maxConfigFileSize is the maximum allowed size for a configuration file (10 MB).
const maxConfigFileSize = 10 * 1024 * 1024

// SafeReadFile reads a file after verifying it does not exceed maxConfigFileSize.
// This prevents unbounded memory allocation from unexpectedly large files.
//
// Note: downstream JSON/YAML parsing uses standard library unmarshallers without
// custom nesting depth limits. The file size cap provides practical protection
// against deeply nested input causing stack exhaustion during parsing.
func SafeReadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxConfigFileSize {
		return nil, fmt.Errorf("file %q exceeds maximum size (%d > %d bytes)", path, info.Size(), maxConfigFileSize)
	}
	return os.ReadFile(path)
}

// CopyFile copies a file from src to dst using streaming I/O, preserving permissions.
// Parent directories for dst are created if needed.
func CopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", dst, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source %q: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination %q: %w", dst, err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		_ = dstFile.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("copying %q to %q: %w", src, dst, err)
	}

	if err := dstFile.Close(); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("closing destination %q: %w", dst, err)
	}

	// Preserve source permissions
	srcInfo, err := os.Stat(src)
	if err == nil {
		_ = os.Chmod(dst, srcInfo.Mode())
	}

	return nil
}
