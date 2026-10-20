package vibe

import (
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("vibe", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Mistral Vibe's native format.
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

	// 3. Settings — TOML output (Vibe native format)
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

// TranslateMCPToVibe converts IR MCP format to Vibe's mcp_servers array format.
// Input:  {"mcpServers": {"name": {"command": "cmd", "args": ["a","b"], "env": {"K":"V"}}}}
// Output: [{"name": "name", "transport": "stdio", "command": "cmd", "args": ["a","b"], "env": {"K":"V"}}]
func TranslateMCPToVibe(mcpData map[string]any) []any {
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

	var result []any

	for name, serverRaw := range servers {
		server, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}

		vibeServer := make(map[string]any)
		vibeServer["name"] = name

		// Determine transport type
		_, hasURL := server["url"]
		_, hasCommand := server["command"]

		if hasURL && !hasCommand {
			// Remote server (HTTP or streamable-http)
			vibeServer["transport"] = "http"
			if url, ok := server["url"].(string); ok {
				vibeServer["url"] = url
			}

			// Copy auth config if present
			if auth, ok := server["auth"]; ok {
				vibeServer["auth"] = auth
			}
		} else {
			// Local server (stdio)
			vibeServer["transport"] = "stdio"

			if cmd, ok := server["command"].(string); ok {
				vibeServer["command"] = cmd
			}

			if argsRaw, ok := server["args"]; ok {
				if args, ok := argsRaw.([]any); ok {
					vibeServer["args"] = args
				}
			}

			if envRaw, ok := server["env"]; ok {
				if env, ok := envRaw.(map[string]any); ok {
					vibeServer["env"] = env
				}
			}
		}

		// Copy other fields (timeout settings, etc.)
		for k, v := range server {
			if k != "command" && k != "args" && k != "env" && k != "url" && k != "auth" {
				vibeServer[k] = v
			}
		}

		result = append(result, vibeServer)
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
