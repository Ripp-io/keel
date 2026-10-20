package kilo

import (
	"log/slog"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
	compiler.RegisterReader("kilo", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Kilo Code native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "kilo" }

// Read reads a Kilo Code project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "kilo",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills"}
	if err := compiler.ReadCategories(layerPath, ".kilo", categories, ir); err != nil {
		return nil, err
	}

	compiler.ReadSnippets(layerPath, ".kilo", ir)
	compiler.ReadBinDir(layerPath, ir)

	r.readOtherDotDirFiles(layerPath, ir)

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .kilo/kilo.jsonc
// and layer-root kilo.jsonc (and the convention name settings.kilo.json).
// The first found file wins. It decomposes the contents into MCP, permissions,
// portable settings, and Kilo-specific settings.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".kilo", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read kilo.jsonc", "keys", len(raw), "layer", layerPath)

	// Extract MCP and translate from Kilo format to IR format
	if mcpRaw, ok := raw["mcp"]; ok {
		if mcpMap, ok := mcpRaw.(map[string]any); ok {
			if translated := TranslateMCPFromKilo(mcpMap); len(translated) > 0 {
				ir.MCP = translated
			}
		}
		delete(raw, "mcp")
	}

	// Extract permissions and translate from Kilo format to IR format
	// Kilo uses "permission" (singular), not "permissions" (plural)
	if permsRaw, ok := raw["permission"]; ok {
		if permsMap, ok := permsRaw.(map[string]any); ok {
			if translated := TranslatePermissionsFromKilo(permsMap); len(translated) > 0 {
				ir.Permissions = translated
			}
		}
		delete(raw, "permission")
	}

	// Extract portable model setting
	if modelVal, ok := raw["model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = modelVal
		delete(raw, "model")
	}

	// Extract portable provider setting
	if providerVal, ok := raw["provider"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["provider"] = providerVal
		delete(raw, "provider")
	}

	// Remaining keys are Kilo-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["kilo"] == nil {
			ir.AgentSpecific["kilo"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["kilo"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "kilo")

	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .kilo/ directory
// that are not handled by specific readers (settings, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"kilo.jsonc":         true,
		"settings.kilo.json": true,
		".keel.json":         true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".kilo", excludeFiles, knownCategories, ir)
}

// TranslateMCPFromKilo converts Kilo MCP format to IR format.
// Kilo's format is similar to OpenCode V2 but with some variations.
// Input:  {"servers": {"name": {"command": ["cmd","arg"], "environment": {"K":"V"}}}}
// Output: {"mcpServers": {"name": {"command": "cmd", "args": ["arg"], "env": {"K":"V"}}}}
func TranslateMCPFromKilo(mcpData map[string]any) map[string]any {
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

		// Handle remote vs local servers (same as OpenCode)
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

		// Copy other fields except type, disabled, timeout, protocol (Kilo-specific)
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

// TranslatePermissionsFromKilo converts Kilo permissions format to IR format.
// Kilo uses a map structure with tool categories and actions.
// Input:  {"allow": ["action1", "action2"], "deny": ["action3"]}
// Output: {"allow": ["action1", "action2"], "deny": ["action3"]}
// Note: Kilo's permission format is already close to IR format, but we normalize it.
func TranslatePermissionsFromKilo(perms map[string]any) map[string]any {
	if perms == nil || len(perms) == 0 {
		return nil
	}

	result := make(map[string]any)

	// Process allow list
	if allowRaw, ok := perms["allow"]; ok {
		if allow, ok := allowRaw.([]any); ok && len(allow) > 0 {
			result["allow"] = allow
		}
	}

	// Process deny list
	if denyRaw, ok := perms["deny"]; ok {
		if deny, ok := denyRaw.([]any); ok && len(deny) > 0 {
			result["deny"] = deny
		}
	}

	// Process ask list (Kilo-specific)
	if askRaw, ok := perms["ask"]; ok {
		if ask, ok := askRaw.([]any); ok && len(ask) > 0 {
			// Map "ask" to agent-specific (not portable)
			// For now, we'll treat it as a variation of allow/deny
			// but store it separately for round-tripping
			result["ask"] = ask
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
