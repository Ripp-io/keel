package vibe

import (
	"log/slog"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
	compiler.RegisterReader("vibe", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Mistral Vibe native configuration into a LayerIR.
//
// NOTE: Vibe uses config.toml natively, but this reader expects config.json
// because go.mod doesn't include a TOML library yet. Users can manually convert
// config.toml to config.json for import, or a TOML library can be added in a
// future enhancement (e.g., github.com/pelletier/go-toml/v2).
type Reader struct{}

func (r *Reader) AgentName() string { return "vibe" }

// Read reads a Vibe project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "vibe",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills"}
	if err := compiler.ReadCategories(layerPath, ".vibe", categories, ir); err != nil {
		return nil, err
	}

	compiler.ReadSnippets(layerPath, ".vibe", ir)
	compiler.ReadBinDir(layerPath, ir)

	r.readOtherDotDirFiles(layerPath, ir)

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .vibe/config.json
// and layer-root config.json (and the convention name settings.vibe.json).
// The first found file wins. It decomposes the contents into MCP, portable
// settings, and Vibe-specific settings.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".vibe", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read vibe config", "keys", len(raw), "layer", layerPath)

	// Extract MCP and translate from Vibe format to IR format
	if mcpServersRaw, ok := raw["mcp_servers"]; ok {
		if mcpServers, ok := mcpServersRaw.([]any); ok {
			if translated := TranslateMCPFromVibe(mcpServers); len(translated) > 0 {
				ir.MCP = translated
			}
		}
		delete(raw, "mcp_servers")
	}

	// Extract portable model setting
	if modelVal, ok := raw["active_model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = modelVal
		delete(raw, "active_model")
	}

	// Extract providers and models arrays (Vibe-specific)
	if providersRaw, ok := raw["providers"]; ok {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["vibe"] == nil {
			ir.AgentSpecific["vibe"] = make(map[string]any)
		}
		ir.AgentSpecific["vibe"]["providers"] = providersRaw
		delete(raw, "providers")
	}

	if modelsRaw, ok := raw["models"]; ok {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["vibe"] == nil {
			ir.AgentSpecific["vibe"] = make(map[string]any)
		}
		ir.AgentSpecific["vibe"]["models"] = modelsRaw
		delete(raw, "models")
	}

	// Remaining keys are Vibe-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["vibe"] == nil {
			ir.AgentSpecific["vibe"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["vibe"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "vibe")

	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .vibe/ directory
// that are not handled by specific readers (settings, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"config.toml":         true,
		"config.json":         true,
		"settings.vibe.json":  true,
		".keel.json":          true,
		".env":                true, // Vibe's API key file
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets", "prompts", "tools",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".vibe", excludeFiles, knownCategories, ir)
}

// TranslateMCPFromVibe converts Vibe's mcp_servers array format to IR format.
// Input:  [{"name": "name", "transport": "stdio", "command": "cmd", "args": ["a"], "env": {"K":"V"}}]
// Output: {"mcpServers": {"name": {"command": "cmd", "args": ["a"], "env": {"K":"V"}}}}
func TranslateMCPFromVibe(mcpServers []any) map[string]any {
	if mcpServers == nil || len(mcpServers) == 0 {
		return nil
	}

	translated := make(map[string]any)

	for _, serverRaw := range mcpServers {
		server, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}

		// Get server name
		name, ok := server["name"].(string)
		if !ok || name == "" {
			continue
		}

		irServer := make(map[string]any)

		// Handle transport type
		transport, _ := server["transport"].(string)
		if transport == "http" || transport == "streamable-http" {
			// Remote server
			if url, ok := server["url"].(string); ok {
				irServer["url"] = url
			}

			// Copy auth config
			if auth, ok := server["auth"]; ok {
				irServer["auth"] = auth
			}
		} else {
			// Local server (stdio or other)
			if cmd, ok := server["command"].(string); ok {
				irServer["command"] = cmd
			}

			if argsRaw, ok := server["args"]; ok {
				if args, ok := argsRaw.([]any); ok {
					irServer["args"] = args
				}
			}

			if envRaw, ok := server["env"]; ok {
				if env, ok := envRaw.(map[string]any); ok {
					irServer["env"] = env
				}
			}
		}

		// Copy other fields except name and transport
		for k, v := range server {
			if k != "name" && k != "transport" && k != "command" && k != "args" && k != "env" && k != "url" && k != "auth" {
				irServer[k] = v
			}
		}

		translated[name] = irServer
	}

	if len(translated) == 0 {
		return nil
	}

	return map[string]any{"mcpServers": translated}
}
