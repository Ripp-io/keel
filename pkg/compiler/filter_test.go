package compiler

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestApplyFilter_NilFilter(t *testing.T) {
	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "test.md", Content: "content"},
		},
		Categories: map[string][]CategoryFile{
			"skills": {
				{RelPath: "test.md", Content: []byte("skill")},
			},
		},
	}

	applyFilter(ir, nil)

	// Should be unchanged.
	assert.Len(t, ir.Instructions, 1)
	assert.Len(t, ir.Categories["skills"], 1)
}

func TestApplyFilter_SectionDisabled(t *testing.T) {
	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"skills": {
				{RelPath: "security.md", Content: []byte("skill1")},
				{RelPath: "api.md", Content: []byte("skill2")},
			},
			"commands": {
				{RelPath: "deploy.md", Content: []byte("cmd")},
			},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"skills": {Disabled: true},
		},
	}

	applyFilter(ir, filter)

	// Skills category should be removed.
	assert.NotContains(t, ir.Categories, "skills")
	// Commands should remain (not mentioned in filter).
	assert.Contains(t, ir.Categories, "commands")
	assert.Len(t, ir.Categories["commands"], 1)
}

func TestApplyFilter_IncludeOnly(t *testing.T) {
	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"skills": {
				{RelPath: "security-audit.md"},
				{RelPath: "security-scan.md"},
				{RelPath: "api-client.md"},
				{RelPath: "deploy.md"},
			},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"skills": {Include: []string{"security-*"}},
		},
	}

	applyFilter(ir, filter)

	require.Contains(t, ir.Categories, "skills")
	files := ir.Categories["skills"]
	require.Len(t, files, 2)
	assert.Equal(t, "security-audit.md", files[0].RelPath)
	assert.Equal(t, "security-scan.md", files[1].RelPath)
}

func TestApplyFilter_ExcludeOnly(t *testing.T) {
	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md"},
				{RelPath: "test.md"},
				{RelPath: "draft-feature.md"},
			},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"commands": {Exclude: []string{"draft-*"}},
		},
	}

	applyFilter(ir, filter)

	require.Contains(t, ir.Categories, "commands")
	files := ir.Categories["commands"]
	require.Len(t, files, 2)
	assert.Equal(t, "deploy.md", files[0].RelPath)
	assert.Equal(t, "test.md", files[1].RelPath)
}

func TestApplyFilter_IncludeAndExclude(t *testing.T) {
	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"skills": {
				{RelPath: "security-audit.md"},
				{RelPath: "security-scan.md"},
				{RelPath: "security-draft.md"},
				{RelPath: "api-client.md"},
			},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"skills": {
				Include: []string{"security-*"},
				Exclude: []string{"*-draft.md"},
			},
		},
	}

	applyFilter(ir, filter)

	require.Contains(t, ir.Categories, "skills")
	files := ir.Categories["skills"]
	require.Len(t, files, 2)
	assert.Equal(t, "security-audit.md", files[0].RelPath)
	assert.Equal(t, "security-scan.md", files[1].RelPath)
}

func TestApplyFilter_InstructionBlocks(t *testing.T) {
	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "intro.md"},
			{Filename: "advanced.md"},
			{Filename: "deprecated.md"},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"instructions": {Exclude: []string{"deprecated*"}},
		},
	}

	applyFilter(ir, filter)

	require.Len(t, ir.Instructions, 2)
	assert.Equal(t, "intro.md", ir.Instructions[0].Filename)
	assert.Equal(t, "advanced.md", ir.Instructions[1].Filename)
}

func TestApplyFilter_Snippets(t *testing.T) {
	ir := &LayerIR{
		Snippets: map[string][]byte{
			"security-checklist": []byte("content1"),
			"security-template":  []byte("content2"),
			"api-docs":           []byte("content3"),
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"snippets": {Include: []string{"security-*"}},
		},
	}

	applyFilter(ir, filter)

	require.Len(t, ir.Snippets, 2)
	assert.Contains(t, ir.Snippets, "security-checklist")
	assert.Contains(t, ir.Snippets, "security-template")
	assert.NotContains(t, ir.Snippets, "api-docs")
}

func TestApplyFilter_SnippetsDisabled(t *testing.T) {
	ir := &LayerIR{
		Snippets: map[string][]byte{
			"snippet1": []byte("content1"),
			"snippet2": []byte("content2"),
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"snippets": {Disabled: true},
		},
	}

	applyFilter(ir, filter)

	assert.Empty(t, ir.Snippets)
}

