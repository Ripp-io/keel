package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/globaldeps"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestGlobalList_EmptyManifest(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, runGlobalList(&buf, filepath.Join(t.TempDir(), ".manifest.json")))
	assert.Contains(t, buf.String(), "No global dependencies installed.")
}

func TestGlobalList_PopulatedManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".manifest.json")
	m := &globaldeps.Manifest{Deps: map[string]globaldeps.Dep{
		"coordination-cli": {InstalledAt: "2026-04-30T10:00:00Z", Consumers: []string{"/projects/alpha", "/projects/beta"}, Method: "download"},
		"yamllint":         {InstalledAt: "2026-05-09T10:00:00Z", Consumers: []string{"/projects/alpha"}, Method: "install"},
	}}
	require.NoError(t, globaldeps.SaveManifest(m, manifestPath))
	var buf bytes.Buffer
	require.NoError(t, runGlobalList(&buf, manifestPath))
	output := buf.String()
	assert.Contains(t, output, "Global dependencies")
	assert.Contains(t, output, dir+"/")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "CONSUMERS")
	assert.Contains(t, output, "INSTALLED")
	assert.Contains(t, output, "coordination-cli")
	assert.Contains(t, output, "yamllint")
	assert.Contains(t, output, "2 deps, 3 consumers")
}

func TestGlobalList_SortedByName(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".manifest.json")
	m := &globaldeps.Manifest{Deps: map[string]globaldeps.Dep{
		"zebra":  {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{"/a"}},
		"alpha":  {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{"/b"}},
		"middle": {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{"/c"}},
	}}
	require.NoError(t, globaldeps.SaveManifest(m, manifestPath))
	var buf bytes.Buffer
	require.NoError(t, runGlobalList(&buf, manifestPath))
	output := buf.String()
	alphaIdx := strings.Index(output, "alpha")
	middleIdx := strings.Index(output, "middle")
	zebraIdx := strings.Index(output, "zebra")
	require.Positive(t, alphaIdx)
	require.Positive(t, middleIdx)
	require.Positive(t, zebraIdx)
	assert.Less(t, alphaIdx, middleIdx)
	assert.Less(t, middleIdx, zebraIdx)
}

func TestGlobalClean_NoOrphans(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".manifest.json")
	require.NoError(t, globaldeps.SaveManifest(&globaldeps.Manifest{Deps: map[string]globaldeps.Dep{"rg": {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{dir}}}}, manifestPath))
	var buf bytes.Buffer
	require.NoError(t, runGlobalClean(&buf, strings.NewReader(""), manifestPath, true))
	assert.Contains(t, buf.String(), "No orphaned global dependencies.")
}

func TestGlobalClean_EmptyManifest(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, runGlobalClean(&buf, strings.NewReader(""), filepath.Join(t.TempDir(), ".manifest.json"), true))
	assert.Contains(t, buf.String(), "No orphaned global dependencies.")
}

func TestGlobalClean_RemovesOrphansWithYes(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".manifest.json")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "orphan-tool"), []byte("binary"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "another-tool"), []byte("binary"), 0755))
	m := &globaldeps.Manifest{Deps: map[string]globaldeps.Dep{
		"orphan-tool":  {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{}},
		"another-tool": {InstalledAt: "2026-05-10T10:00:00Z", Consumers: []string{}},
		"kept-tool":    {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{dir}},
	}}
	require.NoError(t, globaldeps.SaveManifest(m, manifestPath))
	var buf bytes.Buffer
	require.NoError(t, runGlobalClean(&buf, strings.NewReader(""), manifestPath, true))
	output := buf.String()
	assert.Contains(t, output, "removed orphan-tool")
	assert.Contains(t, output, "removed another-tool")
	assert.Contains(t, output, "Removed 2 global dependencies.")
	assert.NoFileExists(t, filepath.Join(dir, "orphan-tool"))
	assert.NoFileExists(t, filepath.Join(dir, "another-tool"))
	reloaded, err := globaldeps.LoadManifest(manifestPath)
	require.NoError(t, err)
	assert.NotContains(t, reloaded.Deps, "orphan-tool")
	assert.NotContains(t, reloaded.Deps, "another-tool")
	assert.Contains(t, reloaded.Deps, "kept-tool")
}

