package remote

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// extractionGuard.checkPath tests
// ---------------------------------------------------------------------------

func TestExtractionGuard_CheckPath_Valid(t *testing.T) {
	t.Parallel()
	guard := &extractionGuard{}
	targetDir := t.TempDir()

	absPath, err := guard.checkPath("subdir/file.txt", targetDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(targetDir, "subdir", "file.txt"), absPath)
}

func TestExtractionGuard_CheckPath_ZipSlip(t *testing.T) {
	t.Parallel()
	guard := &extractionGuard{}
	targetDir := t.TempDir()

	_, err := guard.checkPath("../../etc/passwd", targetDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path escapes target directory")
}

func TestExtractionGuard_CheckPath_CleanTraversal(t *testing.T) {
	t.Parallel()
	guard := &extractionGuard{}
	targetDir := t.TempDir()

	// A path with ".." that cleans to something inside target should succeed.
	absPath, err := guard.checkPath("subdir/../file.txt", targetDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(targetDir, "file.txt"), absPath)
}

// ---------------------------------------------------------------------------
// extractionGuard.checkFile tests
// ---------------------------------------------------------------------------

func TestExtractionGuard_CheckFile_Valid(t *testing.T) {
	t.Parallel()
	guard := &extractionGuard{}

	err := guard.checkFile("file.txt", 1024)
	require.NoError(t, err)
	assert.Equal(t, 1, guard.fileCount)
	assert.Equal(t, int64(1024), guard.totalSize)
}

func TestExtractionGuard_CheckFile_ExceedsFileSize(t *testing.T) {
	t.Parallel()
	guard := &extractionGuard{}

	err := guard.checkFile("large.bin", MaxFileSize+1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum file size")
}

func TestExtractionGuard_CheckFile_ExceedsFileCount(t *testing.T) {
	t.Parallel()
	guard := &extractionGuard{}

	// Fill up to the limit.
	for i := 0; i < MaxFileCount; i++ {
		require.NoError(t, guard.checkFile("file.txt", 1), "checkFile() at count %d", i)
	}

	// One more should fail.
	err := guard.checkFile("one-too-many.txt", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum file count")
}

func TestExtractionGuard_CheckFile_ExceedsTotalSize(t *testing.T) {
	t.Parallel()
	guard := &extractionGuard{}

	// Accumulate total size across multiple files, each under MaxFileSize.
	perFile := int64(MaxFileSize - 1)
	filesNeeded := MaxTotalSize/perFile + 1

	for i := int64(0); i < filesNeeded-1; i++ {
		err := guard.checkFile("chunk.bin", perFile)
		if err != nil {
			// Once we exceed total, we stop.
			require.Contains(t, err.Error(), "maximum total extracted size")
			return // test passes -- we hit the limit
		}
	}

	// One more should push over the total limit.
	err := guard.checkFile("final.bin", perFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum total extracted size")
}

func TestExtractionGuard_CheckFile_AccumulatesCorrectly(t *testing.T) {
	t.Parallel()
	guard := &extractionGuard{}

	sizes := []int64{100, 200, 300}
	for i, size := range sizes {
		require.NoError(t, guard.checkFile("file.txt", size), "step %d", i)
	}

	assert.Equal(t, 3, guard.fileCount)
	assert.Equal(t, int64(600), guard.totalSize)
}
