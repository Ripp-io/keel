package pathutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainsPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		base   string
		target string
		want   bool
	}{
		{"path inside base", "/a/b", "/a/b/c", true},
		{"path is base", "/a/b", "/a/b", true},
		{"path outside via parent", "/a/b", "/a/c", false},
		{"path outside via dotdot", "/a/b", "/a/b/../c", false},
		{"deeply nested inside", "/a", "/a/b/c/d/e", true},
		{"sibling directory", "/a/b", "/a/bc", false},
		{"valid subpath", "/tmp/base", "/tmp/base/sub/dir", true},
		{"path traversal escape", "/tmp/base", "/tmp/base/../../etc/passwd", false},
		{"dot-dot subpath resolves inside", "/tmp/base", "/tmp/base/../base/sub", true},
		{"exact match", "/tmp/base", "/tmp/base", true},
		{"sibling named similarly", "/tmp/base", "/tmp/other", false},
		{"prefix attack with similar name", "/tmp/base", "/tmp/base-evil/payload", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsPath(tt.base, tt.target)
			assert.Equal(t, tt.want, got, "ContainsPath(%q, %q)", tt.base, tt.target)
		})
	}
}

func TestContainsPath_RealDirs(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	// Valid subpath.
	sub := filepath.Join(base, "sub", "dir")
	require.NoError(t, os.MkdirAll(sub, 0755))
	assert.True(t, ContainsPath(base, sub), "ContainsPath(%q, %q) should be true", base, sub)

	// Path traversal escape.
	escaped := filepath.Join(base, "..", "..", "etc")
	assert.False(t, ContainsPath(base, escaped), "ContainsPath(%q, %q) should be false (path escapes base)", base, escaped)
}

func TestContainsPath_NonExistentTarget(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	// Target does not exist but is logically inside base.
	target := filepath.Join(base, "does", "not", "exist")
	assert.True(t, ContainsPath(base, target), "ContainsPath(%q, %q) should be true for non-existent but contained path", base, target)
}

func TestContainsPath_Symlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	require.NoError(t, os.MkdirAll(realDir, 0755))

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(realDir, link))

	// A file under the symlinked directory should still be contained in dir.
	target := filepath.Join(link, "file.txt")
	assert.True(t, ContainsPath(dir, target), "ContainsPath(%q, %q) should be true (symlink within base)", dir, target)
}
