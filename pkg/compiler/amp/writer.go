package amp

import (
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("amp", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Amp's native format.
// Output paths are determined by the AgentProfile (scope-dependent).
type Writer struct{}

func (w *Writer) Write(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string, _ compiler.CompileOptions) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord
	agentDir := filepath.Join(outputDir, profile.OutputRoot)

	// 1. Create output directories.
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return nil, err
	}

	// 2. Instructions
	if rec, ok, err := compiler.WriteInstructions(result, profile, outputDir, nil); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 3. Settings — result.Settings is already fully assembled (with embedded MCP and permissions)
	if rec, ok, err := compiler.WriteSettings(result, profile, outputDir); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 4. Common output (categories, bin)
	commonRecords, err := compiler.WriteCommonOutput(result, profile, outputDir)
	if err != nil {
		return nil, err
	}
	records = append(records, commonRecords...)

	return records, nil
}

// TranslateMCPToAmp converts IR MCP format to Amp format.
// Amp uses the same format as Claude (command + args), so this is mostly a passthrough.
// Input:  {"mcpServers": {"name": {"command": "cmd", "args": ["a","b"], "env": {"K":"V"}}}}
// Output: {"name": {"command": "cmd", "args": ["a","b"], "env": {"K":"V"}}}
func TranslateMCPToAmp(mcpData map[string]any) map[string]any {
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

	// Amp uses the same format as Claude/IR, so just return the servers map
	if len(servers) == 0 {
		return nil
	}

	return servers
}

// TranslatePermissionsToAmp converts IR permissions format to Amp format.
// Amp uses amp.commands.allowlist for allowed shell commands.
// Input:  {"allow": ["Bash(npm test)", "Bash(npm build)", "Bash"], "deny": ["Edit"]}
// Output: ["npm test", "npm build", "*"]
func TranslatePermissionsToAmp(perms map[string]any) []any {
	if perms == nil || len(perms) == 0 {
		return nil
	}

	var result []any

	// Process allow list - only Bash commands for amp.commands.allowlist
	if allowRaw, ok := perms["allow"]; ok {
		if allow, ok := allowRaw.([]any); ok {
			for _, perm := range allow {
				if s, ok := perm.(string); ok {
					if entry := translateBashPermToAmp(s); entry != "" {
						result = append(result, entry)
					}
				}
			}
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

// translateBashPermToAmp extracts shell commands from Bash permissions.
// Only processes Bash() entries, ignores other tools.
func translateBashPermToAmp(perm string) string {
	// Check if this is a Bash permission
	if !strings.HasPrefix(perm, "Bash") {
		return "" // Ignore non-Bash permissions
	}

	// Handle bare "Bash" -> "*"
	if perm == "Bash" {
		return "*"
	}

	// Parse Bash(command) format
	openParen := strings.Index(perm, "(")
	if openParen == -1 {
		return ""
	}

	// Find matching closing paren
	if !strings.HasSuffix(perm, ")") {
		return ""
	}

	// Extract command
	command := perm[openParen+1 : len(perm)-1]
	return command
}

// TranslateDenyListToAmp converts IR deny list to Amp format.
// Amp uses amp.tools.disable with tool names.
// Input:  ["Bash", "Edit", "WebSearch"]
// Output: ["bash", "edit", "websearch"]
func TranslateDenyListToAmp(denies []any) []any {
	if denies == nil || len(denies) == 0 {
		return nil
	}

	var result []any

	for _, itemRaw := range denies {
		item, ok := itemRaw.(string)
		if !ok {
			continue
		}

		// Map IR tool names to Amp tool names (lowercase)
		toolName := mapIRToolToAmp(item)
		result = append(result, toolName)
	}

	return result
}

// mapIRToolToAmp maps IR tool names to Amp tool names.
func mapIRToolToAmp(tool string) string {
	switch tool {
	case "Bash":
		return "bash"
	case "Read":
		return "read"
	case "Edit", "Write":
		return "edit"
	case "WebSearch":
		return "websearch"
	case "WebFetch":
		return "webfetch"
	case "Agent":
		return "agent"
	default:
		// For unknown tools, lowercase
		return strings.ToLower(tool)
	}
}
