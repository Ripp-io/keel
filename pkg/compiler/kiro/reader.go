package kiro

import (
	"log/slog"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
	compiler.RegisterReader("kiro", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Kiro native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "kiro" }

// Read reads a Kiro project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "kiro",
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
	if err := compiler.ReadCategories(layerPath, ".kiro", categories, ir); err != nil {
		return nil, err
	}

	compiler.ReadSnippets(layerPath, ".kiro", ir)
	compiler.ReadBinDir(layerPath, ir)

	r.readOtherDotDirFiles(layerPath, ir)

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .kiro/settings/cli.json
// and layer-root cli.json (and the convention name settings.kiro.json).
// The first found file wins.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".kiro", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read kiro settings", "keys", len(raw), "layer", layerPath)

	// Extract permissions and translate from Kiro format to IR format
	if permsRaw, ok := raw["permissions"]; ok {
		if permsArray, ok := permsRaw.([]any); ok {
			if translated := TranslatePermissionsFromKiro(permsArray); len(translated) > 0 {
				ir.Permissions = translated
			}
		}
		delete(raw, "permissions")
	}

	// Extract portable model setting
	if modelVal, ok := raw["model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = modelVal
		delete(raw, "model")
	}

	// Remaining keys are Kiro-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["kiro"] == nil {
			ir.AgentSpecific["kiro"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["kiro"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "kiro")

	return nil
}

// readMCP reads MCP configuration from .kiro/settings/mcp.json.
func (r *Reader) readMCP(layerPath string, ir *compiler.LayerIR) error {
	mcpPath := filepath.Join(layerPath, ".kiro", "settings", "mcp.json")
	mcpData, err := compiler.ReadJSONConfig([]string{mcpPath})
	if err != nil {
		return err
	}
	if mcpData == nil {
		return nil
	}

	slog.Debug("read kiro mcp.json", "keys", len(mcpData), "layer", layerPath)

	// Kiro uses the same MCP format as Claude/IR (mcpServers)
	if serversRaw, ok := mcpData["mcpServers"]; ok {
		if servers, ok := serversRaw.(map[string]any); ok && len(servers) > 0 {
			ir.MCP = map[string]any{"mcpServers": servers}
		}
	}

	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .kiro/ directory
// that are not handled by specific readers (settings, mcp, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"settings/cli.json":     true,
		"settings/mcp.json":     true,
		"cli.json":              true,
		"settings.kiro.json":    true,
		".keel.json":            true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets", "settings",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".kiro", excludeFiles, knownCategories, ir)
}

// TranslatePermissionsFromKiro converts Kiro permissions format to IR format.
// Kiro uses a similar format to OpenCode: array of permission objects.
// Input:  [{"tool":"Bash","resource":"npm test","effect":"allow"}, ...]
// Output: {"allow": ["Bash(npm test)"], "deny": [...]}
func TranslatePermissionsFromKiro(perms []any) map[string]any {
	if perms == nil || len(perms) == 0 {
		return nil
	}

	allows := []any{}
	denies := []any{}

	for _, permRaw := range perms {
		perm, ok := permRaw.(map[string]any)
		if !ok {
			continue
		}

		tool, _ := perm["tool"].(string)
		resource, _ := perm["resource"].(string)
		effect, _ := perm["effect"].(string)

		if tool == "" || effect == "" {
			continue
		}

		// Build permission string
		var permStr string
		if resource == "" || resource == "*" {
			permStr = tool
		} else {
			permStr = tool + "(" + resource + ")"
		}

		// Sort into allow/deny lists
		if effect == "allow" {
			allows = append(allows, permStr)
		} else if effect == "deny" {
			denies = append(denies, permStr)
		}
	}

	if len(allows) == 0 && len(denies) == 0 {
		return nil
	}

	result := make(map[string]any)
	if len(allows) > 0 {
		result["allow"] = allows
	}
	if len(denies) > 0 {
		result["deny"] = denies
	}

	return result
}
