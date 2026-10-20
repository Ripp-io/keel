package antigravity

import (
	"log/slog"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
	compiler.RegisterReader("antigravity", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Antigravity CLI native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "antigravity" }

// Read reads an Antigravity project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "antigravity",
		LayerPath:   layerPath,
	}

	// Read instructions from GEMINI.md or AGENTS.md
	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	// Read settings from .gemini/antigravity-cli/settings.json or layer-root settings files
	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}

	// Read MCP from .agents/mcp_config.json or .gemini/config/mcp_config.json
	if err := r.readMCP(layerPath, ir); err != nil {
		return nil, err
	}

	// Read categories from .agents/ (skills, commands, agents)
	categories := []string{"commands", "agents", "skills"}
	if err := compiler.ReadCategories(layerPath, ".agents", categories, ir); err != nil {
		return nil, err
	}

	// Read snippets if present
	compiler.ReadSnippets(layerPath, ".gemini", ir)

	// Read bin directory
	compiler.ReadBinDir(layerPath, ir)

	// Read other files in .gemini/ that aren't handled above
	r.readOtherDotDirFiles(layerPath, ir)

	// Read keel.yml sidecar if present
	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .gemini/antigravity-cli/settings.json
// and layer-root settings files. The first found file wins.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".gemini/antigravity-cli", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read antigravity settings.json", "keys", len(raw), "layer", layerPath)

	// Extract portable model setting
	if modelVal, ok := raw["model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = modelVal
		delete(raw, "model")
	}

	// Remaining keys are Antigravity-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["antigravity"] == nil {
			ir.AgentSpecific["antigravity"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["antigravity"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific settings
	compiler.ExtractPortableConcepts(ir, "antigravity")

	return nil
}

// readMCP reads MCP configuration from mcp_config.json.
// Antigravity uses a separate MCP config file at .agents/mcp_config.json (project)
// or .gemini/config/mcp_config.json (user).
func (r *Reader) readMCP(layerPath string, ir *compiler.LayerIR) error {
	// Try .agents/mcp_config.json first (project scope), then user scope location
	candidates := []string{"mcp_config.json", ".agents/mcp_config.json", ".gemini/config/mcp_config.json"}
	return compiler.ReadMCPConfig(layerPath, candidates, ir)
}

// readOtherDotDirFiles reads arbitrary files from the .gemini/ directory
// that are not handled by specific readers (settings, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"settings.json":             true,
		"settings.antigravity.json": true,
		".keel.json":                true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".gemini", excludeFiles, knownCategories, ir)
}
