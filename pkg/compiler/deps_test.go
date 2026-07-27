package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/globaldeps"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

func TestProcessDeps_SkippedWithoutAllowInstall(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {Install: "echo install", Global: false},
	}

	records, err := processDeps(deps, outputDir, false, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 1)

	rec := records[0]
	assert.Equal(t, "skipped", rec.Method)
	assert.Equal(t, "mytool", rec.Name)
	assert.Equal(t, "project", rec.Scope)
}

func TestProcessDeps_VerifyNotExecutedWithoutAllowInstall(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	vars := map[string]string{}

	// Use a verify command that creates a marker file. If verify runs,
	// the marker file will exist. Since allowInstall is false, verify
	// must NOT execute — the marker must not appear.
	markerFile := filepath.Join(t.TempDir(), "verify-ran-marker")
	verifyCmd := fmt.Sprintf("touch %s", markerFile)

	deps := map[string]schema.DepSpec{
		"mytool": {
			Install: "echo install",
			Global:  true,
			Verify:  verifyCmd,
		},
	}

	records, err := processDeps(deps, outputDir, false, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "skipped", records[0].Method,
		"dep should be skipped when allowInstall is false")

	// The marker file must not exist — verify must not have run.
	_, err = os.Stat(markerFile)
	assert.True(t, os.IsNotExist(err),
		"verify command should NOT execute when allowInstall is false")
}

func TestProcessDeps_SkipsPreInstalled(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {Install: "echo install"},
	}

	preInstalled := map[string]string{"mytool": "install"}

	records, err := processDeps(deps, outputDir, true, false, vars, preInstalled)
	require.NoError(t, err)
	require.Len(t, records, 1)

	// Pre-installed dep should record the method from the previous pass.
	rec := records[0]
	assert.Equal(t, "install", rec.Method)
	assert.Equal(t, "mytool", rec.Name)
	assert.Equal(t, "project", rec.Scope)

	// Variables should still be injected.
	expectedPath := filepath.Join(outputDir, ".keel", "bin", "mytool")
	assert.Equal(t, expectedPath, vars["keel.deps.mytool.path"])
}

func TestProcessDeps_PreInstalledSkipsDownload(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {Download: "https://example.com/mytool"},
	}

	preInstalled := map[string]string{"mytool": "download"}

	records, err := processDeps(deps, outputDir, true, false, vars, preInstalled)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "download", records[0].Method)

	// The binary should NOT have been downloaded (no server to hit).
	// If it tried to download, the test would fail with a connection error.
}

func TestProcessDeps_VariablesInjected(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"alpha": {Install: "echo install", Global: false},
		"beta":  {Download: "https://example.com/beta", Global: true},
	}

	// Run without allow-install so we only test variable injection.
	_, err := processDeps(deps, outputDir, false, false, vars, nil)
	require.NoError(t, err)

	// Check alpha variables (project scope).
	expectedAlphaPath := filepath.Join(outputDir, ".keel", "bin", "alpha")
	expectedAlphaDir := filepath.Join(outputDir, ".keel", "bin")
	assert.Equal(t, expectedAlphaPath, vars["keel.deps.alpha.path"])
	assert.Equal(t, expectedAlphaDir, vars["keel.deps.alpha.dir"])

	// Check beta variables (global scope).
	globalDir := globaldeps.BinDir()
	expectedBetaPath := filepath.Join(globalDir, "beta")
	assert.Equal(t, expectedBetaPath, vars["keel.deps.beta.path"])
	assert.Equal(t, globalDir, vars["keel.deps.beta.dir"])
}

