package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestCollectShadowed_NoOverlays(t *testing.T) { assert.Empty(t, collectShadowed(nil)) }
func TestCollectShadowed_EmptyOverlays(t *testing.T) {
	assert.Empty(t, collectShadowed(map[string][]string{}))
}
func TestCollectShadowed_SingleWriter(t *testing.T) {
	assert.Empty(t, collectShadowed(map[string][]string{"commands/deploy.md": {"01-company"}}))
}

func TestCollectShadowed_TwoWriters(t *testing.T) {
	result := collectShadowed(map[string][]string{"commands/deploy.md": {"01-company", "03-team"}})
	require.Len(t, result, 1)
	assert.Equal(t, "commands/deploy.md", result[0].path)
	assert.Equal(t, "01-company → 03-team", result[0].chain)
}

func TestCollectShadowed_ThreeWriters(t *testing.T) {
	result := collectShadowed(map[string][]string{"skills/review.md": {"01-base", "02-org", "03-team"}})
	require.Len(t, result, 1)
	assert.Equal(t, "01-base → 02-org → 03-team", result[0].chain)
}

func TestCollectShadowed_SortedByPath(t *testing.T) {
	result := collectShadowed(map[string][]string{"skills/z.md": {"a", "b"}, "commands/a.md": {"c", "d"}, "agents/mid.md": {"e", "f"}})
	require.Len(t, result, 3)
	assert.Equal(t, "agents/mid.md", result[0].path)
	assert.Equal(t, "commands/a.md", result[1].path)
	assert.Equal(t, "skills/z.md", result[2].path)
}

func TestCollectShadowed_WriterNamesPreserved(t *testing.T) {
	result := collectShadowed(map[string][]string{"commands/deploy.md": {"layer-alpha", "layer-beta"}})
	require.Len(t, result, 1)
	assert.Equal(t, "layer-alpha → layer-beta", result[0].chain)
}

func TestPrintCompileReport_IncludesShadowedSection(t *testing.T) {
	layers := []config.Layer{{Path: "/layers/01-company", Format: "keel"}, {Path: "/layers/03-team", Format: "keel"}}
	result := &compiler.CompileResult{Output: compiler.CompileOutput{Agent: "claude", Overlays: map[string][]string{".claude/commands/deploy.md": {"01-company", "03-team"}}}}
	var buf bytes.Buffer
	printCompileReport(&buf, layers, result, false)
	output := buf.String()
	assert.Contains(t, output, "Shadowed files:")
	assert.Contains(t, output, ".claude/commands/deploy.md")
	assert.Contains(t, output, "01-company")
	assert.Contains(t, output, "03-team")
}

func TestPrintCompileReport_NoShadowedSectionWhenNone(t *testing.T) {
	var buf bytes.Buffer
	printCompileReport(&buf, []config.Layer{{Path: "/layers/01-company", Format: "keel"}}, &compiler.CompileResult{Output: compiler.CompileOutput{Agent: "claude", Overlays: map[string][]string{".claude/commands/deploy.md": {"01-company"}}}}, false)
	assert.NotContains(t, buf.String(), "Shadowed files:")
}

func TestPrintCompileReport_SummaryIncludesShadowCount(t *testing.T) {
	var buf bytes.Buffer
	printCompileReport(&buf, []config.Layer{{Path: "/layers/01-company", Format: "keel"}, {Path: "/layers/03-team", Format: "keel"}}, &compiler.CompileResult{Output: compiler.CompileOutput{Agent: "claude", Overlays: map[string][]string{".claude/commands/deploy.md": {"01-company", "03-team"}}}}, false)
	assert.Contains(t, buf.String(), "1 shadowed")
}

func TestPrintCompileReport_SummaryNoShadowCountWhenZero(t *testing.T) {
	var buf bytes.Buffer
	printCompileReport(&buf, []config.Layer{{Path: "/layers/01-company", Format: "keel"}}, &compiler.CompileResult{Output: compiler.CompileOutput{Agent: "claude"}}, false)
	assert.NotContains(t, buf.String(), "shadowed")
}
