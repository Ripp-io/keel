package kiro

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

func init() {
	compiler.RegisterWriter("kiro", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Kiro's native format.
// Output paths are determined by the AgentProfile (scope-dependent).
type Writer struct{}

func (w *Writer) Write(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string, _ compiler.CompileOptions) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord
	agentDir := filepath.Join(outputDir, profile.OutputRoot)
	settingsDir := filepath.Join(agentDir, "settings")

	// 1. Create output directories.
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return nil, err
	}

	// 2. Instructions
	if rec, ok, err := compiler.WriteInstructions(result, profile, outputDir, nil); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 3. Settings
	if rec, ok, err := compiler.WriteSettings(result, profile, outputDir); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 4. MCP — write to separate file
	if rec, ok, err := w.writeMCP(result, profile, outputDir); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 5. Common output (categories, bin)
	commonRecords, err := compiler.WriteCommonOutput(result, profile, outputDir)
	if err != nil {
		return nil, err
	}
	records = append(records, commonRecords...)

	return records, nil
}

// writeMCP writes the MCP configuration to .kiro/settings/mcp.json.
func (w *Writer) writeMCP(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string) (manifest.FileRecord, bool, error) {
	if len(result.Content.MCP) == 0 {
		return manifest.FileRecord{}, false, nil
	}

	mcpPath := filepath.Join(outputDir, profile.MCPOutput)

	// Read existing MCP file if present
	existing, err := compiler.ReadJSONConfig([]string{mcpPath})
	if err != nil {
		return manifest.FileRecord{}, false, err
	}

	// Deep-merge new MCP data with existing
	var merged map[string]any
	if existing != nil {
		merged = merge.DeepMergeMapsWithPolicies(result.Content.MCP, existing, nil)
	} else {
		merged = result.Content.MCP
	}

	// Write MCP file
	if err := compiler.WriteJSONFile(mcpPath, merged); err != nil {
		return manifest.FileRecord{}, false, err
	}

	// Serialize merged content for hash
	content, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return manifest.FileRecord{}, false, err
	}

	record := manifest.FileRecord{
		Output:   profile.MCPOutput,
		Strategy: "deep-merge",
		Hash:     manifest.HashContent(content),
	}

	return record, true, nil
}

// TranslatePermissionsToKiro converts IR permissions format to Kiro format.
// Input:  {"allow": ["Bash(npm test)", "Read"], "deny": ["Bash(rm *)"]}
// Output: [{"tool":"Bash","resource":"npm test","effect":"allow"}, ...]
func TranslatePermissionsToKiro(perms map[string]any) []any {
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

// translatePermEntry parses a single permission entry and translates it to Kiro format.
// Handles "Tool(args)" and bare "Tool" formats.
func translatePermEntry(perm string, effect string) map[string]any {
	// Parse Tool(args) format
	openParen := strings.Index(perm, "(")
	if openParen == -1 {
		// Bare tool name
		return map[string]any{
			"tool":     perm,
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
		"tool":     tool,
		"resource": resource,
		"effect":   effect,
	}
}
