package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

func runExplain(t *testing.T, outputDir string, args ...string) (string, error) {
	t.Helper()
	cmd := newExplainCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append([]string{outputDir}, args...))
	err := cmd.Execute()
	return buf.String(), err
}

const fullFixture = `
agent: claude
compiled_at: "2026-05-05T14:39:12Z"
keel_version: dev
name: My Demo
layers:
  - path: /keel/layers/00-base
    type: config
    format: keel
    version: v1
  - path: /keel/layers/01-company
    type: config
    format: keel
    version: v1
  - path: /keel/layers/03-team
    type: config
    format: keel
    version: v1
  - path: /keel/layers/04-project
    type: config
    format: keel
    version: v1
file_ops:
  - output: CLAUDE.md
    strategy: concatenate
    sources:
      - layer: 00-base
        file: instructions.md
      - layer: 01-company
        file: instructions.md
      - layer: 03-team
        file: instructions.md
      - layer: 04-project
        file: instructions.md
  - output: .claude/settings.json
    strategy: deep-merge
    sources:
      - layer: 01-company
        file: settings.claude.json
      - layer: 04-project
        file: settings.claude.json
  - output: .claude/commands/security-scan.md
    strategy: overlay
    sources:
      - layer: 01-company
        file: security-scan.md
  - output: .claude/commands/deploy.md
    strategy: overlay
    sources:
      - layer: 03-team
        file: deploy.md
  - output: .mcp.json
    strategy: deep-merge
    sources:
      - layer: 01-company
        file: mcp-sonarqube.json
      - layer: 03-team
        file: mcp-context7.json
force_locked:
  - category: command
    key: security-scan.md
    locked_by: 01-company
  - category: mcp
    key: sonarqube
    locked_by: 01-company
  - category: hook
    key: PreToolUse
    locked_by: 01-company
repos:
  gateway:
    url: "git@example.com:demo/gateway.git"
    path: gateway
    branch: main
    required: true
  ui:
    url: "git@example.com:demo/ui.git"
    path: ui
    branch: main
  service:
    url: "git@example.com:demo/service.git"
    path: service
    branch: main
    required: true
`

func TestExplainFull(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplain(t, dir)
	require.NoError(t, err)
	assert.Contains(t, got, "Compiled: claude")
	assert.Contains(t, got, "2026-05-05T14:39:12Z")
	assert.Contains(t, got, "keel dev")
	assert.Contains(t, got, "Layers (4):")
	assert.Contains(t, got, "/keel/layers/00-base")
	assert.Contains(t, got, "/keel/layers/04-project")
	assert.Contains(t, got, "Files (5):")
	assert.Contains(t, got, "CLAUDE.md")
	assert.Contains(t, got, "concatenate")
	assert.Contains(t, got, "deep-merge")
	assert.Contains(t, got, "Force-locks (3):")
	assert.Contains(t, got, "locked by 01-company")
	assert.Contains(t, got, "Repos (3):")
	assert.Contains(t, got, "git@example.com:demo/gateway.git")
}

func TestExplainFiles(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplain(t, dir, "files")
	require.NoError(t, err)
	assert.Contains(t, got, "Files (5):")
	assert.Contains(t, got, "CLAUDE.md")
	assert.NotContains(t, got, "Layers (")
	assert.NotContains(t, got, "Force-locks (")
	assert.NotContains(t, got, "Compiled:")
	assert.NotContains(t, got, "Repos (")
}

func TestExplainLocks(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplain(t, dir, "locks")
	require.NoError(t, err)
	assert.Contains(t, got, "Force-locks (3):")
	assert.Contains(t, got, "command")
	assert.Contains(t, got, "security-scan.md")
	assert.Contains(t, got, "locked by 01-company")
	assert.NotContains(t, got, "Layers (")
	assert.NotContains(t, got, "Files (")
	assert.NotContains(t, got, "Compiled:")
}

func TestExplainLayers(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplain(t, dir, "layers")
	require.NoError(t, err)
	assert.Contains(t, got, "Layers (4):")
	assert.Contains(t, got, "1.")
	assert.Contains(t, got, "4.")
	assert.Contains(t, got, "/keel/layers/00-base")
	assert.Contains(t, got, "keel v1")
	assert.NotContains(t, got, "Files (")
	assert.NotContains(t, got, "Force-locks (")
	assert.NotContains(t, got, "Compiled:")
}

func TestExplainEmpty(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "agent: claude\ncompiled_at: \"2026-05-05T14:39:12Z\"\nkeel_version: dev\n")
	got, err := runExplain(t, dir)
	require.NoError(t, err)
	assert.Contains(t, got, "Compiled: claude")
	assert.Contains(t, got, "Layers (0):")
	assert.Contains(t, got, "Files (0):")
	assert.Contains(t, got, "Force-locks (0):")
	assert.Contains(t, got, "(none)")
	assert.NotContains(t, got, "Repos")
}

func TestExplainMissingKeelState(t *testing.T) {
	_, err := runExplain(t, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run 'keel compile' first")
}

func TestExplainInvalidSection(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "agent: claude\ncompiled_at: \"2026-05-05T14:39:12Z\"\nkeel_version: dev\n")
	_, err := runExplain(t, dir, "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown section "bogus"`)
}

func TestExplainLockedAnnotation(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplain(t, dir, "files")
	require.NoError(t, err)
	assert.Contains(t, got, "[locked]")
}

func TestExplainDeps(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "agent: claude\ncompiled_at: \"2026-05-12T10:00:00Z\"\nkeel_version: dev\ninstalled_deps:\n  - name: coordination-cli\n    scope: global\n    method: download\n    path: /Users/user/.keel/bin/coordination-cli\n  - name: my-linter\n    scope: project\n    method: install\n    path: /Users/user/project/.keel/bin/my-linter\n  - name: setup-script\n    scope: project\n    method: skipped\n    path: /Users/user/project/.keel/bin/setup-script\n")
	got, err := runExplain(t, dir, "deps")
	require.NoError(t, err)
	assert.Contains(t, got, "Dependencies (3):")
	assert.Contains(t, got, "NAME")
	assert.Contains(t, got, "coordination-cli")
	assert.Contains(t, got, "global")
	assert.Contains(t, got, "download")
	assert.Contains(t, got, "my-linter")
	assert.Contains(t, got, "skipped")
}

