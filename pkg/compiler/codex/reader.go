package codex

import (
	"log/slog"
	"strings"
	"unicode"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
)

func init() {
	compiler.RegisterReader("codex", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Codex CLI native configuration into a LayerIR.
// Codex stores config in .codex/config.toml (or config.json) and instructions
// in AGENTS.md.
type Reader struct{}

func (r *Reader) AgentName() string { return "codex" }

// Read reads a Codex project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "codex",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills"}
	if err := compiler.ReadCategories(layerPath, ".codex", categories, ir); err != nil {
		return nil, err
	}

	compiler.ReadSnippets(layerPath, ".codex", ir)
	compiler.ReadBinDir(layerPath, ir)

	r.readOtherDotDirFiles(layerPath, ir)

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .codex/config.json,
// .codex/config.toml, and layer-root config files. The first found file wins.
// It decomposes the contents into MCP, permissions, portable settings, and
// Codex-specific settings.
//
// Note: TOML parsing is not implemented yet. If config.toml is found, it's skipped
// with a warning. Add a TOML library dependency to support native config.toml reading.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := compiler.SettingsReadCandidates(layerPath, ".codex", ProjectProfile().SettingsCandidates)

	// Filter out .toml files for now (no TOML parser available)
	jsonCandidates := []string{}
	for _, candidate := range candidates {
		if strings.HasSuffix(candidate, ".toml") {
			slog.Debug("skipping TOML file (no parser available)", "file", candidate)
			continue
		}
		jsonCandidates = append(jsonCandidates, candidate)
	}

	raw, err := compiler.ReadJSONConfigStripped(jsonCandidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read codex config", "keys", len(raw), "layer", layerPath)

	// Extract MCP and translate from Codex format to IR format
	if mcpRaw, ok := raw["mcp"]; ok {
		if mcpMap, ok := mcpRaw.(map[string]any); ok {
			if translated := TranslateMCPFromCodex(mcpMap); len(translated) > 0 {
				ir.MCP = translated
			}
		}
		delete(raw, "mcp")
	}

	// Extract permissions and translate from Codex approval_policy format to IR format
	if policyRaw, ok := raw["approval_policy"]; ok {
		if policyMap, ok := policyRaw.(map[string]any); ok {
			if translated := TranslatePermissionsFromCodex(policyMap); len(translated) > 0 {
				ir.Permissions = translated
			}
		}
		delete(raw, "approval_policy")
	}

	// Extract portable model setting
	if modelVal, ok := raw["model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = modelVal
		delete(raw, "model")
	}

	// Remaining keys are Codex-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["codex"] == nil {
			ir.AgentSpecific["codex"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["codex"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "codex")

	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .codex/ directory
// that are not handled by specific readers (settings, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"config.toml":          true,
		"config.json":          true,
		"settings.codex.json":  true,
		".keel.json":           true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".codex", excludeFiles, knownCategories, ir)
}

// TranslateMCPFromCodex converts Codex MCP format to IR format.
// Input:  {"servers": {"name": {"command": ["cmd","arg"], "env": {"K":"V"}}}}
// Output: {"mcpServers": {"name": {"command": "cmd", "args": ["arg"], "env": {"K":"V"}}}}
func TranslateMCPFromCodex(mcpData map[string]any) map[string]any {
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

		// Handle URL-based (remote) servers
		if url, ok := server["url"].(string); ok {
			irServer["url"] = url
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

		// Keep env as-is (Codex uses "env" like IR)
		if envRaw, ok := server["env"]; ok {
			if env, ok := envRaw.(map[string]any); ok {
				irServer["env"] = env
			}
		}

		// Copy other fields except command and env (already processed)
		for k, v := range server {
			if k != "command" && k != "env" && k != "url" {
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

// TranslatePermissionsFromCodex converts Codex approval_policy format to IR format.
// Input:  {"auto_approve": ["shell:npm test", "read:*"], "auto_deny": ["shell:rm *"]}
// Output: {"allow": ["Bash(npm test)", "Read"], "deny": ["Bash(rm *)"]}
func TranslatePermissionsFromCodex(policy map[string]any) map[string]any {
	if policy == nil || len(policy) == 0 {
		return nil
	}

	allows := []any{}
	denies := []any{}

	// Process auto_approve list
	if approveRaw, ok := policy["auto_approve"]; ok {
		if approve, ok := approveRaw.([]any); ok {
			for _, entry := range approve {
				if s, ok := entry.(string); ok {
					if perm := translateCodexEntryToIR(s); perm != "" {
						allows = append(allows, perm)
					}
				}
			}
		}
	}

	// Process auto_deny list
	if denyRaw, ok := policy["auto_deny"]; ok {
		if deny, ok := denyRaw.([]any); ok {
			for _, entry := range deny {
				if s, ok := entry.(string); ok {
					if perm := translateCodexEntryToIR(s); perm != "" {
						denies = append(denies, perm)
					}
				}
			}
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

// translateCodexEntryToIR converts a Codex approval entry to IR permission format.
// Input:  "shell:npm test" or "read:*"
// Output: "Bash(npm test)" or "Read"
func translateCodexEntryToIR(entry string) string {
	parts := strings.SplitN(entry, ":", 2)
	if len(parts) != 2 {
		return ""
	}

	action := parts[0]
	resource := parts[1]

	toolName := mapCodexActionToToolName(action)

	// Build permission string
	if resource == "" || resource == "*" {
		return toolName
	}
	return toolName + "(" + resource + ")"
}

// mapCodexActionToToolName maps Codex action names back to IR tool names.
func mapCodexActionToToolName(action string) string {
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