func TestGlobalClean_InteractivePrompt(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".manifest.json")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "yes-tool"), []byte("binary"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "no-tool"), []byte("binary"), 0755))
	m := &globaldeps.Manifest{Deps: map[string]globaldeps.Dep{
		"no-tool":  {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{}},
		"yes-tool": {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{}},
	}}
	require.NoError(t, globaldeps.SaveManifest(m, manifestPath))
	var buf bytes.Buffer
	require.NoError(t, runGlobalClean(&buf, strings.NewReader("n\ny\n"), manifestPath, false))
	output := buf.String()
	assert.NotContains(t, output, "removed no-tool")
	assert.Contains(t, output, "removed yes-tool")
	assert.FileExists(t, filepath.Join(dir, "no-tool"))
	assert.NoFileExists(t, filepath.Join(dir, "yes-tool"))
}

func TestGlobalClean_CleansDeadConsumers(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".manifest.json")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rg"), []byte("binary"), 0755))
	require.NoError(t, globaldeps.SaveManifest(&globaldeps.Manifest{Deps: map[string]globaldeps.Dep{"rg": {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{"/does/not/exist/project"}}}}, manifestPath))
	var buf bytes.Buffer
	require.NoError(t, runGlobalClean(&buf, strings.NewReader(""), manifestPath, true))
	assert.Contains(t, buf.String(), "removed rg")
}

func TestCleanCmd_DeregistersFromGlobalManifest(t *testing.T) {
	projectDir := t.TempDir()
	claudeDir := filepath.Join(projectDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	mf := manifest.CompiledManifest{Agent: "claude", FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}}}
	data, _ := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(data, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte("instructions"), 0644))
	globalDir := t.TempDir()
	globalManifestPath := filepath.Join(globalDir, ".manifest.json")
	absProject, _ := filepath.Abs(projectDir)
	gm := &globaldeps.Manifest{Deps: map[string]globaldeps.Dep{
		"shared-tool": {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{absProject, "/other/project"}, Method: "download"},
		"solo-tool":   {InstalledAt: "2026-05-12T10:00:00Z", Consumers: []string{absProject}, Method: "install"},
	}}
	require.NoError(t, globaldeps.SaveManifest(gm, globalManifestPath))
	loaded, err := globaldeps.LoadManifest(globalManifestPath)
	require.NoError(t, err)
	changed := false
	for depName, dep := range loaded.Deps {
		for _, c := range dep.Consumers {
			if c == absProject {
				changed = true
				orphaned := loaded.DeregisterConsumer(depName, absProject)
				if depName == "solo-tool" {
					assert.True(t, orphaned)
				}
				if depName == "shared-tool" {
					assert.False(t, orphaned)
				}
				break
			}
		}
	}
	assert.True(t, changed)
	require.NoError(t, globaldeps.SaveManifest(loaded, globalManifestPath))
	reloaded, err := globaldeps.LoadManifest(globalManifestPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"/other/project"}, reloaded.Deps["shared-tool"].Consumers)
	assert.Empty(t, reloaded.Deps["solo-tool"].Consumers)
}

func TestCleanCmd_NoGlobalConsumers(t *testing.T) {
	projectDir := t.TempDir()
	claudeDir := filepath.Join(projectDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	mf := manifest.CompiledManifest{Agent: "claude", FileOps: []manifest.FileRecord{{Output: "CLAUDE.md", Strategy: "concatenate"}}}
	data, _ := json.MarshalIndent(&mf, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".keel.json"), append(data, '\n'), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte("instructions"), 0644))
	cmd := newCleanCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{projectDir})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Cleaned")
}

func TestGlobalCmd_HasSubcommands(t *testing.T) {
	cmd := newGlobalCmd()
	assert.Equal(t, "global", cmd.Use)
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	assert.True(t, subNames["list"])
	assert.True(t, subNames["clean"])
}
