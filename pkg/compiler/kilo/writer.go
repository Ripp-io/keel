package kilo

import (
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("kilo", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Kilo Code's native format.
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

// TranslateMCPToKilo converts IR MCP format to Kilo format.
// Input:  {"mcpServers": {"name": {"command": "cmd", "args": ["a","b"], "env": {"K":"V"}}}}
// Output: {"servers": {"name": {"command": ["cmd","a","b"], "environment": {"K":"V"}}}}
func TranslateMCPToKilo(mcpData map[string]any) map[string]any {
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

		kiloServer := make(map[string]any)

		// Determine if this is a remote or local server
		_, hasURL := server["url"]
		_, hasCommand := server["command"]
		isRemote := hasURL && !hasCommand

		if isRemote {
			// Remote server: set type and keep URL
			kiloServer["type"] = "remote"
			if url, ok := server["url"].(string); ok {
				kiloServer["url"] = url
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
				kiloServer["command"] = cmdAny
			}
		}

		// Rename env → environment
		if envRaw, ok := server["env"]; ok {
			if env, ok := envRaw.(map[string]any); ok {
				kiloServer["environment"] = env
			}
		}

		// Copy other fields (exclude already-processed fields)
		for k, v := range server {
			if k != "command" && k != "args" && k != "env" && k != "url" {
				kiloServer[k] = v
			}
		}

		translated[name] = kiloServer
	}

	if len(translated) == 0 {
		return nil
	}

	return map[string]any{"servers": translated}
}

// TranslatePermissionsToKilo converts IR permissions format to Kilo format.
// Input:  {"allow": ["action1", "action2"], "deny": ["action3"]}
// Output: {"allow": ["action1", "action2"], "deny": ["action3"]}
// Note: Kilo's permission format is already close to IR format.
func TranslatePermissionsToKilo(perms map[string]any) map[string]any {
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

	// Process ask list (if it exists from round-tripping)
	if askRaw, ok := perms["ask"]; ok {
		if ask, ok := askRaw.([]any); ok && len(ask) > 0 {
			result["ask"] = ask
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