func TestExplainDeps_Empty(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "agent: claude\ncompiled_at: \"2026-05-12T10:00:00Z\"\nkeel_version: dev\n")
	got, err := runExplain(t, dir, "deps")
	require.NoError(t, err)
	assert.Contains(t, got, "No dependencies defined.")
}

func TestExplainDeps_InFullOutput(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "agent: claude\ncompiled_at: \"2026-05-12T10:00:00Z\"\nkeel_version: dev\ninstalled_deps:\n  - name: my-tool\n    scope: project\n    method: install\n    path: /tmp/bin/my-tool\n")
	got, err := runExplain(t, dir)
	require.NoError(t, err)
	assert.Contains(t, got, "Dependencies (1):")
	assert.Contains(t, got, "my-tool")
}

func TestExplainLayerVersionFallback(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "agent: claude\ncompiled_at: \"2026-05-05T14:39:12Z\"\nkeel_version: dev\nlayers:\n  - path: /keel/layers/00-base\n    type: config\n    format: keel\n  - path: /keel/layers/01-project\n    type: config\n    format: native\n    version: v1\n")
	got, err := runExplain(t, dir, "layers")
	require.NoError(t, err)
	assert.Contains(t, got, "keel -")
	assert.Contains(t, got, "native v1")
}

func runExplainJSON(t *testing.T, outputDir string, args ...string) (string, error) {
	t.Helper()
	cmd := newExplainCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Build args: --format json [args...] [outputDir]
	fullArgs := []string{"--format", "json"}
	fullArgs = append(fullArgs, args...)
	fullArgs = append(fullArgs, outputDir)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()
	return buf.String(), err
}

func TestExplainJSON_FullManifest(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplainJSON(t, dir)
	require.NoError(t, err)
	var cfg manifest.CompiledManifest
	require.NoError(t, json.Unmarshal([]byte(got), &cfg))
	assert.Equal(t, "claude", cfg.Agent)
	assert.Equal(t, "My Demo", cfg.Name)
	assert.Len(t, cfg.LayerStack, 4)
	assert.Len(t, cfg.FileOps, 5)
	assert.Len(t, cfg.ForceLocked, 3)
	assert.Len(t, cfg.Repos, 3)
}

func TestExplainJSON_Name(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "name: My Demo\nagent: claude\ncompiled_at: \"2026-01-01T00:00:00Z\"\n")
	got, err := runExplainJSON(t, dir, "name")
	require.NoError(t, err)
	assert.Equal(t, "My Demo\n", got)
}

func TestExplainJSON_Repos(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplainJSON(t, dir, "repos")
	require.NoError(t, err)
	var repos map[string]schema.Repo
	require.NoError(t, json.Unmarshal([]byte(got), &repos))
	assert.Len(t, repos, 3)
	gw, ok := repos["gateway"]
	require.True(t, ok)
	assert.Equal(t, "git@example.com:demo/gateway.git", gw.URL)
}

func TestExplainJSON_Repo(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplainJSON(t, dir, "repo", "gateway")
	require.NoError(t, err)
	var repo schema.Repo
	require.NoError(t, json.Unmarshal([]byte(got), &repo))
	assert.Equal(t, "git@example.com:demo/gateway.git", repo.URL)
	assert.Equal(t, "gateway", repo.Path)
	assert.Equal(t, "main", repo.Branch)
}

func TestExplainJSON_Layers(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplainJSON(t, dir, "layers")
	require.NoError(t, err)
	var layers []manifest.LayerRecord
	require.NoError(t, json.Unmarshal([]byte(got), &layers))
	require.Len(t, layers, 4)
	assert.Equal(t, "/keel/layers/00-base", layers[0].Path)
}

func TestExplainJSON_Deps(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "agent: claude\ncompiled_at: \"2026-05-12T10:00:00Z\"\nkeel_version: dev\ninstalled_deps:\n  - name: coordination-cli\n    scope: global\n    method: download\n    path: /Users/user/.keel/bin/coordination-cli\n")
	got, err := runExplainJSON(t, dir, "deps")
	require.NoError(t, err)
	var deps []manifest.InstalledDepRecord
	require.NoError(t, json.Unmarshal([]byte(got), &deps))
	require.Len(t, deps, 1)
	assert.Equal(t, "coordination-cli", deps[0].Name)
}

func TestExplainJSON_Files(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplainJSON(t, dir, "files")
	require.NoError(t, err)
	var files []manifest.FileRecord
	require.NoError(t, json.Unmarshal([]byte(got), &files))
	require.Len(t, files, 5)
	assert.Equal(t, "CLAUDE.md", files[0].Output)
}

func TestExplainJSON_Locks(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, fullFixture)
	got, err := runExplainJSON(t, dir, "locks")
	require.NoError(t, err)
	var locks []manifest.ForceLockRecord
	require.NoError(t, json.Unmarshal([]byte(got), &locks))
	require.Len(t, locks, 3)
	assert.Equal(t, "command", locks[0].Category)
}
