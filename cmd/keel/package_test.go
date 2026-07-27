package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageCmd_DefaultFormat(t *testing.T) {
	srcDir := t.TempDir()
	writePackageFixture(t, srcDir, "hello.txt", "hello")
	cmd := newPackageCmd()
	outDir := t.TempDir()
	expectedPath := filepath.Join(outDir, filepath.Base(srcDir)+".tar.gz")
	cmd.SetArgs([]string{"--from", srcDir, "--to", expectedPath})
	require.NoError(t, cmd.Execute())
	_, err := os.Stat(expectedPath)
	require.NoError(t, err)
}

func TestPackageCmd_ZipFormat(t *testing.T) {
	srcDir := t.TempDir()
	writePackageFixture(t, srcDir, "hello.txt", "hello")
	cmd := newPackageCmd()
	outDir := t.TempDir()
	expectedPath := filepath.Join(outDir, "out.zip")
	cmd.SetArgs([]string{"--from", srcDir, "--format", "zip", "--to", expectedPath})
	require.NoError(t, cmd.Execute())
	_, err := os.Stat(expectedPath)
	require.NoError(t, err)
}

func TestPackageCmd_InvalidFormat(t *testing.T) {
	srcDir := t.TempDir()
	writePackageFixture(t, srcDir, "hello.txt", "hello")
	cmd := newPackageCmd()
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--from", srcDir, "--format", "xml"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xml")
}

func TestPackageCmd_MissingDir(t *testing.T) {
	cmd := newPackageCmd()
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--from", "/nonexistent/path/that/does/not/exist"})
	err := cmd.Execute()
	require.Error(t, err)
	errMsg := err.Error()
	assert.True(t, strings.Contains(errMsg, "does not exist") || strings.Contains(errMsg, "not a directory"), "got: %v", err)
}

func TestPackageCmd_DirNotADirectory(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(tmpFile, []byte("content"), 0644))
	cmd := newPackageCmd()
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--from", tmpFile})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestPackageCmd_CustomOutput(t *testing.T) {
	srcDir := t.TempDir()
	writePackageFixture(t, srcDir, "file.txt", "data")
	customPath := filepath.Join(t.TempDir(), "custom.tar.gz")
	cmd := newPackageCmd()
	cmd.SetArgs([]string{"--from", srcDir, "--to", customPath})
	require.NoError(t, cmd.Execute())
	_, err := os.Stat(customPath)
	require.NoError(t, err)
}

func TestPackageCmd_DefaultOutputPath(t *testing.T) {
	srcDir := t.TempDir()
	writePackageFixture(t, srcDir, "file.txt", "data")
	expectedPath := srcDir + ".tar.gz"
	cmd := newPackageCmd()
	cmd.SetArgs([]string{"--from", srcDir})
	require.NoError(t, cmd.Execute())
	_, err := os.Stat(expectedPath)
	require.NoError(t, err)
	os.Remove(expectedPath)
}

func TestPackageCmd_TgzAlias(t *testing.T) {
	srcDir := t.TempDir()
	writePackageFixture(t, srcDir, "file.txt", "data")
	outDir := t.TempDir()
	expectedPath := filepath.Join(outDir, "out.tar.gz")
	cmd := newPackageCmd()
	cmd.SetArgs([]string{"--from", srcDir, "--format", "tgz", "--to", expectedPath})
	require.NoError(t, cmd.Execute())
	_, err := os.Stat(expectedPath)
	require.NoError(t, err)
}

func TestPackageCmd_MissingDirFlag(t *testing.T) {
	cmd := newPackageCmd()
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	// Cobra's MarkFlagRequired produces a different error format
	assert.Contains(t, err.Error(), "from")
}

func writePackageFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}
