// compiler_snippet_test.go — snippet resolution integration tests covering
// single-layer includes, cross-layer references, agent file snippets,
// missing snippet errors, output exclusion, and native layer snippets.
package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestCompileClaude_SnippetResolution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	snippetsDir := filepath.Join(layerDir, "snippets")
	require.NoError(t, os.MkdirAll(snippetsDir, 0755))

	keelYAML := `schema: v1
instructions:
  - source: instructions.md
snippets:
  - source: snippets/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(snippetsDir, "reasoning.md"), []byte("Always explain your reasoning before acting."), 0644))

	instrContent := `# Instructions

<!-- keel:include reasoning -->

Follow team conventions.
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instrContent), 0644))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel, Version: "v1"},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err, "reading CLAUDE.md")
	text := string(content)

	assert.Contains(t, text, "Always explain your reasoning before acting.")
	assert.NotContains(t, text, "keel:include")
	assert.Contains(t, text, "Follow team conventions.")
}

func TestCompileClaude_SnippetCrossLayer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layer1 := filepath.Join(dir, "01-company")
	snippetsDir := filepath.Join(layer1, "snippets")
	require.NoError(t, os.MkdirAll(snippetsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer1, "keel.yml"), []byte("schema: v1\nsnippets:\n  - source: snippets/\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(snippetsDir, "shared-rule.md"), []byte("Always review code before merging."), 0644))

	layer2 := filepath.Join(dir, "02-team")
	require.NoError(t, os.MkdirAll(layer2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "keel.yml"), []byte("schema: v1\ninstructions:\n  - source: instructions.md\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "instructions.md"), []byte("# Team Rules\n\n<!-- keel:include shared-rule -->\n"), 0644))

	layers := []config.Layer{
		{Path: layer1, Type: "company", Format: config.FormatKeel, Version: "v1"},
		{Path: layer2, Type: "team", Format: config.FormatKeel, Version: "v1"},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err, "reading CLAUDE.md")
	text := string(content)

	assert.Contains(t, text, "Always review code before merging.")
	assert.NotContains(t, text, "keel:include")
}

func TestCompileClaude_SnippetInAgentFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	snippetsDir := filepath.Join(layerDir, "snippets")
	agentsDir := filepath.Join(layerDir, "agents")
	for _, d := range []string{snippetsDir, agentsDir} {
		require.NoError(t, os.MkdirAll(d, 0755))
	}

	keelYAML := `schema: v1
agents:
  - source: agents/
snippets:
  - source: snippets/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(snippetsDir, "safety.md"), []byte("Never delete production data."), 0644))
	agentContent := "# Code Reviewer\n\n<!-- keel:include safety -->\n\nReview all changes carefully.\n"
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "code-reviewer.md"), []byte(agentContent), 0644))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel, Version: "v1"},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "agents", "code-reviewer.md"))
	require.NoError(t, err, "reading agent file")
	text := string(content)

	assert.Contains(t, text, "Never delete production data.")
	assert.NotContains(t, text, "keel:include")
	assert.Contains(t, text, "Review all changes carefully.")
}

func TestCompileClaude_MissingSnippetError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	snippetsDir := filepath.Join(layerDir, "snippets")
	require.NoError(t, os.MkdirAll(snippetsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(snippetsDir, "other.md"), []byte("Other content"), 0644))

	keelYAML := `schema: v1
instructions:
  - source: instructions.md
snippets:
  - source: snippets/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYAML), 0644))
	instrContent := "# Instructions\n\n<!-- keel:include nonexistent-snippet -->\n"
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instrContent), 0644))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel, Version: "v1"},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.Error(t, err, "expected error for missing snippet reference")
	assert.Contains(t, err.Error(), "nonexistent-snippet")
}

func TestCompileClaude_SnippetsDirNotInOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	snippetsDir := filepath.Join(layerDir, ".claude", "snippets")
	require.NoError(t, os.MkdirAll(snippetsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(snippetsDir, "secret.md"), []byte("This should not appear"), 0644))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	outputSnippets := filepath.Join(outputDir, ".claude", "snippets")
	_, err = os.Stat(outputSnippets)
	assert.True(t, os.IsNotExist(err), "snippets/ should NOT appear in compiled output")
}

func TestCompileClaude_NativeLayerSnippets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layer1 := filepath.Join(dir, "01-native")
	snippetsDir := filepath.Join(layer1, "snippets")
	require.NoError(t, os.MkdirAll(snippetsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(snippetsDir, "from-native.md"), []byte("Content from native layer snippet."), 0644))

	layer2 := filepath.Join(dir, "02-dsl")
	require.NoError(t, os.MkdirAll(layer2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "keel.yml"), []byte("schema: v1\ninstructions:\n  - source: instructions.md\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "instructions.md"), []byte("<!-- keel:include from-native -->"), 0644))

	layers := []config.Layer{
		{Path: layer1, Type: "base", Format: config.FormatKeel},
		{Path: layer2, Type: "project", Format: config.FormatKeel, Version: "v1"},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(content), "Content from native layer snippet.")
}
