package agentplugins

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("agentplugins", func() compiler.Writer { return &Writer{} })
}

// schemaBaseURL is the base URL for Agent Plugins 1.0.0 JSON schemas.
const schemaBaseURL = "https://agent-plugins.org/schemas/1.0.0"

// Writer serializes an AccumulatedResult to the Agent Plugins 1.0 format.
// Output structure:
//
//	plugin.json   manifest at output root
//	skills/       SKILL.md files at output root
//	mcp.json      MCP config at output root (only if MCP servers exist)
type Writer struct{}

func (w *Writer) Write(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string, _ compiler.CompileOptions) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord

	// Create .agentplugins/ for the .keel.json manifest.
	agentDir := filepath.Join(outputDir, profile.OutputRoot)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return nil, err
	}

	// 1. Write skills/ at output root (not under .agentplugins/).
	if skills, ok := result.Content.Categories["skills"]; ok && len(skills) > 0 {
		catRecords, err := compiler.WriteCategory(outputDir, "skills", skills)
		if err != nil {
			return nil, err
		}
		records = append(records, catRecords...)
	}

	// 2. Write mcp.json at output root (only if MCP servers exist).
	if len(result.Content.MCP) > 0 {
		mcpData := TranslateMCPToAgentPlugins(result.Content.MCP)
		mcpPath := filepath.Join(outputDir, "mcp.json")
		hash, err := compiler.WriteJSONFileWithHash(mcpPath, mcpData)
		if err != nil {
			return nil, err
		}
		records = append(records, manifest.FileRecord{
			Output:   "mcp.json",
			Strategy: "deep-merge",
			Hash:     hash,
		})
	}

	// 3. Generate and write plugin.json at output root.
	pluginMeta := buildPluginManifest(result)
	pluginPath := filepath.Join(outputDir, "plugin.json")
	hash, err := compiler.WriteJSONFileWithHash(pluginPath, pluginMeta)
	if err != nil {
		return nil, err
	}
	records = append(records, manifest.FileRecord{
		Output:   "plugin.json",
		Strategy: "deep-merge",
		Hash:     hash,
	})

	// 4. State directory.
	if err := compiler.WriteStateDir(outputDir); err != nil {
		return nil, err
	}

	return records, nil
}

// buildPluginManifest constructs the plugin.json content from accumulated data.
// Fields come from keel.yml metadata and template variables.
//
// Required fields: $schema, name
// Optional fields: version, description, author, homepage, repository, license, keywords
func buildPluginManifest(result *compiler.AccumulatedResult) map[string]any {
	meta := map[string]any{
		"$schema": schemaBaseURL + "/plugin.schema.json",
		"name":    sanitizePluginName(result.Metadata.Name),
	}

	if result.Metadata.Description != "" {
		meta["description"] = result.Metadata.Description
	}

	vars := result.Tracking.Variables

	if v := lookupVar(vars, "plugin.version"); v != "" {
		meta["version"] = v
	}

	// author object — only included if at least one field is set
	author := buildAuthor(vars)
	if author != nil {
		meta["author"] = author
	}

	// Simple string fields from variables
	for _, field := range []struct{ manifestKey, varKey string }{
		{"homepage", "plugin.homepage"},
		{"repository", "plugin.repository"},
		{"license", "plugin.license"},
	} {
		if v := lookupVar(vars, field.varKey); v != "" {
			meta[field.manifestKey] = v
		}
	}

	// keywords from comma-separated variable
	if v := lookupVar(vars, "plugin.keywords"); v != "" {
		keywords := splitKeywords(v)
		if len(keywords) > 0 {
			meta["keywords"] = keywords
		}
	}

	// Provenance extension — lightweight compilation metadata.
	// Full provenance (hashes, governance, signatures) is in .keel.json.
	extensions := buildExtensions(result)
	if len(extensions) > 0 {
		meta["extensions"] = extensions
	}

	return meta
}

