package pathutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeReadFile_NormalFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "small.yml")
	content := []byte("schema: v1\nname: test\n")
	require.NoError(t, os.WriteFile(path, content, 0644))

	got, err := SafeReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(content), string(got))
}

func TestSafeReadFile_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := SafeReadFile("/nonexistent/file.yml")
	require.Error(t, err)
}

func TestSafeReadFile_ExceedsMaxSize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.yml")

	f, err := os.Create(path)
	require.NoError(t, err)
	_, err = f.Seek(maxConfigFileSize, 0)
	require.NoError(t, err)
	_, err = f.Write([]byte{0})
	require.NoError(t, err)
	f.Close()

	_, err = SafeReadFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum size")
}

func TestSafeReadFile_ExactlyAtMaxSize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "exact.yml")

	f, err := os.Create(path)
	require.NoError(t, err)
	_, err = f.Seek(maxConfigFileSize-1, 0)
	require.NoError(t, err)
	_, err = f.Write([]byte{0})
	require.NoError(t, err)
	f.Close()

	_, err = SafeReadFile(path)
	require.NoError(t, err, "SafeReadFile should accept file at exactly maxConfigFileSize")
}

func TestSafeReadFile_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yml")
	require.NoError(t, os.WriteFile(path, []byte{}, 0644))

	got, err := SafeReadFile(path)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCopyFile_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "source.txt")
	content := []byte("test content\nline 2\n")
	require.NoError(t, os.WriteFile(src, content, 0644))

	dst := filepath.Join(dir, "subdir", "destination.txt")
	err := CopyFile(src, dst)
	require.NoError(t, err)

	// Verify content was copied
	gotContent, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, string(content), string(gotContent))

	// Verify permissions were preserved
	srcInfo, err := os.Stat(src)
	require.NoError(t, err)
	dstInfo, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, srcInfo.Mode(), dstInfo.Mode())
}

func TestCopyFile_CreatesParentDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "source.txt")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	dst := filepath.Join(dir, "deep", "nested", "path", "file.txt")
	err := CopyFile(src, dst)
	require.NoError(t, err)

	// Verify the file exists
	_, err = os.Stat(dst)
	assert.NoError(t, err)
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "nonexistent.txt")
	dst := filepath.Join(dir, "dest.txt")

	err := CopyFile(src, dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opening source")
}

func TestCopyFile_OverwritesExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "source.txt")
	require.NoError(t, os.WriteFile(src, []byte("new content"), 0644))

	dst := filepath.Join(dir, "dest.txt")
	require.NoError(t, os.WriteFile(dst, []byte("old content"), 0644))

	err := CopyFile(src, dst)
	require.NoError(t, err)

	gotContent, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(gotContent))
}

func TestCopyFile_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(src, []byte{}, 0644))

	dst := filepath.Join(dir, "dest.txt")
	err := CopyFile(src, dst)
	require.NoError(t, err)

	gotContent, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Empty(t, gotContent)
}
