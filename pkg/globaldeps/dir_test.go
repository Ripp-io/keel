package globaldeps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinDir_ReturnsKeelBin(t *testing.T) {
	t.Parallel()

	dir := BinDir()
	assert.Contains(t, dir, ".keel")
	assert.Equal(t, "bin", filepath.Base(dir))
}

func TestDir_ReturnsKeel(t *testing.T) {
	t.Parallel()

	dir := Dir()
	assert.Contains(t, dir, ".keel")
	assert.Equal(t, ".keel", filepath.Base(dir))
}

func TestBinDir_FallbackOnNoHome(t *testing.T) {
	// Save and unset HOME
	orig := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer func() {
		if orig != "" {
			os.Setenv("HOME", orig)
		}
	}()

	dir := BinDir()

	// The fallback should trigger, returning a relative path
	expected := filepath.Join(".", ".keel", "bin")
	assert.Equal(t, expected, dir, "should return fallback path when HOME is unset")
}

func TestDir_FallbackOnNoHome(t *testing.T) {
	// Save and unset HOME
	orig := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer func() {
		if orig != "" {
			os.Setenv("HOME", orig)
		}
	}()

	dir := Dir()

	// The fallback should trigger, returning a relative path
	expected := filepath.Join(".", ".keel")
	assert.Equal(t, expected, dir, "should return fallback path when HOME is unset")
}
