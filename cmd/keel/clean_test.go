package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/globaldeps"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestCleanCmd_RemovesCompiledOutput(t *testing.T) {
	dir := t.TempDir()
	mf := manifest.CompiledManifest{Agent: "claude", FileOps: []manifest.FileRecord{{Output: ".claude/settings.json", Strategy: "deep-merge"}, {Output: "CLAUDE.md", Strategy: "concatenate"}}}
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	data, err := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(data, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("instructions"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "custom.md"), []byte("user"), 0644))

	cmd := newCleanCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{dir})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Cleaned")
	assert.NoFileExists(t, filepath.Join(dir, "CLAUDE.md"))
	assert.NoFileExists(t, filepath.Join(claudeDir, "settings.json"))
	assert.NoFileExists(t, filepath.Join(claudeDir, ".keel.json"))
	assert.FileExists(t, filepath.Join(claudeDir, "custom.md"))
}

func TestCleanCmd_MultiAgentClean(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	cm := manifest.CompiledManifest{Agent: "claude", FileOps: []manifest.FileRecord{{Output: ".claude/settings.json", Strategy: "deep-merge"}, {Output: "CLAUDE.md", Strategy: "concatenate"}}}
	cd, err := json.MarshalIndent(&cm, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(cd, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("claude"), 0644))

	cursorDir := filepath.Join(dir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))
	crm := manifest.CompiledManifest{Agent: "cursor", FileOps: []manifest.FileRecord{{Output: ".cursor/rules/00-keel.mdc", Strategy: "concatenate"}, {Output: ".cursor/mcp.json", Strategy: "deep-merge"}}}
	crd, err := json.MarshalIndent(&crm, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, ".keel.json"), append(crd, '\n'), 0644))
	rulesDir := filepath.Join(cursorDir, "rules")
	require.NoError(t, os.MkdirAll(rulesDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rulesDir, "00-keel.mdc"), []byte("cursor rules"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "mcp.json"), []byte("{}"), 0644))

	cmd := newCleanCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{dir})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Cleaned")
	assert.NoFileExists(t, filepath.Join(dir, "CLAUDE.md"))
	assert.NoFileExists(t, filepath.Join(claudeDir, "settings.json"))
	assert.NoFileExists(t, filepath.Join(claudeDir, ".keel.json"))
	assert.NoFileExists(t, filepath.Join(rulesDir, "00-keel.mdc"))
	assert.NoFileExists(t, filepath.Join(cursorDir, "mcp.json"))
	assert.NoFileExists(t, filepath.Join(cursorDir, ".keel.json"))
}

func TestCleanCmd_MultiAgentGlobalDeregistration(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	cm := manifest.CompiledManifest{Agent: "claude", FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}}}
	cd, err := json.MarshalIndent(&cm, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(cd, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("instructions"), 0644))

	cursorDir := filepath.Join(dir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))
	crm := manifest.CompiledManifest{Agent: "cursor", FileOps: []manifest.FileRecord{{Output: ".cursor/mcp.json", Strategy: "deep-merge"}}}
	crd, err := json.MarshalIndent(&crm, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, ".keel.json"), append(crd, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "mcp.json"), []byte("{}"), 0644))

	globalDir := t.TempDir()
	globalManifestPath := filepath.Join(globalDir, ".manifest.json")
	absDir, _ := filepath.Abs(dir)
	gm := &globaldeps.Manifest{Deps: map[string]globaldeps.Dep{"shared-tool": {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{absDir, "/other/project"}, Method: "download"}}}
	require.NoError(t, globaldeps.SaveManifest(gm, globalManifestPath))

	loaded, err := globaldeps.LoadManifest(globalManifestPath)
	require.NoError(t, err)
	for depName, dep := range loaded.Deps {
		for _, c := range dep.Consumers {
			if c == absDir {
				loaded.DeregisterConsumer(depName, absDir)
				break
			}
		}
	}
	require.NoError(t, globaldeps.SaveManifest(loaded, globalManifestPath))

	reloaded, err := globaldeps.LoadManifest(globalManifestPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"/other/project"}, reloaded.Deps["shared-tool"].Consumers)
}

func TestCleanCmd_NoManifest(t *testing.T) {
	cmd := newCleanCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{t.TempDir()})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "No compiled output to clean")
}
