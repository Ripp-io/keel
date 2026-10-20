package amp

import (
	"log/slog"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
	compiler.RegisterReader("amp", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Amp native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "amp" }

// Read reads an Amp project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "amp",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills"}
	if err := compiler.ReadCategories(layerPath, ".amp", categories, ir); err != nil {
		return nil, err
	}

	compiler.ReadSnippets(layerPath, ".amp", ir)
	compiler.ReadBinDir(layerPath, ir)

	r.readOtherDotDirFiles(layerPath, ir)

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .amp/settings.json
// and layer-root settings.json (and the convention name settings.amp.json).
// The first found file wins. It decomposes the contents into MCP, permissions,
// portable settings, and Amp-specific settings.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".amp", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read amp settings.json", "keys", len(raw), "layer", layerPath)

	// Extract MCP and translate from Amp format to IR format
	if mcpRaw, ok := raw["amp.mcpServers"]; ok {
		if mcpMap, ok := mcpRaw.(map[string]any); ok {
			if translated := TranslateMCPFromAmp(mcpMap); len(translated) > 0 {
				ir.MCP = translated
			}
		}
		delete(raw, "amp.mcpServers")
	}

	// Extract permissions and translate from Amp format to IR format
	if permsRaw, ok := raw["amp.commands.allowlist"]; ok {
		if permsArray, ok := permsRaw.([]any); ok {
			if translated := TranslatePermissionsFromAmp(permsArray); len(translated) > 0 {
				ir.Permissions = translated
			}
		}
		delete(raw, "amp.commands.allowlist")
	}

	// Also check for amp.tools.disable (deny list)
	if denyRaw, ok := raw["amp.tools.disable"]; ok {
		if denyArray, ok := denyRaw.([]any); ok {
			if translated := TranslateDenyListFromAmp(denyArray); len(translated) > 0 {
				if ir.Permissions == nil {
					ir.Permissions = make(map[string]any)
				}
				ir.Permissions["deny"] = translated
			}
		}
		delete(raw, "amp.tools.disable")
	}

	// Extract portable model setting
	if modelVal, ok := raw["amp.model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = modelVal
		delete(raw, "amp.model")
	}

	// Remaining keys are Amp-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["amp"] == nil {
			ir.AgentSpecific["amp"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["amp"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "amp")

	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .amp/ directory
// that are not handled by specific readers (settings, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"settings.json":     true,
		"settings.amp.json": true,
		".keel.json":        true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".amp", excludeFiles, knownCategories, ir)
}

// TranslateMCPFromAmp converts Amp MCP format to IR format.
// Amp uses the same format as Claude (command + args), so this is mostly a passthrough.
// Input:  {"server-name": {"command": "cmd", "args": ["arg"], "env": {"K":"V"}}}
// Output: {"mcpServers": {"server-name": {"command": "cmd", "args": ["arg"], "env": {"K":"V"}}}}
func TranslateMCPFromAmp(mcpData map[string]any) map[string]any {
	if mcpData == nil || len(mcpData) == 0 {
		return nil
	}

	translated := make(map[string]any)
	for name, serverRaw := range mcpData {
		server, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}

		// Amp uses the same format as Claude/IR, so just copy the server config
		translated[name] = server
	}

	if len(translated) == 0 {
		return nil
	}

	return map[string]any{"mcpServers": translated}
}

// TranslatePermissionsFromAmp converts Amp permissions format to IR format.
// Amp uses amp.commands.allowlist with command strings.
// Input:  ["npm test", "npm build", "*"]
// Output: {"allow": ["Bash(npm test)", "Bash(npm build)", "Bash"]}
func TranslatePermissionsFromAmp(allowlist []any) map[string]any {
	if allowlist == nil || len(allowlist) == 0 {
		return nil
	}

	allows := []any{}

	for _, itemRaw := range allowlist {
		item, ok := itemRaw.(string)
		if !ok {
			continue
		}

		// Amp allowlist items are shell commands or "*" for all commands
		var permStr string
		if item == "*" {
			permStr = "Bash"
		} else {
			permStr = "Bash(" + item + ")"
		}
		allows = append(allows, permStr)
	}

	if len(allows) == 0 {
		return nil
	}

	return map[string]any{"allow": allows}
}

// TranslateDenyListFromAmp converts Amp deny list to IR format.
// Amp uses amp.tools.disable with tool names.
// Input:  ["Bash", "Edit", "WebSearch"]
// Output: ["Bash", "Edit", "WebSearch"]
func TranslateDenyListFromAmp(denyList []any) []any {
	if denyList == nil || len(denyList) == 0 {
		return nil
	}

	denies := []any{}

	for _, itemRaw := range denyList {
		item, ok := itemRaw.(string)
		if !ok {
			continue
		}

		// Map Amp tool names to IR tool names
		toolName := mapAmpToolToIR(item)
		denies = append(denies, toolName)
	}

	return denies
}

// mapAmpToolToIR maps Amp tool names to IR tool names.
func mapAmpToolToIR(tool string) string {
	// Amp tool names are typically lowercase or camelCase
	// Map common variations to IR tool names (PascalCase)
	switch strings.ToLower(tool) {
	case "bash", "shell":
		return "Bash"
	case "read":
		return "Read"
	case "edit", "write":
		return "Edit"
	case "websearch":
		return "WebSearch"
	case "webfetch":
		return "WebFetch"
	case "agent", "subagent":
		return "Agent"
	default:
		// For unknown tools, capitalize first letter
		if tool == "" {
			return tool
		}
		// If already capitalized, return as-is
		if len(tool) > 0 && tool[0] >= 'A' && tool[0] <= 'Z' {
			return tool
		}
		// Otherwise, capitalize first letter
		return strings.ToUpper(tool[:1]) + tool[1:]
	}
}
