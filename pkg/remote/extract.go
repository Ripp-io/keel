package remote

import (
	"fmt"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
)

// extractionGuard tracks size and count limits during archive extraction.
type extractionGuard struct {
	totalSize int64
	fileCount int
}

// checkPath validates that a file path does not escape the target directory
// (zip-slip protection). Returns the cleaned absolute path or an error.
func (g *extractionGuard) checkPath(name, targetDir string) (string, error) {
	absPath := filepath.Join(targetDir, filepath.Clean(name))
	if !pathutil.ContainsPath(targetDir, absPath) {
		return "", fmt.Errorf("path escapes target directory: %s", name)
	}
	return absPath, nil
}

// checkFile validates a single file against extraction limits.
// Returns an error if the file would exceed any limit.
func (g *extractionGuard) checkFile(name string, size int64) error {
	g.fileCount++
	if g.fileCount > MaxFileCount {
		return fmt.Errorf("archive exceeds maximum file count (%d)", MaxFileCount)
	}

	if size > MaxFileSize {
		return fmt.Errorf("file %q exceeds maximum file size (%d bytes)", name, MaxFileSize)
	}

	g.totalSize += size
	if g.totalSize > MaxTotalSize {
		return fmt.Errorf("archive exceeds maximum total extracted size (%d bytes)", MaxTotalSize)
	}

	return nil
}
