package skills

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec represents a fully validated Agent Skills specification.
type Spec struct {
	Name          string            // required
	Description   string            // required
	License       string            // optional
	Compatibility string            // optional
	AllowedTools  string            // optional, space-separated tool patterns
	Metadata      map[string]string // optional; string values only (validation accepts any mapping)
	Body          string            // markdown body after frontmatter
}

// GenerateSKILLMD produces Agent Skills compliant SKILL.md content from a Spec.
// Returns an error if the spec fails validation.
func GenerateSKILLMD(spec Spec) ([]byte, error) {
	// Validate first — use empty dirName since we validate the name field
	// against itself (no directory check needed for generation).
	warnings := ValidateSpec(spec, spec.Name)
	if len(warnings) > 0 {
		msgs := make([]string, len(warnings))
		for i, w := range warnings {
			msgs[i] = fmt.Sprintf("%s: %s", w.Field, w.Message)
		}
		return nil, fmt.Errorf("spec validation failed: %s", strings.Join(msgs, "; "))
	}

	// Build ordered frontmatter map. We use yaml.Node to control field order
	// since yaml.Marshal on map[string]any uses sorted keys which doesn't
	// guarantee our desired order (name, description, then alphabetical optionals).
	fm := buildOrderedFrontmatter(spec)

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("marshaling frontmatter: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(bytes.TrimRight(fmBytes, "\n"))
	buf.WriteString("\n---\n")
	if spec.Body != "" {
		buf.WriteString(spec.Body)
		// Ensure trailing newline
		if spec.Body[len(spec.Body)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}

	return buf.Bytes(), nil
}

// ValidateSpec checks a Spec against Agent Skills rules (same rules as ValidateDir
// but operating on the structured type instead of filesystem).
func ValidateSpec(spec Spec, dirName string) []Warning {
	fm := specToFrontmatterMap(spec)
	return validateFrontmatter(fm, dirName)
}

// specToFrontmatterMap converts a Spec into a map[string]any for validation.
func specToFrontmatterMap(spec Spec) map[string]any {
	fm := map[string]any{
		"name":        spec.Name,
		"description": spec.Description,
	}
	if spec.License != "" {
		fm["license"] = spec.License
	}
	if spec.Compatibility != "" {
		fm["compatibility"] = spec.Compatibility
	}
	if spec.AllowedTools != "" {
		fm["allowed-tools"] = spec.AllowedTools
	}
	if len(spec.Metadata) > 0 {
		meta := make(map[string]any, len(spec.Metadata))
		for k, v := range spec.Metadata {
			meta[k] = v
		}
		fm["metadata"] = meta
	}
	return fm
}

// buildOrderedFrontmatter creates a yaml.Node tree with controlled field order:
// name, description, then optional fields in alphabetical order.
func buildOrderedFrontmatter(spec Spec) *yaml.Node {
	mapping := &yaml.Node{
		Kind: yaml.MappingNode,
	}

	// Required fields first (fixed order)
	addScalarPair(mapping, "name", spec.Name)
	addScalarPair(mapping, "description", spec.Description)

	// Optional fields in alphabetical order
	type optionalField struct {
		key string
		val string
	}

	var optionals []optionalField
	if spec.AllowedTools != "" {
		optionals = append(optionals, optionalField{"allowed-tools", spec.AllowedTools})
	}
	if spec.Compatibility != "" {
		optionals = append(optionals, optionalField{"compatibility", spec.Compatibility})
	}
	if spec.License != "" {
		optionals = append(optionals, optionalField{"license", spec.License})
	}

	sort.Slice(optionals, func(i, j int) bool {
		return optionals[i].key < optionals[j].key
	})

	for _, opt := range optionals {
		addScalarPair(mapping, opt.key, opt.val)
	}

	// Metadata (mapping type)
	if len(spec.Metadata) > 0 {
		metaKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "metadata"}
		metaVal := &yaml.Node{Kind: yaml.MappingNode}

		keys := make([]string, 0, len(spec.Metadata))
		for k := range spec.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			addScalarPair(metaVal, k, spec.Metadata[k])
		}

		mapping.Content = append(mapping.Content, metaKey, metaVal)
	}

	doc := &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{mapping},
	}

	return doc
}

// addScalarPair appends a key-value scalar pair to a mapping node.
func addScalarPair(mapping *yaml.Node, key, value string) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}