func TestProcessDeps_DownloadRawBinary(t *testing.T) {
	remote.DisableSSRFForTest()
	t.Cleanup(remote.RestoreSSRFForTest)

	binaryContent := []byte("#!/bin/sh\necho hello\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	}))
	defer srv.Close()

	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {Download: srv.URL + "/mytool"},
	}

	records, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "download", records[0].Method)

	// Verify the binary was placed correctly.
	binPath := filepath.Join(outputDir, ".keel", "bin", "mytool")
	data, err := os.ReadFile(binPath)
	require.NoError(t, err, "expected binary at %s", binPath)
	assert.Equal(t, string(binaryContent), string(data))

	// Verify executable permissions.
	info, err := os.Stat(binPath)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode().Perm()&0111, "expected executable permissions, got %o", info.Mode().Perm())
}

func TestProcessDeps_DownloadArchiveWithExecutable(t *testing.T) {
	remote.DisableSSRFForTest()
	t.Cleanup(remote.RestoreSSRFForTest)

	// Create a tar.gz with a specific file structure.
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"bin/my-actual-binary": "binary content here",
		"README.md":            "readme",
	})
	archiveData, err := os.ReadFile(archivePath)
	require.NoError(t, err, "reading test archive")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData)
	}))
	defer srv.Close()

	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {
			Download:   srv.URL + "/mytool.tar.gz",
			Executable: "bin/my-actual-binary",
		},
	}

	records, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "download", records[0].Method)

	// Verify the correct file was placed.
	binPath := filepath.Join(outputDir, ".keel", "bin", "mytool")
	data, err := os.ReadFile(binPath)
	require.NoError(t, err, "expected binary at %s", binPath)
	assert.Equal(t, "binary content here", string(data))
}

func TestProcessDeps_DownloadArchiveDefaultName(t *testing.T) {
	remote.DisableSSRFForTest()
	t.Cleanup(remote.RestoreSSRFForTest)

	// Create a tar.gz where a file matches the dep name.
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"mytool":    "default name binary",
		"README.md": "readme",
	})
	archiveData, err := os.ReadFile(archivePath)
	require.NoError(t, err, "reading test archive")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData)
	}))
	defer srv.Close()

	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {Download: srv.URL + "/mytool.tar.gz"},
	}

	records, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 1)

	binPath := filepath.Join(outputDir, ".keel", "bin", "mytool")
	data, err := os.ReadFile(binPath)
	require.NoError(t, err, "expected binary at %s", binPath)
	assert.Equal(t, "default name binary", string(data))
}

func TestProcessDeps_SHA256Match(t *testing.T) {
	remote.DisableSSRFForTest()
	t.Cleanup(remote.RestoreSSRFForTest)

	binaryContent := []byte("trusted binary content")
	h := sha256.Sum256(binaryContent)
	expectedHash := hex.EncodeToString(h[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	}))
	defer srv.Close()

	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {
			Download: srv.URL + "/mytool",
			SHA256:   expectedHash,
		},
	}

	records, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "download", records[0].Method)

	// Verify the binary was placed.
	binPath := filepath.Join(outputDir, ".keel", "bin", "mytool")
	_, err = os.Stat(binPath)
	require.NoError(t, err, "expected binary at %s", binPath)
}

func TestProcessDeps_SHA256Mismatch(t *testing.T) {
	remote.DisableSSRFForTest()
	t.Cleanup(remote.RestoreSSRFForTest)

	binaryContent := []byte("some binary content")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	}))
	defer srv.Close()

	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {
			Download: srv.URL + "/mytool",
			SHA256:   "0000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	_, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.Error(t, err, "expected error for sha256 mismatch")
	assert.Contains(t, err.Error(), "sha256 mismatch")
}

func TestProcessDeps_URLTemplateExpansion(t *testing.T) {
	remote.DisableSSRFForTest()
	t.Cleanup(remote.RestoreSSRFForTest)

	var requestedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Write([]byte("binary"))
	}))
	defer srv.Close()

	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {Download: srv.URL + "/releases/${os}/${arch}/mytool"},
	}

	_, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.NoError(t, err)

	expectedPath := fmt.Sprintf("/releases/%s/%s/mytool", runtime.GOOS, runtime.GOARCH)
	assert.Equal(t, expectedPath, requestedPath, "URL template not expanded")
}

