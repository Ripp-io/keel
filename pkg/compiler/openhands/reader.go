package openhands

import (
	"log/slog"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
	compiler.RegisterReader("openhands", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads OpenHands native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "openhands" }

// Read reads an OpenHands project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "openhands",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}

	if err := r.readMCP(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills"}
	if err := compiler.ReadCategories(layerPath, ".openhands", categories, ir); err != nil {
		return nil, err
	}

	compiler.ReadSnippets(layerPath, ".openhands", ir)
	compiler.ReadBinDir(layerPath, ir)

	r.readOtherDotDirFiles(layerPath, ir)

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .openhands/config.json
// and layer-root config.json (and the convention name settings.openhands.json).
// The first found file wins.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".openhands", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read config.json", "keys", len(raw), "layer", layerPath)

	// Extract portable model setting
	if modelVal, ok := raw["model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = modelVal
		delete(raw, "model")
	}

	// Remaining keys are OpenHands-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["openhands"] == nil {
			ir.AgentSpecific["openhands"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["openhands"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "openhands")

	return nil
}

// readMCP reads MCP configuration from .openhands/mcp.json.
func (r *Reader) readMCP(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".openhands", []string{"mcp.json"})
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read mcp.json", "keys", len(raw), "layer", layerPath)

	// OpenHands uses standard MCP format with mcpServers key
	if len(raw) > 0 {
		ir.MCP = raw
	}

	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .openhands/ directory
// that are not handled by specific readers (settings, categories, snippets, mcp).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"config.json":             true,
		"settings.openhands.json": true,
		"mcp.json":                true,
		".keel.json":              true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".openhands", excludeFiles, knownCategories, ir)
}
