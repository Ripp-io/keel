package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

func TestCompile_DepsAndBin_CrossFunctional(t *testing.T) {
	remote.DisableSSRFForTest()
	t.Cleanup(remote.RestoreSSRFForTest)

	testScript := []byte("#!/bin/sh\necho \"keel-test-tool v1.0\"\n")
	scriptHash := sha256.Sum256(testScript)
	scriptSHA256 := hex.EncodeToString(scriptHash[:])

	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"tools/archive-tool": string(testScript),
		"README.md":          "archive readme",
	})
	archiveData, err := os.ReadFile(archivePath)
	require.NoError(t, err, "reading test archive")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/raw-tool.sh":
			w.Write(testScript)
		case "/archive-tool.tar.gz":
			w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rootDir := t.TempDir()

	scriptSourcePath := filepath.Join(rootDir, "source-tool.sh")
	require.NoError(t, os.WriteFile(scriptSourcePath, testScript, 0755))

	checkEnvScript := []byte("#!/bin/sh\necho \"env: $KEEL_ENV\"\n")

	layer2Dir := filepath.Join(rootDir, "02-team-tools")
	require.NoError(t, os.MkdirAll(layer2Dir, 0755))
	layer2Keel := fmt.Sprintf("schema: \"1\"\nname: team-tools\n\ndeps:\n  raw-tool:\n    download: \"%s/raw-tool.sh\"\n    sha256: \"%s\"\n  archive-tool:\n    download: \"%s/archive-tool.tar.gz\"\n    executable: \"tools/archive-tool\"\n  global-tool:\n    download: \"%s/raw-tool.sh\"\n    global: true\n", srv.URL, scriptSHA256, srv.URL, srv.URL)
	require.NoError(t, os.WriteFile(filepath.Join(layer2Dir, "keel.yml"), []byte(layer2Keel), 0644))

	layer3Dir := filepath.Join(rootDir, "03-project")
	require.NoError(t, os.MkdirAll(layer3Dir, 0755))

	mcpJSON := "{\n  \"mcpServers\": {\n    \"local-tool-server\": {\n      \"command\": \"${keel:keel.deps.installed-tool.path}\",\n      \"args\": [\"--serve\"]\n    }\n  }\n}"
	require.NoError(t, os.WriteFile(filepath.Join(layer3Dir, "mcp.json"), []byte(mcpJSON), 0644))

	hooksJSON := "{\n  \"PreToolUse\": [\n    {\n      \"matcher\": \"Bash(*)\",\n      \"hooks\": [\n        {\n          \"type\": \"command\",\n          \"command\": \"${keel:keel.bin.project}/check-env.sh\"\n        }\n      ]\n    }\n  ]\n}"
	require.NoError(t, os.WriteFile(filepath.Join(layer3Dir, "hooks.json"), []byte(hooksJSON), 0644))

	instrContent := "# Project Setup\n\nThe archive tool is available at: ${keel:keel.deps.archive-tool.path}\nThe raw tool dir is: ${keel:keel.deps.raw-tool.dir}\n"
	require.NoError(t, os.WriteFile(filepath.Join(layer3Dir, "instructions.md"), []byte(instrContent), 0644))

	layer3Keel := "schema: \"1\"\nname: project\n\ninstructions:\n  - source: instructions.md\nmcp:\n  servers:\n    - source: mcp.json\nhooks:\n  - source: hooks.json\n"
	require.NoError(t, os.WriteFile(filepath.Join(layer3Dir, "keel.yml"), []byte(layer3Keel), 0644))

	buildLayer1 := func(t *testing.T, targetDir string) string {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "01-company-tools")
		binDir := filepath.Join(dir, "bin")
		require.NoError(t, os.MkdirAll(binDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(binDir, "check-env.sh"), checkEnvScript, 0755))
		keel := fmt.Sprintf("schema: \"1\"\nname: company-tools\n\nbin:\n  - source: bin\n\ndeps:\n  installed-tool:\n    install: \"cp %s %s/installed-tool && chmod +x %s/installed-tool\"\n", scriptSourcePath, targetDir, targetDir)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keel), 0644))
		return dir
	}

	t.Run("AllowInstall", func(t *testing.T) {
		outputDir := t.TempDir()
		installTargetDir := filepath.Join(outputDir, ".keel", "bin")
		layer1Dir := buildLayer1(t, installTargetDir)
		layers := []config.Layer{
			{Path: layer1Dir, Type: "company", Format: config.FormatKeel},
			{Path: layer2Dir, Type: "team", Format: config.FormatKeel},
			{Path: layer3Dir, Type: "project", Format: config.FormatKeel},
		}
		result, err := Compile("claude", layers, outputDir, CompileOptions{AllowInstall: true, NoPrune: true})
		require.NoError(t, err, "Compile()")

		// 1. bin/ overlay
		checkEnvPath := filepath.Join(outputDir, ".keel", "bin", "check-env.sh")
		assertFileExists(t, checkEnvPath)
		assertFileExecutable(t, checkEnvPath)
		testutil.AssertFileContent(t, checkEnvPath, string(checkEnvScript))

		// 2. install dep
		installedToolPath := filepath.Join(outputDir, ".keel", "bin", "installed-tool")
		assertFileExists(t, installedToolPath)
		assertFileExecutable(t, installedToolPath)
		testutil.AssertFileContent(t, installedToolPath, string(testScript))

		// 3. download dep (raw binary)
		rawToolPath := filepath.Join(outputDir, ".keel", "bin", "raw-tool")
		assertFileExists(t, rawToolPath)
		assertFileExecutable(t, rawToolPath)
		testutil.AssertFileContent(t, rawToolPath, string(testScript))

		// 4. download dep (archive + executable field)
		archiveToolPath := filepath.Join(outputDir, ".keel", "bin", "archive-tool")
		assertFileExists(t, archiveToolPath)
		assertFileExecutable(t, archiveToolPath)
		testutil.AssertFileContent(t, archiveToolPath, string(testScript))

		// 5. global dep
		var globalRec *depRecordSnapshot
		for _, dep := range result.Dependencies.InstalledDeps {
			if dep.Name == "global-tool" {
				globalRec = &depRecordSnapshot{dep.Name, dep.Scope, dep.Method, dep.Path}
				break
			}
		}
		require.NotNil(t, globalRec, "global-tool in InstalledDeps")
		assert.Equal(t, "global", globalRec.scope, "global-tool scope")
		assert.Contains(t, globalRec.path, ".keel/bin/", "global-tool path should contain .keel/bin/")

		// 6. MCP variable resolution
		mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
		require.NoError(t, err, "reading .mcp.json")
		var mcp map[string]any
		require.NoError(t, json.Unmarshal(mcpData, &mcp), "parsing .mcp.json")
		servers := mcp["mcpServers"].(map[string]any)
		localServer := servers["local-tool-server"].(map[string]any)
		mcpCommand := localServer["command"].(string)
		assert.NotContains(t, mcpCommand, "${keel:", "MCP command should have resolved variable")
		assert.Equal(t, installedToolPath, mcpCommand, "MCP command path")

		// 7. Hook variable resolution
		settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
		require.NoError(t, err, "reading settings.json")
		var settings map[string]any
		require.NoError(t, json.Unmarshal(settingsData, &settings), "parsing settings.json")
		hooks := settings["hooks"].(map[string]any)
		preToolUse := hooks["PreToolUse"].([]any)
		hookEntry := preToolUse[0].(map[string]any)
		hookHooks := hookEntry["hooks"].([]any)
		hookCmd := hookHooks[0].(map[string]any)
		hookCommand := hookCmd["command"].(string)
		assert.NotContains(t, hookCommand, "${keel:", "hook command should have resolved variable")
		expectedHookCmd := filepath.Join(outputDir, ".keel", "bin") + "/check-env.sh"
		assert.Equal(t, expectedHookCmd, hookCommand, "hook command path")

		// 8. Instruction variable resolution
		instrData, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
		require.NoError(t, err, "reading CLAUDE.md")
		instrText := string(instrData)
		assert.NotContains(t, instrText, "${keel:", "instructions should have resolved variables")
		assert.Contains(t, instrText, archiveToolPath, "instructions should contain resolved archive-tool path")
		expectedRawDir := filepath.Join(outputDir, ".keel", "bin")
		assert.Contains(t, instrText, expectedRawDir, "instructions should contain resolved raw-tool dir")

		// 9. .keel.json manifest
		manifestData, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
		require.NoError(t, err, "reading .keel.json")
		manifestText := string(manifestData)
		assert.Contains(t, manifestText, "\"installed_deps\"", ".keel.json should contain installed_deps key")
		for _, depName := range []string{"installed-tool", "raw-tool", "archive-tool", "global-tool"} {
			assert.Contains(t, manifestText, depName, ".keel.json should reference dep %q", depName)
		}

		// Verify dep records have correct methods
		depsByName := make(map[string]string)
		for _, dep := range result.Dependencies.InstalledDeps {
			depsByName[dep.Name] = dep.Method
		}
		expectedMethods := map[string]string{
			"installed-tool": "install", "raw-tool": "download",
			"archive-tool": "download", "global-tool": "download",
		}
		for name, wantMethod := range expectedMethods {
			gotMethod, ok := depsByName[name]
			if assert.True(t, ok, "dep record for %q", name) {
				assert.Equal(t, wantMethod, gotMethod, "dep %q method", name)
			}
		}
	})

	t.Run("SkippedWithoutAllowInstall", func(t *testing.T) {
		outputDir := t.TempDir()
		installTargetDir := filepath.Join(outputDir, ".keel", "bin")
		layer1Dir := buildLayer1(t, installTargetDir)
		layers := []config.Layer{
			{Path: layer1Dir, Type: "company", Format: config.FormatKeel},
			{Path: layer2Dir, Type: "team", Format: config.FormatKeel},
			{Path: layer3Dir, Type: "project", Format: config.FormatKeel},
		}
		result, err := Compile("claude", layers, outputDir, CompileOptions{AllowInstall: false, NoPrune: true})
		require.NoError(t, err, "Compile()")

		// bin/ overlay still works without --allow-install
		checkEnvPath := filepath.Join(outputDir, ".keel", "bin", "check-env.sh")
		assertFileExists(t, checkEnvPath)
		assertFileExecutable(t, checkEnvPath)
		testutil.AssertFileContent(t, checkEnvPath, string(checkEnvScript))

		// All deps are recorded as skipped
		for _, dep := range result.Dependencies.InstalledDeps {
			assert.Equal(t, "skipped", dep.Method, "dep %q should be skipped without --allow-install", dep.Name)
		}
		assert.Len(t, result.Dependencies.InstalledDeps, 4, "dep records (all skipped)")

		// No dep binaries are installed (only bin/ overlay files exist)
		for _, depName := range []string{"installed-tool", "raw-tool", "archive-tool"} {
			depPath := filepath.Join(outputDir, ".keel", "bin", depName)
			_, err := os.Stat(depPath)
			assert.True(t, os.IsNotExist(err), "dep binary %q should NOT exist without --allow-install", depName)
		}

		// Per-dep variables are still injected
		mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
		require.NoError(t, err, "reading .mcp.json")
		var mcp map[string]any
		require.NoError(t, json.Unmarshal(mcpData, &mcp), "parsing .mcp.json")
		servers := mcp["mcpServers"].(map[string]any)
		localServer := servers["local-tool-server"].(map[string]any)
		mcpCommand := localServer["command"].(string)
		assert.NotContains(t, mcpCommand, "${keel:", "MCP command should have resolved variable even without --allow-install")
		expectedInstalledPath := filepath.Join(outputDir, ".keel", "bin", "installed-tool")
		assert.Equal(t, expectedInstalledPath, mcpCommand, "MCP command path")

		// Instructions have resolved paths
		instrData, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
		require.NoError(t, err, "reading CLAUDE.md")
		assert.NotContains(t, string(instrData), "${keel:", "instructions should have resolved variables even without --allow-install")

		// Hooks have resolved paths
		settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
		require.NoError(t, err, "reading settings.json")
		assert.NotContains(t, string(settingsData), "${keel:", "hooks should have resolved variables even without --allow-install")

		// .keel.json manifest records skipped deps
		manifestData, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
		require.NoError(t, err, "reading .keel.json")
		manifestText := string(manifestData)
		assert.Contains(t, manifestText, "\"installed_deps\"", ".keel.json should contain installed_deps even for skipped deps")
		assert.Contains(t, manifestText, "\"skipped\"", ".keel.json should contain skipped method for deps without --allow-install")
	})
}

type depRecordSnapshot struct {
	name   string
	scope  string
	method string
	path   string
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.NoError(t, err, "expected file to exist at %s", path)
}

func assertFileExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if !assert.NoError(t, err, "cannot stat %s", path) {
		return
	}
	assert.NotEqual(t, os.FileMode(0), info.Mode().Perm()&0111, "expected %s to be executable, got permissions %o", path, info.Mode().Perm())
}
