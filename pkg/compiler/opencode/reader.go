package opencode

import (
	"log/slog"
	"unicode"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
	compiler.RegisterReader("opencode", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads OpenCode V2 native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "opencode" }

// Read reads an OpenCode project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "opencode",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills"}
	if err := compiler.ReadCategories(layerPath, ".opencode", categories, ir); err != nil {
		return nil, err
	}

	compiler.ReadSnippets(layerPath, ".opencode", ir)
	compiler.ReadBinDir(layerPath, ir)

	r.readOtherDotDirFiles(layerPath, ir)

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .opencode/opencode.json
// and layer-root opencode.json (and the convention name settings.opencode.json).
// The first found file wins. It decomposes the contents into MCP, permissions,
// portable settings, and OpenCode-specific settings.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".opencode", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read opencode.json", "keys", len(raw), "layer", layerPath)

	// Extract MCP and translate from OpenCode V2 format to IR format
	if mcpRaw, ok := raw["mcp"]; ok {
		if mcpMap, ok := mcpRaw.(map[string]any); ok {
			if translated := TranslateMCPFromOpenCode(mcpMap); len(translated) > 0 {
				ir.MCP = translated
			}
		}
		delete(raw, "mcp")
	}

	// Extract permissions and translate from OpenCode V2 format to IR format
	if permsRaw, ok := raw["permissions"]; ok {
		if permsArray, ok := permsRaw.([]any); ok {
			if translated := TranslatePermissionsFromOpenCode(permsArray); len(translated) > 0 {
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

	// Remaining keys are OpenCode-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["opencode"] == nil {
			ir.AgentSpecific["opencode"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["opencode"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "opencode")

	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .opencode/ directory
// that are not handled by specific readers (settings, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"opencode.json":         true,
		"settings.opencode.json": true,
		".keel.json":            true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".opencode", excludeFiles, knownCategories, ir)
}

// TranslateMCPFromOpenCode converts OpenCode V2 MCP format to IR format.
// Input:  {"servers": {"name": {"type": "local", "command": ["cmd","arg"], "environment": {"K":"V"}}}}
// Output: {"mcpServers": {"name": {"command": "cmd", "args": ["arg"], "env": {"K":"V"}}}}
func TranslateMCPFromOpenCode(mcpData map[string]any) map[string]any {
	if mcpData == nil || len(mcpData) == 0 {
		return nil
	}

	serversRaw, ok := mcpData["servers"]
	if !ok {
		return nil
	}

	servers, ok := serversRaw.(map[string]any)
	if !ok {
		return nil
	}

	translated := make(map[string]any)
	for name, serverRaw := range servers {
		server, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}

		irServer := make(map[string]any)

		// Handle remote vs local servers
		serverType, _ := server["type"].(string)
		if serverType == "remote" {
			// Remote server: keep URL, don't process command array
			if url, ok := server["url"].(string); ok {
				irServer["url"] = url
			}
		} else {
			// Local server: split command array into command + args
			if cmdRaw, ok := server["command"]; ok {
				if cmdArray, ok := cmdRaw.([]any); ok && len(cmdArray) > 0 {
					// First element is command
					if cmd, ok := cmdArray[0].(string); ok {
						irServer["command"] = cmd
					}
					// Rest are args
					if len(cmdArray) > 1 {
						irServer["args"] = cmdArray[1:]
					}
				}
			}
		}

		// Rename environment → env
		if envRaw, ok := server["environment"]; ok {
			if env, ok := envRaw.(map[string]any); ok {
				irServer["env"] = env
			}
		}

		// Copy other fields except type, disabled, timeout, protocol (OpenCode-specific)
		for k, v := range server {
			if k != "command" && k != "environment" && k != "type" && k != "disabled" && k != "timeout" && k != "protocol" {
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

// TranslatePermissionsFromOpenCode converts OpenCode V2 permissions format to IR format.
// Input:  [{"action":"shell","resource":"npm test","effect":"allow"}, ...]
// Output: {"allow": ["Bash(npm test)"], "deny": [...]}
func TranslatePermissionsFromOpenCode(perms []any) map[string]any {
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

		action, _ := perm["action"].(string)
		resource, _ := perm["resource"].(string)
		effect, _ := perm["effect"].(string)

		if action == "" || effect == "" {
			continue
		}

		// Map action back to Claude tool name
		toolName := mapActionToToolName(action)

		// Build permission string
		var permStr string
		if resource == "" || resource == "*" {
			permStr = toolName
		} else {
			permStr = toolName + "(" + resource + ")"
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

// mapActionToToolName maps OpenCode V2 action names back to IR tool names.
func mapActionToToolName(action string) string {
	switch action {
	case "shell":
		return "Bash"
	case "read":
		return "Read"
	case "edit":
		return "Edit"
	case "websearch":
		return "WebSearch"
	case "webfetch":
		return "WebFetch"
	case "subagent":
		return "Agent"
	default:
		// MCP tools or unknown — Title case passthrough
		if action == "" {
			return action
		}
		runes := []rune(action)
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}
}
