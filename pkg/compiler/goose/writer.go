package goose

import (
	"fmt"
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func init() {
	compiler.RegisterWriter("goose", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Goose's native format.
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

	// 3. Settings — result.Settings is already fully assembled (with embedded MCP extensions)
	if rec, ok, err := w.writeSettings(result, profile, outputDir); err != nil {
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

// writeSettings writes the settings file as YAML (Goose uses YAML, not JSON).
func (w *Writer) writeSettings(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string) (manifest.FileRecord, bool, error) {
	if result.Content.SkipSettings || len(result.Content.Settings) == 0 {
		return manifest.FileRecord{}, false, nil
	}
	settingsPath := filepath.Join(outputDir, profile.OutputRoot, profile.SettingsFile)
	hash, err := WriteYAMLFile(settingsPath, result.Content.Settings)
	if err != nil {
		return manifest.FileRecord{}, false, err
	}
	return manifest.FileRecord{
		Output:   filepath.Join(profile.OutputRoot, profile.SettingsFile),
		Strategy: "deep-merge",
		Hash:     hash,
	}, true, nil
}

// WriteYAMLFile marshals data as YAML and writes it to path with a trailing newline.
// Uses atomic write (temp file + rename) to prevent corruption on crash.
// Returns the content hash.
func WriteYAMLFile(path string, data any) (string, error) {
	formatted, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshalling YAML for %s: %w", filepath.Base(path), err)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("creating parent directory for %s: %w", path, err)
	}

	// Write to temp file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".keel-write-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file for %s: %w", path, err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(formatted); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("writing temp file for %s: %w", path, err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("closing temp file for %s: %w", path, err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("renaming temp file to %s: %w", path, err)
	}

	return manifest.HashContent(formatted), nil
}

// TranslateMCPToGoose converts IR MCP format to Goose's extensions format.
// Input:  {"mcpServers": {"name": {"command": "cmd", "args": ["a","b"], "env": {"K":"V"}}}}
// Output: [{"name": "name", "type": "local", "command": ["cmd","a","b"], "environment": {"K":"V"}}]
func TranslateMCPToGoose(mcpData map[string]any) []any {
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

	var extensions []any
	for name, serverRaw := range servers {
		server, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}

		ext := make(map[string]any)
		ext["name"] = name

		// Determine if this is a remote or local server
		_, hasURL := server["url"]
		_, hasCommand := server["command"]
		isRemote := hasURL && !hasCommand

		if isRemote {
			// Remote server: set type and keep URL
			ext["type"] = "remote"
			if url, ok := server["url"].(string); ok {
				ext["url"] = url
			}
		} else {
			// Local server: set type and merge command + args
			ext["type"] = "local"

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
				// Convert to []any for YAML marshaling
				cmdAny := make([]any, len(cmdParts))
				for i, part := range cmdParts {
					cmdAny[i] = part
				}
				ext["command"] = cmdAny
			}
		}

		// Rename env → environment
		if envRaw, ok := server["env"]; ok {
			if env, ok := envRaw.(map[string]any); ok {
				ext["environment"] = env
			}
		}

		// Goose extensions always have enabled: true
		ext["enabled"] = true

		extensions = append(extensions, ext)
	}

	return extensions
}
