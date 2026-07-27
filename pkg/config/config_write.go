package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// configFileContent is a minimal struct for creating new config.yml files.
// It includes only the fields we need to write, avoiding omitempty issues
// with the full CompileConfig struct.
type configFileContent struct {
	Output map[string]string `yaml:"output,omitempty"`
	Layers []string          `yaml:"layers"`
}

// CreateConfigFile creates a new .keel/config.yml file with the given layers
// and output targets. The file is written with 0o644 permissions.
func CreateConfigFile(path string, layers []string, output map[string]string) error {
	content := configFileContent{
		Output: output,
		Layers: layers,
	}

	data, err := yaml.Marshal(content)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// AddLayerToConfigFile surgically adds a layer to an existing config.yml file
// without destroying comments or formatting. It uses the yaml.v3 Node API to
// preserve the existing structure.
func AddLayerToConfigFile(path string, layerSource string) error {
	// Read the existing file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	// Parse into a yaml.Node tree to preserve structure
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	// The root is a DocumentNode, the actual content is in root.Content[0]
	if len(root.Content) == 0 {
		return fmt.Errorf("empty config file")
	}

	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return fmt.Errorf("config file root must be a mapping")
	}

	// Find the "layers" key in the mapping
	layersIndex := -1
	for i := 0; i < len(doc.Content); i += 2 {
		keyNode := doc.Content[i]
		if keyNode.Value == "layers" {
			layersIndex = i
			break
		}
	}

	if layersIndex >= 0 {
		// Layers key exists — append to the sequence
		valueNode := doc.Content[layersIndex+1]
		if valueNode.Kind != yaml.SequenceNode {
			return fmt.Errorf("layers value must be a sequence")
		}

		// Create a new scalar node for the layer source
		newLayer := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: layerSource,
		}
		valueNode.Content = append(valueNode.Content, newLayer)
	} else {
		// Layers key doesn't exist — create it
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "layers",
		}
		valueNode := &yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{
					Kind:  yaml.ScalarNode,
					Value: layerSource,
				},
			},
		}
		doc.Content = append(doc.Content, keyNode, valueNode)
	}

	// Write back with proper formatting
	outFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("opening config file for writing: %w", err)
	}
	defer outFile.Close()

	encoder := yaml.NewEncoder(outFile)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return fmt.Errorf("encoding config file: %w", err)
	}

	return encoder.Close()
}

// MergeOutputTargets adds output targets to an existing config.yml file,
// skipping any that already exist. It uses the yaml.v3 Node API to preserve
// comments and formatting.
func MergeOutputTargets(path string, targets map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	if len(root.Content) == 0 {
		return fmt.Errorf("empty config file")
	}

	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return fmt.Errorf("config file root must be a mapping")
	}

	// Find the "output" key
	outputIndex := -1
	for i := 0; i < len(doc.Content); i += 2 {
		if doc.Content[i].Value == "output" {
			outputIndex = i
			break
		}
	}

	if outputIndex >= 0 {
		// Output key exists — merge new targets
		valueNode := doc.Content[outputIndex+1]
		if valueNode.Kind != yaml.MappingNode {
			return fmt.Errorf("output value must be a mapping")
		}

		// Collect existing keys
		existing := make(map[string]bool)
		for i := 0; i < len(valueNode.Content); i += 2 {
			existing[valueNode.Content[i].Value] = true
		}

		// Add missing targets
		for agent, dir := range targets {
			if existing[agent] {
				continue
			}
			valueNode.Content = append(valueNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: agent},
				&yaml.Node{Kind: yaml.ScalarNode, Value: dir},
			)
		}
	} else {
		// Output key doesn't exist — create it before layers
		outputMap := &yaml.Node{Kind: yaml.MappingNode}
		for agent, dir := range targets {
			outputMap.Content = append(outputMap.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: agent},
				&yaml.Node{Kind: yaml.ScalarNode, Value: dir},
			)
		}

		// Insert output before the layers key (or at the beginning)
		layersIndex := -1
		for i := 0; i < len(doc.Content); i += 2 {
			if doc.Content[i].Value == "layers" {
				layersIndex = i
				break
			}
		}

		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "output"}
		if layersIndex >= 0 {
			// Insert before layers
			newContent := make([]*yaml.Node, 0, len(doc.Content)+2)
			newContent = append(newContent, doc.Content[:layersIndex]...)
			newContent = append(newContent, keyNode, outputMap)
			newContent = append(newContent, doc.Content[layersIndex:]...)
			doc.Content = newContent
		} else {
			doc.Content = append(doc.Content, keyNode, outputMap)
		}
	}

	outFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("opening config file for writing: %w", err)
	}
	defer outFile.Close()

	encoder := yaml.NewEncoder(outFile)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return fmt.Errorf("encoding config file: %w", err)
	}

	return encoder.Close()
}

// RemoveLayerFromConfigFile surgically removes a layer from an existing
// config.yml file without destroying comments or formatting. It uses the
// yaml.v3 Node API to preserve the existing structure.
// The layer is matched by exact source string.
func RemoveLayerFromConfigFile(path string, layerSource string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	if len(root.Content) == 0 {
		return fmt.Errorf("empty config file")
	}

	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return fmt.Errorf("config file root must be a mapping")
	}

	// Find the "layers" sequence
	for i := 0; i < len(doc.Content); i += 2 {
		if doc.Content[i].Value != "layers" {
			continue
		}

		seq := doc.Content[i+1]
		if seq.Kind != yaml.SequenceNode {
			return fmt.Errorf("layers value must be a sequence")
		}

		// Remove matching entries (handle both bare strings and source-key objects)
		filtered := make([]*yaml.Node, 0, len(seq.Content))
		for _, node := range seq.Content {
			match := false
			if node.Kind == yaml.ScalarNode && node.Value == layerSource {
				match = true
			} else if node.Kind == yaml.MappingNode {
				// Check for {source: "..."} form
				for j := 0; j < len(node.Content); j += 2 {
					if node.Content[j].Value == "source" && node.Content[j+1].Value == layerSource {
						match = true
						break
					}
				}
			}
			if !match {
				filtered = append(filtered, node)
			}
		}
		seq.Content = filtered
		break
	}

	outFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("opening config file for writing: %w", err)
	}
	defer outFile.Close()

	encoder := yaml.NewEncoder(outFile)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return fmt.Errorf("encoding config file: %w", err)
	}

	return encoder.Close()
}

// LayerExistsInConfig checks if a layer source is already present in the
// config's LayerEntries.
func LayerExistsInConfig(cfg *CompileConfig, source string) bool {
	for _, entry := range cfg.LayerEntries {
		if entry.Source == source {
			return true
		}
	}
	return false
}
