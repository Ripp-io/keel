package opencode

import (
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("opencode", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to OpenCode V2's native format.
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

// TranslateMCPToOpenCode converts IR MCP format to OpenCode V2 format.
// Input:  {"mcpServers": {"name": {"command": "cmd", "args": ["a","b"], "env": {"K":"V"}}}}
// Output: {"servers": {"name": {"type": "local", "command": ["cmd","a","b"], "environment": {"K":"V"}}}}
func TranslateMCPToOpenCode(mcpData map[string]any) map[string]any {
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

		v2Server := make(map[string]any)

		// Determine if this is a remote or local server
		_, hasURL := server["url"]
		_, hasCommand := server["command"]
		isRemote := hasURL && !hasCommand

		if isRemote {
			// Remote server: set type and keep URL
			v2Server["type"] = "remote"
			if url, ok := server["url"].(string); ok {
				v2Server["url"] = url
			}
		} else {
			// Local server: set type and merge command + args
			v2Server["type"] = "local"

			// Merge command + args into single command array
			var cmdParts []string
			if cmd, ok := server["command"].(string); ok {
				cmdParts = append(cmdParts, cmd)
			}
			if argsRaw, ok := server["args"]; ok {
				if args, ok := argsRaw.([]any); ok {
					for _, arg := range args {
						if s, ok := arg.(string); ok {
							cmdParts = append(cmdParts, s)
						}
					}
				}
			}
			if len(cmdParts) > 0 {
				// Convert to []any for JSON marshaling
				cmdAny := make([]any, len(cmdParts))
				for i, part := range cmdParts {
					cmdAny[i] = part
				}
				v2Server["command"] = cmdAny
			}
		}

		// Rename env → environment
		if envRaw, ok := server["env"]; ok {
			if env, ok := envRaw.(map[string]any); ok {
				v2Server["environment"] = env
			}
		}

		// Copy other fields (exclude already-processed fields)
		for k, v := range server {
			if k != "command" && k != "args" && k != "env" && k != "url" {
				v2Server[k] = v
			}
		}

		translated[name] = v2Server
	}

	if len(translated) == 0 {
		return nil
	}

	return map[string]any{"servers": translated}
}

// TranslatePermissionsToOpenCode converts IR permissions format to OpenCode V2 format.
// Input:  {"allow": ["Bash(npm test)", "Read"], "deny": ["Bash(rm *)"]}
// Output: [{"action":"shell","resource":"npm test","effect":"allow"}, ...]
func TranslatePermissionsToOpenCode(perms map[string]any) []any {
	if perms == nil || len(perms) == 0 {
		return nil
	}

	var result []any

	// Process allow list
	if allowRaw, ok := perms["allow"]; ok {
		if allow, ok := allowRaw.([]any); ok {
			for _, perm := range allow {
				if s, ok := perm.(string); ok {
					if entry := translatePermEntry(s, "allow"); entry != nil {
						result = append(result, entry)
					}
				}
			}
		}
	}

	// Process deny list
	if denyRaw, ok := perms["deny"]; ok {
		if deny, ok := denyRaw.([]any); ok {
			for _, perm := range deny {
				if s, ok := perm.(string); ok {
					if entry := translatePermEntry(s, "deny"); entry != nil {
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

// translatePermEntry parses a single permission entry and translates it to OpenCode V2 format.
// Handles "Tool(args)" and bare "Tool" formats.
func translatePermEntry(perm string, effect string) map[string]any {
	// Parse Tool(args) format
	openParen := strings.Index(perm, "(")
	if openParen == -1 {
		// Bare tool name
		return map[string]any{
			"action":   mapActionName(perm),
			"resource": "*",
			"effect":   effect,
		}
	}

	// Find matching closing paren (outermost)
	if !strings.HasSuffix(perm, ")") {
		// Malformed — skip
		return nil
	}

	tool := perm[:openParen]
	resource := perm[openParen+1 : len(perm)-1]

	return map[string]any{
		"action":   mapActionName(tool),
		"resource": resource,
		"effect":   effect,
	}
}

// mapActionName maps IR tool names to OpenCode V2 action names.
func mapActionName(tool string) string {
	switch tool {
	case "Bash":
		return "shell"
	case "Read":
		return "read"
	case "Edit", "Write":
		return "edit"
	case "WebSearch":
		return "websearch"
	case "WebFetch":
		return "webfetch"
	case "Agent":
		return "subagent"
	default:
		// MCP tools or unknown — lowercase passthrough
		return strings.ToLower(tool)
	}
}