func TestProcessDeps_InstallCommand(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	targetDir := filepath.Join(outputDir, ".keel", "bin")
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vars := map[string]string{}

	// The install command creates a file in the target directory.
	// Since PATH is prepended with targetDir, we write directly.
	installCmd := fmt.Sprintf("echo '#!/bin/sh' > %s/mytool && chmod +x %s/mytool", targetDir, targetDir)

	deps := map[string]schema.DepSpec{
		"mytool": {Install: installCmd},
	}

	records, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "install", records[0].Method)
}

func TestProcessDeps_InstallEnvVars(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	targetDir := filepath.Join(outputDir, ".keel", "bin")
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vars := map[string]string{}

	// Use KEEL_DEP_DIR and KEEL_DEP_NAME to place the binary — proves the
	// env vars are injected and usable by install commands.
	installCmd := `printf '#!/bin/sh\necho mytool v1' > "$KEEL_DEP_DIR/$KEEL_DEP_NAME" && chmod +x "$KEEL_DEP_DIR/$KEEL_DEP_NAME"`

	deps := map[string]schema.DepSpec{
		"mytool": {Install: installCmd},
	}

	records, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "install", records[0].Method)

	// Verify the binary was placed via env vars.
	binPath := filepath.Join(targetDir, "mytool")
	content, err := os.ReadFile(binPath)
	require.NoError(t, err, "binary not found at %s", binPath)
	assert.Contains(t, string(content), "mytool v1")
}

func TestProcessDeps_GlobalVsProject(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"globaltool":  {Install: "echo ok", Global: true},
		"projecttool": {Install: "echo ok", Global: false},
	}

	// Run without allow-install to just check variable injection and scoping.
	records, err := processDeps(deps, outputDir, false, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 2)

	// Find records by name (sorted order: globaltool, projecttool).
	var globalRec, projectRec manifest.InstalledDepRecord
	for _, r := range records {
		switch r.Name {
		case "globaltool":
			globalRec = r
		case "projecttool":
			projectRec = r
		}
	}

	assert.Equal(t, "global", globalRec.Scope)
	assert.Equal(t, "project", projectRec.Scope)

	// Verify global dep targets ~/.keel/bin/.
	globalDir := globaldeps.BinDir()
	assert.Equal(t, globalDir, vars["keel.deps.globaltool.dir"])

	// Verify project dep targets .keel/bin/ under outputDir.
	projectDir := filepath.Join(outputDir, ".keel", "bin")
	assert.Equal(t, projectDir, vars["keel.deps.projecttool.dir"])
}

func TestProcessDeps_VerifySkipsGlobalInstall(t *testing.T) {
	t.Parallel()
	// We cannot easily override globaldeps.BinDir() in a unit test, so we
	// verify the underlying verifyInstalled function directly. The global
	// dep verify-skip path calls verifyInstalled identically.
	dir := t.TempDir()

	assert.True(t, verifyInstalled("true", dir), "verifyInstalled('true') should return true")
	assert.False(t, verifyInstalled("false", dir), "verifyInstalled('false') should return false")
}

func TestProcessDeps_DownloadZipArchive(t *testing.T) {
	remote.DisableSSRFForTest()
	t.Cleanup(remote.RestoreSSRFForTest)

	// Create a zip archive with a file matching the dep name.
	archivePath := testutil.CreateTestZip(t, t.TempDir(), map[string]string{
		"mytool":    "zip binary content",
		"README.md": "readme",
	})
	archiveData, err := os.ReadFile(archivePath)
	require.NoError(t, err, "reading test archive")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData)
	}))
	defer srv.Close()

	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {Download: srv.URL + "/mytool.zip"},
	}

	records, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 1)

	binPath := filepath.Join(outputDir, ".keel", "bin", "mytool")
	data, err := os.ReadFile(binPath)
	require.NoError(t, err, "expected binary at %s", binPath)
	assert.Equal(t, "zip binary content", string(data))
}

