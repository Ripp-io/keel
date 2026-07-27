package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLayerEntry_UnmarshalYAML_BareString(t *testing.T) {
	yml := `./path/to/layer`
	var entry LayerEntry
	err := yaml.Unmarshal([]byte(yml), &entry)
	require.NoError(t, err)
	assert.Equal(t, "./path/to/layer", entry.Source)
	assert.Nil(t, entry.Filter)
}

func TestLayerEntry_UnmarshalYAML_ObjectWithSourceOnly(t *testing.T) {
	yml := `
source: /keel/layers/base
`
	var entry LayerEntry
	err := yaml.Unmarshal([]byte(yml), &entry)
	require.NoError(t, err)
	assert.Equal(t, "/keel/layers/base", entry.Source)
	assert.Nil(t, entry.Filter)
}

func TestLayerEntry_UnmarshalYAML_ObjectWithFilter(t *testing.T) {
	yml := `
source: /keel/layers/base
filter:
  skills:
    include: ["security-*"]
`
	var entry LayerEntry
	err := yaml.Unmarshal([]byte(yml), &entry)
	require.NoError(t, err)
	assert.Equal(t, "/keel/layers/base", entry.Source)
	require.NotNil(t, entry.Filter)
	require.NotNil(t, entry.Filter.Sections["skills"])
	assert.Equal(t, []string{"security-*"}, entry.Filter.Sections["skills"].Include)
}

func TestLayerEntry_UnmarshalYAML_ListMixed(t *testing.T) {
	yml := `
- ./local/layer
- source: /remote/layer
  filter:
    commands:
      exclude: ["draft-*"]
- /another/local
`
	var entries []LayerEntry
	err := yaml.Unmarshal([]byte(yml), &entries)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	assert.Equal(t, "./local/layer", entries[0].Source)
	assert.Nil(t, entries[0].Filter)

	assert.Equal(t, "/remote/layer", entries[1].Source)
	require.NotNil(t, entries[1].Filter)
	require.NotNil(t, entries[1].Filter.Sections["commands"])
	assert.Equal(t, []string{"draft-*"}, entries[1].Filter.Sections["commands"].Exclude)

	assert.Equal(t, "/another/local", entries[2].Source)
	assert.Nil(t, entries[2].Filter)
}

func TestLayerFilter_UnmarshalYAML_SectionDisabled(t *testing.T) {
	yml := `
skills: false
commands:
  include: ["deploy*"]
`
	var filter LayerFilter
	err := yaml.Unmarshal([]byte(yml), &filter)
	require.NoError(t, err)

	require.NotNil(t, filter.Sections["skills"])
	assert.True(t, filter.Sections["skills"].Disabled)

	require.NotNil(t, filter.Sections["commands"])
	assert.False(t, filter.Sections["commands"].Disabled)
	assert.Equal(t, []string{"deploy*"}, filter.Sections["commands"].Include)
}

func TestLayerFilter_UnmarshalYAML_IncludeAndExclude(t *testing.T) {
	yml := `
skills:
  include: ["security-*", "api-*"]
  exclude: ["*-draft"]
`
	var filter LayerFilter
	err := yaml.Unmarshal([]byte(yml), &filter)
	require.NoError(t, err)

	require.NotNil(t, filter.Sections["skills"])
	sf := filter.Sections["skills"]
	assert.Equal(t, []string{"security-*", "api-*"}, sf.Include)
	assert.Equal(t, []string{"*-draft"}, sf.Exclude)
	assert.False(t, sf.Disabled)
}

func TestLayerFilter_UnmarshalYAML_MultipleSections(t *testing.T) {
	yml := `
skills:
  include: ["security-*"]
commands: false
snippets:
  exclude: ["deprecated-*"]
`
	var filter LayerFilter
	err := yaml.Unmarshal([]byte(yml), &filter)
	require.NoError(t, err)

	require.Len(t, filter.Sections, 3)

	require.NotNil(t, filter.Sections["skills"])
	assert.Equal(t, []string{"security-*"}, filter.Sections["skills"].Include)

	require.NotNil(t, filter.Sections["commands"])
	assert.True(t, filter.Sections["commands"].Disabled)

	require.NotNil(t, filter.Sections["snippets"])
	assert.Equal(t, []string{"deprecated-*"}, filter.Sections["snippets"].Exclude)
}

func TestCompileConfig_NormalizeLayers_CLIPrecedence(t *testing.T) {
	cfg := CompileConfig{
		Layers: []string{"/cli/layer1", "/cli/layer2"},
		LayerEntries: []LayerEntry{
			{Source: "/config/layer1"},
			{Source: "/config/layer2"},
		},
	}

	normalized := cfg.NormalizeLayers()
	require.Len(t, normalized, 2)
	assert.Equal(t, "/cli/layer1", normalized[0].Source)
	assert.Equal(t, "/cli/layer2", normalized[1].Source)
	assert.Nil(t, normalized[0].Filter)
	assert.Nil(t, normalized[1].Filter)
}

func TestCompileConfig_NormalizeLayers_ConfigFile(t *testing.T) {
	cfg := CompileConfig{
		LayerEntries: []LayerEntry{
			{Source: "/config/layer1"},
			{Source: "/config/layer2", Filter: &LayerFilter{
				Sections: map[string]*SectionFilter{
					"skills": {Include: []string{"api-*"}},
				},
			}},
		},
	}

	normalized := cfg.NormalizeLayers()
	require.Len(t, normalized, 2)
	assert.Equal(t, "/config/layer1", normalized[0].Source)
	assert.Nil(t, normalized[0].Filter)
	assert.Equal(t, "/config/layer2", normalized[1].Source)
	require.NotNil(t, normalized[1].Filter)
	assert.NotNil(t, normalized[1].Filter.Sections["skills"])
}

func TestCompileConfig_NormalizeLayers_EmptyCLI(t *testing.T) {
	cfg := CompileConfig{
		Layers: nil,
		LayerEntries: []LayerEntry{
			{Source: "/config/layer"},
		},
	}

	normalized := cfg.NormalizeLayers()
	require.Len(t, normalized, 1)
	assert.Equal(t, "/config/layer", normalized[0].Source)
}

func TestCompileConfig_NormalizeLayers_EmptyBoth(t *testing.T) {
	cfg := CompileConfig{
		Layers:       nil,
		LayerEntries: nil,
	}

	normalized := cfg.NormalizeLayers()
	assert.Empty(t, normalized)
}
