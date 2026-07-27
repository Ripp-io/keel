package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestFrontmatter_CommandForceLocked(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-frontmatter", "company", "project")
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "security-scan.md"))
	require.NoError(t, err, "reading security-scan.md")
	assert.Contains(t, string(content), "frontmatter force-locked, cannot be overridden")
	assert.NotContains(t, string(content), "should be blocked")
	assert.NotContains(t, string(content), "force: true",
		"expected frontmatter to be stripped from compiled output")

	buildPath := filepath.Join(outputDir, ".claude", "commands", "build.md")
	_, err = os.Stat(buildPath)
	assert.NoError(t, err, "expected build.md from project layer to exist")

	deployContent, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "deploy.md"))
	require.NoError(t, err, "reading deploy.md")
	assert.Contains(t, string(deployContent), "should override company")

	found := false
	for _, fl := range result.Governance.ForceLocked {
		if fl.Category == "command" && fl.Key == "security-scan.md" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected force-lock manifest entry for command:security-scan.md from frontmatter")
}

func TestFrontmatter_AgentToolFieldsPreserved(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-frontmatter", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "agents", "code-reviewer.md"))
	require.NoError(t, err, "reading code-reviewer.md")
	contentStr := string(content)

	assert.Contains(t, contentStr, "name: code-reviewer")
	assert.Contains(t, contentStr, "description: Reviews code for security")
	assert.Contains(t, contentStr, "model: sonnet")
	assert.NotContains(t, contentStr, "force:")
	assert.Contains(t, contentStr, "checks for security vulnerabilities")
	assert.NotContains(t, contentStr, "Project override")
}

func TestFrontmatter_CursorCompiler(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-frontmatter", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "commands", "security-scan.md"))
	require.NoError(t, err, "reading security-scan.md")
	assert.Contains(t, string(content), "frontmatter force-locked, cannot be overridden")
	assert.NotContains(t, string(content), "force: true")
}

func TestFrontmatter_InstructionsFrontmatterStripped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(layerDir, "instructions.md"),
		[]byte("---\nforce: true\nscope: global\n---\nInstruction content only.\n"),
		0644,
	))
	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "test",
		Format: config.FormatKeel,
	}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	contentStr := string(content)
	assert.Contains(t, contentStr, "Instruction content only.")
	assert.NotContains(t, contentStr, "force: true")
	assert.NotContains(t, contentStr, "scope: global")
}

func TestFrontmatter_ForceFalseNotLocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layer1Dir := filepath.Join(dir, "layer1")
	layer1Cmds := filepath.Join(layer1Dir, "commands")
	require.NoError(t, os.MkdirAll(layer1Cmds, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(layer1Cmds, "scan.md"),
		[]byte("---\nforce: false\n---\nLayer1 scan.\n"),
		0644,
	))

	layer2Dir := filepath.Join(dir, "layer2")
	layer2Cmds := filepath.Join(layer2Dir, "commands")
	require.NoError(t, os.MkdirAll(layer2Cmds, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(layer2Cmds, "scan.md"),
		[]byte("Layer2 override of scan.\n"),
		0644,
	))

	layers := []config.Layer{
		{Path: layer1Dir, Type: "base", Format: config.FormatKeel},
		{Path: layer2Dir, Type: "project", Format: config.FormatKeel},
	}
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "scan.md"))
	require.NoError(t, err, "reading scan.md")
	assert.Contains(t, string(content), "Layer2 override")

	hasScanLock := false
	for _, fl := range result.Governance.ForceLocked {
		if fl.Category == "command" && fl.Key == "scan.md" {
			hasScanLock = true
		}
	}
	assert.False(t, hasScanLock, "force: false should NOT create a force-lock entry")
}

func TestFrontmatter_NonMarkdownFileUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	cmdsDir := filepath.Join(layerDir, "commands")
	require.NoError(t, os.MkdirAll(cmdsDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(cmdsDir, "config.txt"),
		[]byte("---\nforce: true\n---\nText content.\n"),
		0644,
	))
	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "test",
		Format: config.FormatKeel,
	}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "config.txt"))
	require.NoError(t, err, "reading config.txt")
	assert.Contains(t, string(content), "force: true",
		"expected .txt file to be copied unchanged (no frontmatter processing)")
}

func TestFrontmatter_MdcToolFieldsPreserved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	rulesDir := filepath.Join(layerDir, "rules")
	require.NoError(t, os.MkdirAll(rulesDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(rulesDir, "pci-compliance.mdc"),
		[]byte("---\ndescription: PCI compliance rules\nglobs: \"**/*.go\"\nalwaysApply: true\nforce: true\n---\nPCI compliance content.\n"),
		0644,
	))
	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "test",
		Format: config.FormatKeel,
	}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "rules", "pci-compliance.mdc"))
	require.NoError(t, err, "reading pci-compliance.mdc")
	contentStr := string(content)

	assert.Contains(t, contentStr, "description: PCI compliance rules")
	assert.Contains(t, contentStr, "alwaysApply: true")
	assert.NotContains(t, contentStr, "force:")
	assert.Contains(t, contentStr, "PCI compliance content.")
}
