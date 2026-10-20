package qwen

import (
	"log/slog"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
	compiler.RegisterReader("qwen", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Qwen Code native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "qwen" }

// Read reads a Qwen Code project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "qwen",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills"}
	if err := compiler.ReadCategories(layerPath, ".qwen", categories, ir); err != nil {
		return nil, err
	}

	compiler.ReadSnippets(layerPath, ".qwen", ir)
	compiler.ReadBinDir(layerPath, ir)

	r.readOtherDotDirFiles(layerPath, ir)

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .qwen/settings.json
// and layer-root settings.json (and the convention name settings.qwen.json).
// The first found file wins. It decomposes the contents into MCP, portable
// settings, and Qwen-specific settings.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".qwen", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read qwen settings.json", "keys", len(raw), "layer", layerPath)

	// Extract MCP servers (standard mcpServers format, same as Claude)
	if mcpServersRaw, ok := raw["mcpServers"]; ok {
		if mcpServers, ok := mcpServersRaw.(map[string]any); ok && len(mcpServers) > 0 {
			ir.MCP = map[string]any{"mcpServers": mcpServers}
		}
		delete(raw, "mcpServers")
	}

	// Extract portable settings (model, modelProviders)
	portableKeys := []string{"model", "modelProviders"}
	for _, key := range portableKeys {
		if val, ok := raw[key]; ok {
			if ir.Settings == nil {
				ir.Settings = make(map[string]any)
			}
			ir.Settings[key] = val
			delete(raw, key)
		}
	}

	// Remaining keys are Qwen-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["qwen"] == nil {
			ir.AgentSpecific["qwen"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["qwen"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "qwen")

	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .qwen/ directory
// that are not handled by specific readers (settings, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"settings.json":      true,
		"settings.qwen.json": true,
		".keel.json":         true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".qwen", excludeFiles, knownCategories, ir)
}