func TestProcessDeps_DeterministicOrder(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"zebra": {Install: "echo ok"},
		"alpha": {Install: "echo ok"},
		"mango": {Install: "echo ok"},
	}

	records, err := processDeps(deps, outputDir, false, false, vars, nil)
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, "alpha", records[0].Name)
	assert.Equal(t, "mango", records[1].Name)
	assert.Equal(t, "zebra", records[2].Name)
}

func TestProcessDeps_InstallCommandFails(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	vars := map[string]string{}

	deps := map[string]schema.DepSpec{
		"mytool": {Install: "echo 'something went wrong' && exit 1"},
	}

	_, err := processDeps(deps, outputDir, true, false, vars, nil)
	require.Error(t, err, "expected error for failed install command")
	assert.Contains(t, err.Error(), "install command failed")
}

func TestExpandURLTemplate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://example.com/releases/${os}/${arch}/tool",
			expected: fmt.Sprintf("https://example.com/releases/%s/%s/tool", runtime.GOOS, runtime.GOARCH),
		},
		{
			input:    "https://example.com/tool-${os}.tar.gz",
			expected: fmt.Sprintf("https://example.com/tool-%s.tar.gz", runtime.GOOS),
		},
		{
			input:    "https://example.com/tool.tar.gz",
			expected: "https://example.com/tool.tar.gz",
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, expandURLTemplate(tt.input), "expandURLTemplate(%q)", tt.input)
	}
}

func TestDetectDepArchiveFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url      string
		expected string
	}{
		{"https://example.com/tool.tar.gz", "tar.gz"},
		{"https://example.com/tool.tgz", "tar.gz"},
		{"https://example.com/tool.zip", "zip"},
		{"https://example.com/tool.TGZ", "tar.gz"},
		{"https://example.com/tool.ZIP", "zip"},
		{"https://example.com/tool", ""},
		{"https://example.com/tool.bin", ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, remote.DetectArchiveFormat(tt.url), "DetectArchiveFormat(%q)", tt.url)
	}
}

func TestFileSHA256(t *testing.T) {
	t.Parallel()
	tmpFile := filepath.Join(t.TempDir(), "testfile")
	content := []byte("test content for hashing")
	require.NoError(t, os.WriteFile(tmpFile, content, 0644))

	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	actual, err := fileSHA256(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestPrependPath(t *testing.T) {
	t.Parallel()
	env := []string{"HOME=/home/user", "PATH=/usr/bin:/bin", "TERM=xterm"}
	result := prependPath(env, "/custom/bin")

	var pathVal string
	for _, e := range result {
		if len(e) > 5 && e[:5] == "PATH=" {
			pathVal = e[5:]
		}
	}

	expected := "/custom/bin" + string(os.PathListSeparator) + "/usr/bin:/bin"
	assert.Equal(t, expected, pathVal)
}

func TestLocateExecutable_WithField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "tool"), []byte("binary"), 0755))

	path, err := locateExecutable(dir, "mytool", "bin/tool")
	require.NoError(t, err)
	assert.Equal(t, "tool", filepath.Base(path))
}

func TestLocateExecutable_DefaultName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mytool"), []byte("binary"), 0755))

	path, err := locateExecutable(dir, "mytool", "")
	require.NoError(t, err)
	assert.Equal(t, "mytool", filepath.Base(path))
}

func TestLocateExecutable_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "othertool"), []byte("binary"), 0755))

	_, err := locateExecutable(dir, "mytool", "")
	require.Error(t, err, "expected error when no matching file found")
	assert.Contains(t, err.Error(), "no file named")
}

