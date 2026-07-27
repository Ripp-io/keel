package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// LayerEntry supports both simple string and complex object layer specifications.
// Simple: "path/to/layer" → LayerEntry{Source: "path/to/layer"}
// Complex: {source: "path", filter: {skills: {include: ["x"]}}}
type LayerEntry struct {
	Source string       `yaml:"source"`
	Filter *LayerFilter `yaml:"filter,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling to support both bare strings
// and full object notation for layer entries.
func (e *LayerEntry) UnmarshalYAML(node *yaml.Node) error {
	// Attempt to unmarshal as a bare string first.
	if node.Kind == yaml.ScalarNode {
		e.Source = node.Value
		return nil
	}

	// Otherwise, unmarshal as the full struct.
	type rawEntry LayerEntry
	var raw rawEntry
	if err := node.Decode(&raw); err != nil {
		return err
	}
	*e = LayerEntry(raw)
	return nil
}

// LayerFilter specifies per-section filtering for a layer.
// Sections not mentioned pass through unfiltered (denylist default).
type LayerFilter struct {
	// Sections maps section names to their filter specs.
	// Section values can be:
	//   - false (disable the section entirely)
	//   - {include: [...], exclude: [...]} (pattern-based filtering)
	Sections map[string]*SectionFilter
}

// UnmarshalYAML implements custom YAML unmarshaling for LayerFilter.
// Handles polymorphic section values: false vs {include/exclude}.
func (f *LayerFilter) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("filter must be a mapping, got %v", node.Kind)
	}

	f.Sections = make(map[string]*SectionFilter)

	// Iterate through the mapping node's content (key-value pairs).
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		sectionName := keyNode.Value

		// Check if the value is a boolean false.
		if valNode.Kind == yaml.ScalarNode && valNode.Tag == "!!bool" && valNode.Value == "false" {
			f.Sections[sectionName] = &SectionFilter{Disabled: true}
			continue
		}

		// Otherwise, unmarshal as a SectionFilter object.
		var sf SectionFilter
		if err := valNode.Decode(&sf); err != nil {
			return fmt.Errorf("invalid filter for section %q: %w", sectionName, err)
		}
		f.Sections[sectionName] = &sf
	}

	return nil
}

// SectionFilter defines include/exclude patterns for a section.
type SectionFilter struct {
	Include  []string `yaml:"include,omitempty"`
	Exclude  []string `yaml:"exclude,omitempty"`
	Disabled bool     // true when YAML value is `false`
}