// buildExtensions constructs the extensions object for plugin.json.
// Includes Keel provenance under the io.ripp.keel namespace.
func buildExtensions(result *compiler.AccumulatedResult) map[string]any {
	provenance := map[string]any{
		"compiled_at":  time.Now().UTC().Format(time.RFC3339),
		"keel_version": result.Tracking.Variables["keel.version"],
	}

	// Layer names (sorted for deterministic output).
	if len(result.Metadata.LayerNames) > 0 {
		var names []string
		for _, name := range result.Metadata.LayerNames {
			if name != "" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		if len(names) > 0 {
			layerList := make([]any, len(names))
			for i, n := range names {
				layerList[i] = n
			}
			provenance["layers"] = layerList
		}
	}

	// Merge any existing extensions from AgentSpecific (round-tripped from
	// a previously imported plugin.json).
	extensions := make(map[string]any)
	if agentData, ok := result.Content.AgentSpecific["agentplugins"]; ok {
		if ext, ok := agentData["extensions"].(map[string]any); ok {
			for k, v := range ext {
				extensions[k] = v
			}
		}
	}

	// Map agent-specific settings to vendor extension namespaces.
	// Agents whose data appears in the IR get a corresponding extension entry
	// so the plugin can carry agent-specific configuration portably.
	// Agents without a known namespace use io.keel.agent.<name> as fallback
	// to preserve data for round-trip fidelity.
	for agent, data := range result.Content.AgentSpecific {
		if agent == "agentplugins" {
			continue // already handled above
		}
		ns := agentToNamespace(agent)
		if ns == "" {
			ns = "io.keel.agent." + agent
		}
		extensions[ns] = data
	}

	extensions["io.ripp.keel"] = provenance
	return extensions
}

// agentToNamespace maps IR agent names to reverse-domain extension namespaces
// per the Agent Plugins 1.0 convention.
var agentNamespaces = map[string]string{
	"claude":       "com.anthropic.claude",
	"cursor":       "com.anysphere.cursor",
	"codex":        "com.openai.codex",
	"copilot":      "com.github.copilot",
	"kiro":         "com.aws.kiro",
	"antigravity":  "com.google.antigravity",
	"kilo":         "com.kilocode.kilo",
	"amp":          "com.sourcegraph.amp",
	"goose":        "xyz.block.goose",
	"opencode":     "dev.opencode",
	"vibe":         "com.mistral.vibe",
	"junie":        "com.jetbrains.junie",
}

func agentToNamespace(agent string) string {
	return agentNamespaces[agent]
}

// buildAuthor constructs the author object from variables.
// Returns nil if no author fields are set.
func buildAuthor(vars map[string]string) map[string]any {
	author := map[string]any{}
	if v := lookupVar(vars, "plugin.author.name"); v != "" {
		author["name"] = v
	}
	if v := lookupVar(vars, "plugin.author.email"); v != "" {
		author["email"] = v
	}
	if v := lookupVar(vars, "plugin.author.url"); v != "" {
		author["url"] = v
	}
	if len(author) == 0 {
		return nil
	}
	return author
}

// lookupVar returns the first non-empty value from the variables map for the
// given keys. Returns "" if none are set.
func lookupVar(vars map[string]string, keys ...string) string {
	for _, key := range keys {
		if v, ok := vars[key]; ok && v != "" {
			return v
		}
	}
	return ""
}

// splitKeywords splits a comma-separated string into trimmed, non-empty strings.
func splitKeywords(s string) []any {
	parts := strings.Split(s, ",")
	var result []any
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// validPluginName matches the Agent Plugins 1.0 name constraints:
// 1-64 chars, only a-z0-9-., no -- or .., must start and end alphanumeric.
var validPluginName = regexp.MustCompile(`^[a-z0-9]([a-z0-9.\-]{0,62}[a-z0-9])?$`)
var invalidChars = regexp.MustCompile(`[^a-z0-9.\-]`)
var repeatedSep = regexp.MustCompile(`[-]{2,}|[.]{2,}`)

// sanitizePluginName converts a keel.yml name into a valid Agent Plugins name.
// Lowercases, replaces invalid chars with hyphens, collapses repeated
// separators, trims non-alphanumeric edges, and truncates to 64 chars.
func sanitizePluginName(name string) string {
	if name == "" {
		return "unnamed-plugin"
	}

	s := strings.ToLower(name)
	s = invalidChars.ReplaceAllString(s, "-")
	s = repeatedSep.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-.")

	if len(s) > 64 {
		s = s[:64]
		s = strings.TrimRight(s, "-.")
	}

	if s == "" {
		return "unnamed-plugin"
	}

	if validPluginName.MatchString(s) {
		return s
	}

	return "unnamed-plugin"
}

// TranslateMCPToAgentPlugins converts IR MCP format to Agent Plugins 1.0 format.
// Adds $schema and infers the transport type for each server:
//   - stdio: has "command" field
//   - streamable-http: has "url" field (non-SSE)
//   - sse: has "url" field with explicit SSE marker
//
// Input:  {"mcpServers": {"name": {"command": "cmd", "args": [...], "env": {...}}}}
// Output: {"$schema": "...", "mcpServers": {"name": {"type": "stdio", "command": "cmd", "args": [...], "env": {...}}}}
func TranslateMCPToAgentPlugins(mcpData map[string]any) map[string]any {
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

		apServer := make(map[string]any)

		// Infer transport type from fields present.
		_, hasCommand := server["command"]
		_, hasURL := server["url"]

		switch {
		case hasCommand:
			apServer["type"] = "stdio"
		case hasURL:
			// Default to streamable-http for URL-based servers.
			// SSE is legacy — no heuristic to distinguish, so use the modern default.
			// Known limitation: SSE servers imported via the reader lose their
			// explicit type and become streamable-http on re-export.
			apServer["type"] = "streamable-http"
		default:
			// Server has neither command nor url — skip it.
			// The Agent Plugins spec requires a type, and we can't infer one.
			continue
		}

		// Copy all fields from the IR server config.
		for k, v := range server {
			apServer[k] = v
		}

		translated[name] = apServer
	}

	if len(translated) == 0 {
		return nil
	}

	return map[string]any{
		"$schema":    schemaBaseURL + "/mcp.schema.json",
		"mcpServers": translated,
	}
}