func TestLocateExecutable_Ambiguous(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mytool"), []byte("binary1"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "mytool"), []byte("binary2"), 0755))

	_, err := locateExecutable(dir, "mytool", "")
	require.Error(t, err, "expected error for ambiguous match")
	assert.Contains(t, err.Error(), "multiple files")
}

func TestLocateExecutable_ExplicitNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_, err := locateExecutable(dir, "mytool", "nonexistent/path")
	require.Error(t, err, "expected error when executable field points to missing file")
	assert.Contains(t, err.Error(), "not found in archive")
}

// ---------------------------------------------------------------------------
// sanitizeEnv tests
// ---------------------------------------------------------------------------

func TestSanitizeEnv_BlocksSensitiveVars(t *testing.T) {
	t.Parallel()
	env := []string{
		"HOME=/home/user",
		"PATH=/usr/bin",
		"ANTHROPIC_API_KEY=secret-key",
		"OPENAI_API_KEY=sk-1234",
		"AWS_SECRET_ACCESS_KEY=aws-secret",
		"GITHUB_TOKEN=ghp_abc",
		"CUSTOM_VAR=foo",
	}

	result := sanitizeEnv(env)

	// Allowed vars should be present.
	resultStr := strings.Join(result, "\n")
	assert.Contains(t, resultStr, "HOME=", "HOME should be allowed")
	assert.Contains(t, resultStr, "PATH=", "PATH should be allowed")

	// Sensitive vars should be blocked.
	assert.NotContains(t, resultStr, "ANTHROPIC_API_KEY", "API keys should be blocked")
	assert.NotContains(t, resultStr, "OPENAI_API_KEY", "API keys should be blocked")
	assert.NotContains(t, resultStr, "AWS_SECRET_ACCESS_KEY", "secrets should be blocked")
	assert.NotContains(t, resultStr, "GITHUB_TOKEN", "tokens should be blocked")
	assert.NotContains(t, resultStr, "CUSTOM_VAR", "unknown vars should be blocked")
}

func TestSanitizeEnv_AllowsKnownSafeVars(t *testing.T) {
	t.Parallel()
	env := []string{
		"HOME=/home/user",
		"PATH=/usr/bin",
		"GOPATH=/go",
		"LANG=en_US.UTF-8",
		"HTTP_PROXY=http://proxy:8080",
		"KEEL_CUSTOM=value",
	}

	result := sanitizeEnv(env)
	// All of these should survive.
	assert.Len(t, result, len(env), "all known-safe vars should survive")
}

func TestSanitizeEnv_EmptyInput(t *testing.T) {
	t.Parallel()
	result := sanitizeEnv(nil)
	assert.Empty(t, result)
}

func TestIsAllowedEnvVar_AllowedVars(t *testing.T) {
	t.Parallel()
	// Test explicit allowed vars
	assert.True(t, isAllowedEnvVar("HOME=/home/user"), "HOME should be allowed")
	assert.True(t, isAllowedEnvVar("PATH=/usr/bin"), "PATH should be allowed")
	assert.True(t, isAllowedEnvVar("GOPATH=/go"), "GOPATH should be allowed")
	assert.True(t, isAllowedEnvVar("HTTP_PROXY=http://proxy:8080"), "HTTP_PROXY should be allowed")
}

func TestIsAllowedEnvVar_KeelPrefix(t *testing.T) {
	t.Parallel()
	// KEEL_* variables should always be allowed
	assert.True(t, isAllowedEnvVar("KEEL_CUSTOM=value"), "KEEL_* should be allowed")
	assert.True(t, isAllowedEnvVar("KEEL_DEP_DIR=/path"), "KEEL_* should be allowed")
	assert.True(t, isAllowedEnvVar("KEEL_DEP_NAME=foo"), "KEEL_* should be allowed")
}

