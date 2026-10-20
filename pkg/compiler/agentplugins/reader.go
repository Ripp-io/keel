package agentplugins

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
)

// Reader reads an Agent Plugins 1.0 directory and normalizes it into a LayerIR.
// This enables importing existing plugins as Keel layers or using them as
// native-format input for cross-agent compilation.
//
// Reads:
//   - plugin.json → metadata (name, description) and variables (version, author, etc.)
//   - skills/     → categories["skills"]
//   - mcp.json    → MCP config (translated from Agent Plugins format to IR)
type Reader struct{}

func (r *Reader) AgentName() string { return "agentplugins" }

// Read reads an Agent Plugins directory and returns a normalized IR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "agentplugins",
		LayerPath:   layerPath,
	}

	if err := r.readPluginJSON(layerPath, ir); err != nil {
		return nil, err
	}

	if err := r.readMCP(layerPath, ir); err != nil {
		return nil, err
	}

	// Read skills/ at the plugin root.
	if err := compiler.ReadCategoryDir(filepath.Join(layerPath, "skills"), "skills", ir, nil); err != nil {
		return nil, err
	}

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readPluginJSON reads plugin.json and extracts metadata and variables.
// Metadata (name, description) goes to IR top-level fields.
// Other fields (version, author, license, etc.) are stored as variables
// for downstream compilation.
func (r *Reader) readPluginJSON(layerPath string, ir *compiler.LayerIR) error {
	data, err := pathutil.SafeReadFile(filepath.Join(layerPath, "plugin.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	var plugin map[string]any
	if err := json.Unmarshal(data, &plugin); err != nil {
		return err
	}

	slog.Debug("read plugin.json", "keys", len(plugin), "layer", layerPath)

	// Extract metadata.
	if name, ok := plugin["name"].(string); ok {
		ir.Name = name
	}
	if desc, ok := plugin["description"].(string); ok {
		ir.Description = desc
	}

	// Extract optional fields into variables for round-trip fidelity.
	vars := make(map[string]string)

	if v, ok := plugin["version"].(string); ok && v != "" {
		vars["plugin.version"] = v
	}
	if v, ok := plugin["license"].(string); ok && v != "" {
		vars["plugin.license"] = v
	}
	if v, ok := plugin["homepage"].(string); ok && v != "" {
		vars["plugin.homepage"] = v
	}
	if v, ok := plugin["repository"].(string); ok && v != "" {
		vars["plugin.repository"] = v
	}

	// Author object → individual variables.
	if author, ok := plugin["author"].(map[string]any); ok {
		if v, ok := author["name"].(string); ok && v != "" {
			vars["plugin.author.name"] = v
		}
		if v, ok := author["email"].(string); ok && v != "" {
			vars["plugin.author.email"] = v
		}
		if v, ok := author["url"].(string); ok && v != "" {
			vars["plugin.author.url"] = v
		}
	}

	// Keywords array → comma-separated variable.
	if kwRaw, ok := plugin["keywords"].([]any); ok && len(kwRaw) > 0 {
		var keywords []string
		for _, kw := range kwRaw {
			if s, ok := kw.(string); ok {
				keywords = append(keywords, s)
			}
		}
		if len(keywords) > 0 {
			vars["plugin.keywords"] = strings.Join(keywords, ", ")
		}
	}

	if len(vars) > 0 {
		ir.Variables = vars
	}

	// Store extensions as agent-specific data.
	// Vendor extensions (com.anthropic.claude, etc.) are mapped back to IR
	// agent names. Unknown namespaces are preserved under "agentplugins".
	if ext, ok := plugin["extensions"].(map[string]any); ok && len(ext) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		unknownExt := make(map[string]any)
		for ns, data := range ext {
			if ns == "io.ripp.keel" {
				continue // provenance — don't round-trip
			}
			// Check known vendor namespace first.
			if agent := namespaceToAgent(ns); agent != "" {
				if dataMap, ok := data.(map[string]any); ok {
					ir.AgentSpecific[agent] = dataMap
				}
			} else if strings.HasPrefix(ns, "io.keel.agent.") {
				// Keel fallback namespace — extract agent name.
				agent := strings.TrimPrefix(ns, "io.keel.agent.")
				if agent != "" {
					if dataMap, ok := data.(map[string]any); ok {
						ir.AgentSpecific[agent] = dataMap
					}
				}
			} else {
				unknownExt[ns] = data
			}
		}
		if len(unknownExt) > 0 {
			if ir.AgentSpecific["agentplugins"] == nil {
				ir.AgentSpecific["agentplugins"] = make(map[string]any)
			}
			ir.AgentSpecific["agentplugins"]["extensions"] = unknownExt
		}
	}

	return nil
}

// readMCP reads mcp.json and translates from Agent Plugins format to IR format.
// Strips the $schema and type fields that are Agent Plugins-specific.
func (r *Reader) readMCP(layerPath string, ir *compiler.LayerIR) error {
	data, err := pathutil.SafeReadFile(filepath.Join(layerPath, "mcp.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	var mcp map[string]any
	if err := json.Unmarshal(data, &mcp); err != nil {
		return err
	}

	if translated := TranslateMCPFromAgentPlugins(mcp); len(translated) > 0 {
		ir.MCP = translated
	}

	return nil
}

// namespaceToAgent maps reverse-domain extension namespaces back to IR agent names.
func namespaceToAgent(ns string) string {
	// Build reverse map from writer's agentNamespaces.
	for agent, namespace := range agentNamespaces {
		if namespace == ns {
			return agent
		}
	}
	return ""
}

// TranslateMCPFromAgentPlugins converts Agent Plugins MCP format to IR format.
// Strips $schema and per-server type fields (which are inferred during writing).
// Input:  {"$schema": "...", "mcpServers": {"name": {"type": "stdio", "command": "cmd", ...}}}
// Output: {"mcpServers": {"name": {"command": "cmd", ...}}}
func TranslateMCPFromAgentPlugins(mcpData map[string]any) map[string]any {
	if mcpData == nil || len(mcpData) == 0 {
		return nil
	}

	serversRaw, ok := mcpData["mcpServers"]
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
		for k, v := range server {
			// Strip the type field — it's inferred during writing.
			if k == "type" {
				continue
			}
			irServer[k] = v
		}

		translated[name] = irServer
	}

	if len(translated) == 0 {
		return nil
	}

	return map[string]any{"mcpServers": translated}
}