func TestApplyFilter_UnmentionedSectionsPassThrough(t *testing.T) {
	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"skills": {
				{RelPath: "security.md"},
				{RelPath: "api.md"},
			},
			"commands": {
				{RelPath: "deploy.md"},
			},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"skills": {Include: []string{"security*"}},
		},
	}

	applyFilter(ir, filter)

	// Skills filtered.
	require.Contains(t, ir.Categories, "skills")
	assert.Len(t, ir.Categories["skills"], 1)

	// Commands unfiltered (not mentioned in filter).
	require.Contains(t, ir.Categories, "commands")
	assert.Len(t, ir.Categories["commands"], 1)
}

func TestApplyFilter_UnknownSection(t *testing.T) {
	// Capture log output.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ir := &LayerIR{
		LayerPath: "/test/layer",
		Categories: map[string][]CategoryFile{
			"skills": {{RelPath: "test.md"}},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"hooks":    {Disabled: true},      // Not filterable in v1.
			"settings": {Include: []string{}}, // Not filterable in v1.
		},
	}

	applyFilter(ir, filter)

	// Should log warnings about unsupported sections.
	output := buf.String()
	assert.Contains(t, output, "hooks")
	assert.Contains(t, output, "settings")
	assert.Contains(t, output, "unsupported")
}

func TestApplyFilter_FilterEmptiesCategory(t *testing.T) {
	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"skills": {
				{RelPath: "api.md"},
			},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"skills": {Include: []string{"security-*"}},
		},
	}

	applyFilter(ir, filter)

	// Category should be removed when filter leaves it empty.
	assert.NotContains(t, ir.Categories, "skills")
}

func TestApplyFilter_MultipleCategories(t *testing.T) {
	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"skills": {
				{RelPath: "security.md"},
				{RelPath: "api.md"},
			},
			"commands": {
				{RelPath: "deploy.md"},
				{RelPath: "test.md"},
			},
			"agents": {
				{RelPath: "reviewer.md"},
			},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"skills":   {Include: []string{"security*"}},
			"commands": {Exclude: []string{"test*"}},
		},
	}

	applyFilter(ir, filter)

	// Skills filtered to security*.
	require.Contains(t, ir.Categories, "skills")
	assert.Len(t, ir.Categories["skills"], 1)
	assert.Equal(t, "security.md", ir.Categories["skills"][0].RelPath)

	// Commands filtered to exclude test*.
	require.Contains(t, ir.Categories, "commands")
	assert.Len(t, ir.Categories["commands"], 1)
	assert.Equal(t, "deploy.md", ir.Categories["commands"][0].RelPath)

	// Agents unfiltered.
	require.Contains(t, ir.Categories, "agents")
	assert.Len(t, ir.Categories["agents"], 1)
}

func TestApplyFilter_LoopAndWorktreeInclude(t *testing.T) {
	ir := &LayerIR{
		ContentBlocks: map[string][]InstructionBlock{
			"loop": {
				{Filename: "loop-v1.md"},
				{Filename: "loop-v2.md"},
			},
			"worktree-include": {
				{Filename: "include-src.md"},
				{Filename: "include-tests.md"},
			},
		},
	}

	filter := &config.LayerFilter{
		Sections: map[string]*config.SectionFilter{
			"loop":             {Include: []string{"loop-v2*"}},
			"worktree-include": {Exclude: []string{"*-tests*"}},
		},
	}

	applyFilter(ir, filter)

	require.Len(t, ir.ContentBlocks["loop"], 1)
	assert.Equal(t, "loop-v2.md", ir.ContentBlocks["loop"][0].Filename)

	require.Len(t, ir.ContentBlocks["worktree-include"], 1)
	assert.Equal(t, "include-src.md", ir.ContentBlocks["worktree-include"][0].Filename)
}

func TestMatchFilter_GlobPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		{"exact match", "security.md", "security.md", true},
		{"prefix wildcard", "security-*", "security-audit.md", true},
		{"suffix wildcard", "*-draft", "feature-draft", true},
		{"middle wildcard", "api-*-client", "api-rest-client", true},
		{"no match", "security-*", "api.md", false},
		{"question mark", "test?.md", "test1.md", true},
		{"question mark no match", "test?.md", "test10.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &config.SectionFilter{Include: []string{tt.pattern}}
			got := matchFilter(tt.input, sf)
			assert.Equal(t, tt.want, got)
		})
	}
}
