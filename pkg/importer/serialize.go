// Package importer provides the reverse-direction operation of serializing
// a LayerIR back to a keel.yml layer directory. This is used by the
// `keel import` command to convert native agent configs into Keel DSL.
package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

// importFileRef is a clean FileRef for serialization that omits zero-value fields.
type importFileRef struct {
	Source string `yaml:"source,omitempty"`
}

// importSettings holds the settings section for serialized keel.yml output.
type importSettings struct {
	Model  string `yaml:"model,omitempty"`
	Claude string `yaml:"claude,omitempty"`
	Cursor string `yaml:"cursor,omitempty"`
}

// importMCPConfig is the MCP section for serialized keel.yml output.
type importMCPConfig struct {
	Servers []importFileRef `yaml:"servers,omitempty"`
}

// importPermissionsConfig is the permissions section for serialized keel.yml output.
type importPermissionsConfig struct {
	Rules []importFileRef `yaml:"rules,omitempty"`
}

// serializableKeelConfig is the YAML structure written to keel.yml during import.
type serializableKeelConfig struct {
	Schema       string                   `yaml:"schema"`
	Name         string                   `yaml:"name,omitempty"`
	Description  string                   `yaml:"description,omitempty"`
	Instructions []importFileRef          `yaml:"instructions,omitempty"`
	Permissions  *importPermissionsConfig `yaml:"permissions,omitempty"`
	MCP          *importMCPConfig         `yaml:"mcp,omitempty"`
	Hooks        []importFileRef          `yaml:"hooks,omitempty"`
	Commands     []importFileRef          `yaml:"commands,omitempty"`
	Agents       []importFileRef          `yaml:"agents,omitempty"`
	Skills       []importFileRef          `yaml:"skills,omitempty"`
	Plugins      []importFileRef          `yaml:"plugins,omitempty"`
	AgentMemory  []importFileRef          `yaml:"agent-memory,omitempty"`
	Environment  []importFileRef          `yaml:"environment,omitempty"`
	Rules        []importFileRef          `yaml:"rules,omitempty"`
	Settings     *importSettings          `yaml:"settings,omitempty"`
}

// SerializeIR writes a LayerIR as a keel.yml layer directory. It creates
// the output directory and all referenced files. Only non-empty sections
// are emitted -- missing IR fields produce no corresponding file or keel.yml entry.
func SerializeIR(ir *compiler.LayerIR, outputDir string) error {
	if ir == nil {
		return fmt.Errorf("cannot serialize nil IR")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	cfg := serializableKeelConfig{
		Schema: "v1",
	}

	// Name
	cfg.Name = ir.Name
	if cfg.Name == "" {
		cfg.Name = "imported-config"
	}

	// Description
	cfg.Description = ir.Description
	if cfg.Description == "" && ir.SourceAgent != "" {
		cfg.Description = "Imported from " + ir.SourceAgent + " native configuration"
	}

	// Instructions
	if len(ir.Instructions) > 0 {
		var blocks []string
		for _, b := range ir.Instructions {
			blocks = append(blocks, b.Content)
		}
		content := strings.Join(blocks, "\n\n")
		if err := os.WriteFile(filepath.Join(outputDir, "instructions.md"), []byte(content), 0644); err != nil {
			return fmt.Errorf("writing instructions.md: %w", err)
		}
		cfg.Instructions = []importFileRef{{Source: "instructions.md"}}
	}

	// Permissions
	if len(ir.Permissions) > 0 {
		if err := compiler.WriteJSONFile(filepath.Join(outputDir, "permissions.json"), ir.Permissions); err != nil {
			return fmt.Errorf("writing permissions.json: %w", err)
		}
		cfg.Permissions = &importPermissionsConfig{Rules: []importFileRef{{Source: "permissions.json"}}}
	}

	// MCP
	if len(ir.MCP) > 0 {
		if err := compiler.WriteJSONFile(filepath.Join(outputDir, "mcp.json"), ir.MCP); err != nil {
			return fmt.Errorf("writing mcp.json: %w", err)
		}
		cfg.MCP = &importMCPConfig{Servers: []importFileRef{{Source: "mcp.json"}}}
	}

	// Hooks
	if len(ir.Hooks) > 0 {
		if err := compiler.WriteJSONFile(filepath.Join(outputDir, "hooks.json"), ir.Hooks); err != nil {
			return fmt.Errorf("writing hooks.json: %w", err)
		}
		cfg.Hooks = []importFileRef{{Source: "hooks.json"}}
	}

	// ConfigBlocks (environment, etc.)
	for name, config := range ir.ConfigBlocks {
		if len(config) == 0 {
			continue
		}
		filename := name + ".json"
		if err := compiler.WriteJSONFile(filepath.Join(outputDir, filename), config); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
		switch name {
		case "environment":
			cfg.Environment = []importFileRef{{Source: filename}}
		}
	}

	// Agent-specific settings files
	for agent, data := range ir.AgentSpecific {
		if len(data) == 0 {
			continue
		}
		filename := "settings." + agent + ".json"
		if err := compiler.WriteJSONFile(filepath.Join(outputDir, filename), data); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	// Category directories
	categoryFields := map[string]*[]importFileRef{
		"commands":     &cfg.Commands,
		"agents":       &cfg.Agents,
		"skills":       &cfg.Skills,
		"plugins":      &cfg.Plugins,
		"agent-memory": &cfg.AgentMemory,
		"rules":        &cfg.Rules,
	}
	for catName, refs := range categoryFields {
		files, ok := ir.Categories[catName]
		if !ok || len(files) == 0 {
			continue
		}
		if _, err := compiler.WriteCategory(outputDir, catName, files); err != nil {
			return fmt.Errorf("writing category %s: %w", catName, err)
		}
		*refs = []importFileRef{{Source: catName + "/"}}
	}

	// Settings: build when there are portable settings or agent-specific files
	hasPortableSettings := len(ir.Settings) > 0
	hasClaudeSpecific := len(ir.AgentSpecific["claude"]) > 0
	hasCursorSpecific := len(ir.AgentSpecific["cursor"]) > 0
	if hasPortableSettings || hasClaudeSpecific || hasCursorSpecific {
		s := &importSettings{}
		if model, ok := ir.Settings["model"]; ok {
			if ms, ok := model.(string); ok {
				s.Model = ms
			}
		}
		if hasClaudeSpecific {
			s.Claude = "settings.claude.json"
		}
		if hasCursorSpecific {
			s.Cursor = "settings.cursor.json"
		}
		cfg.Settings = s
	}

	// Marshal and write keel.yml
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshalling keel.yml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "keel.yml"), out, 0644); err != nil {
		return fmt.Errorf("writing keel.yml: %w", err)
	}

	return nil
}
