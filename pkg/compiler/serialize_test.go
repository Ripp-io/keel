package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteCategory_PathTraversalRejection(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	files := []CategoryFile{
		{
			RelPath: "../../escape",
			Content: []byte("malicious"),
			IsDir:   false,
		},
	}

	_, err := WriteCategory(tmpDir, "test-category", files)
	require.Error(t, err, "expected error for path traversal")
	assert.Contains(t, err.Error(), `category file "../../escape" escapes output directory`)
}

func TestWriteCategory_ValidFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	files := []CategoryFile{
		{
			RelPath: "file1.md",
			Content: []byte("content1"),
			IsDir:   false,
		},
		{
			RelPath: "subdir/file2.md",
			Content: []byte("content2"),
			IsDir:   false,
		},
	}

	_, err := WriteCategory(tmpDir, "commands", files)
	require.NoError(t, err)

	// Verify files were created
	content1, err := os.ReadFile(filepath.Join(tmpDir, "commands", "file1.md"))
	require.NoError(t, err, "reading file1.md")
	assert.Equal(t, "content1", string(content1))

	content2, err := os.ReadFile(filepath.Join(tmpDir, "commands", "subdir", "file2.md"))
	require.NoError(t, err, "reading subdir/file2.md")
	assert.Equal(t, "content2", string(content2))
}
