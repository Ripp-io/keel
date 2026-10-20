package codex

import (
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("codex", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Codex CLI's native format.
// Output paths are determined by the AgentProfile (scope-dependent).
type Writer struct{}

func (w *Writer) Write(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string, _ compiler.CompileOptions) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord
	agentDir := filepath.Join(outputDir, profile.OutputRoot)

	// 1. Create output directories.
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return nil, err
	}

	// 2. Instructions — AGENTS.md is the native Codex format
	if rec, ok, err := compiler.WriteInstructions(result, profile, outputDir, nil); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 3. Settings — TOML output (Codex native format)
	if rec, ok, err := compiler.WriteTOMLSettings(result, profile, outputDir); err != nil {
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

// TranslateMCPToCodex converts IR MCP format to Codex config.toml format.
// Input:  {"mcpServers": {"name": {"command": "cmd", "args": ["a","b"], "env": {"K":"V"}}}}
// Output: {"servers": {"name": {"command": ["cmd","a","b"], "env": {"K":"V"}}}}
//
// Codex uses a slightly different MCP format than OpenCode:
// - Uses "servers" key directly (no "mcp" wrapper needed in the translation)
// - Command is an array (command + args merged)
// - Uses "env" not "environment"
func TranslateMCPToCodex(mcpData map[string]any) map[string]any {
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

		codexServer := make(map[string]any)

		// Determine if this is a remote or local server
		_, hasURL := server["url"]
		_, hasCommand := server["command"]
		isRemote := hasURL && !hasCommand

		if isRemote {
			// Remote server: keep URL
			if url, ok := server["url"].(string); ok {
				codexServer["url"] = url
			}
		} else {
			// Local server: merge command + args into single command array
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
				codexServer["command"] = cmdAny
			}
		}

		// Keep env as-is (Codex uses "env" not "environment")
		if envRaw, ok := server["env"]; ok {
			if env, ok := envRaw.(map[string]any); ok {
				codexServer["env"] = env
			}
		}

		// Copy other fields (exclude already-processed fields)
		for k, v := range server {
			if k != "command" && k != "args" && k != "env" && k != "url" {
				codexServer[k] = v
			}
		}

		translated[name] = codexServer
	}

	if len(translated) == 0 {
		return nil
	}

	// Return with "servers" key for embedding in settings
	return map[string]any{"servers": translated}
}

// TranslatePermissionsToCodex converts IR permissions format to Codex approval_policy format.
// Input:  {"allow": ["Bash(npm test)", "Read"], "deny": ["Bash(rm *)"]}
// Output: {"auto_approve": ["shell:npm test", "read:*"], "auto_deny": ["shell:rm *"]}
//
// Codex uses approval_policy settings with auto_approve and auto_deny lists.
// Each entry uses the format "action:resource".
func TranslatePermissionsToCodex(perms map[string]any) map[string]any {
	if perms == nil || len(perms) == 0 {
		return nil
	}

	result := make(map[string]any)
	autoApprove := []any{}
	autoDeny := []any{}

	// Process allow list
	if allowRaw, ok := perms["allow"]; ok {
		if allow, ok := allowRaw.([]any); ok {
			for _, perm := range allow {
				if s, ok := perm.(string); ok {
					if entry := translatePermEntryToCodex(s); entry != "" {
						autoApprove = append(autoApprove, entry)
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
					if entry := translatePermEntryToCodex(s); entry != "" {
						autoDeny = append(autoDeny, entry)
					}
				}
			}
		}
	}

	if len(autoApprove) > 0 {
		result["auto_approve"] = autoApprove
	}
	if len(autoDeny) > 0 {
		result["auto_deny"] = autoDeny
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

// translatePermEntryToCodex parses a single permission entry and translates it to Codex format.
// Handles "Tool(args)" and bare "Tool" formats.
// Returns format: "action:resource"
func translatePermEntryToCodex(perm string) string {
	// Parse Tool(args) format
	openParen := strings.Index(perm, "(")
	if openParen == -1 {
		// Bare tool name
		return mapActionNameForCodex(perm) + ":*"
	}

	// Find matching closing paren
	if !strings.HasSuffix(perm, ")") {
		// Malformed — skip
		return ""
	}

	tool := perm[:openParen]
	resource := perm[openParen+1 : len(perm)-1]

	return mapActionNameForCodex(tool) + ":" + resource
}

// mapActionNameForCodex maps IR tool names to Codex action names.
func mapActionNameForCodex(tool string) string {
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