func TestIsAllowedEnvVar_BlockedVars(t *testing.T) {
	t.Parallel()
	// Sensitive/unknown variables should be blocked
	assert.False(t, isAllowedEnvVar("ANTHROPIC_API_KEY=secret"), "API keys should be blocked")
	assert.False(t, isAllowedEnvVar("GITHUB_TOKEN=ghp_123"), "tokens should be blocked")
	assert.False(t, isAllowedEnvVar("CUSTOM_VAR=foo"), "unknown vars should be blocked")
	assert.False(t, isAllowedEnvVar("DEBUG=1"), "unknown vars should be blocked")
}

func TestIsAllowedEnvVar_NoEqualsSign(t *testing.T) {
	t.Parallel()
	// An entry without '=' should not be allowed.
	assert.False(t, isAllowedEnvVar("HOME"), "entry without = should be blocked")
}

// ---------------------------------------------------------------------------
// executeInstall sanitization integration test
// ---------------------------------------------------------------------------

func TestExecuteInstall_DoesNotExposeSecrets(t *testing.T) {
	// Set a sensitive env var, then verify the install command cannot see it.
	t.Setenv("ANTHROPIC_API_KEY", "test-secret-key")

	targetDir := t.TempDir()
	outFile := filepath.Join(targetDir, "env_dump")

	// The install command dumps its entire environment to a file.
	installCmd := fmt.Sprintf("env > %s", outFile)

	err := executeInstall("test-dep", installCmd, targetDir)
	require.NoError(t, err)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err, "reading env dump")

	envOutput := string(data)
	assert.NotContains(t, envOutput, "ANTHROPIC_API_KEY",
		"ANTHROPIC_API_KEY should not be visible to install commands")
	assert.NotContains(t, envOutput, "test-secret-key",
		"secret value should not be visible to install commands")

	// Verify KEEL_DEP_DIR and KEEL_DEP_NAME are injected.
	assert.Contains(t, envOutput, "KEEL_DEP_DIR=",
		"KEEL_DEP_DIR should be injected into install commands")
	assert.Contains(t, envOutput, "KEEL_DEP_NAME=test-dep",
		"KEEL_DEP_NAME should be injected into install commands")
}

func TestSanitizeEnv_AllowlistApproach(t *testing.T) {
	t.Parallel()
	// Build an env with mix of allowed, sensitive, and unknown vars
	env := []string{
		"HOME=/home/user",              // allowed
		"PATH=/usr/bin",                // allowed
		"GOPATH=/go",                   // allowed
		"ANTHROPIC_API_KEY=secret",     // blocked (sensitive)
		"GITHUB_TOKEN=ghp_123",         // blocked (sensitive)
		"CUSTOM_VAR=foo",               // blocked (unknown)
		"KEEL_CUSTOM=bar",              // allowed (KEEL_ prefix)
	}

	result := sanitizeEnv(env)

	// Only allowed vars should survive (HOME, PATH, GOPATH, KEEL_CUSTOM)
	assert.Len(t, result, 4, "only allowed vars should survive")
	resultStr := strings.Join(result, "\n")
	assert.Contains(t, resultStr, "HOME=")
	assert.Contains(t, resultStr, "PATH=")
	assert.Contains(t, resultStr, "GOPATH=")
	assert.Contains(t, resultStr, "KEEL_CUSTOM=")
	assert.NotContains(t, resultStr, "ANTHROPIC_API_KEY")
	assert.NotContains(t, resultStr, "GITHUB_TOKEN")
	assert.NotContains(t, resultStr, "CUSTOM_VAR")
}

func TestSanitizeEnv_ToolchainVars(t *testing.T) {
	t.Parallel()
	// Test that toolchain-specific vars are allowed
	env := []string{
		"GOPATH=/go",
		"GOROOT=/usr/local/go",
		"NPM_CONFIG_PREFIX=/usr/local",
		"CARGO_HOME=/usr/local/cargo",
		"PIP_INDEX_URL=https://pypi.org/simple",
	}

	result := sanitizeEnv(env)
	assert.Len(t, result, len(env), "all toolchain vars should be allowed")
}
