package claude

import (
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("claude", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Claude Code's native format.
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

	// 3. Settings — result.Settings is already fully assembled
	if rec, ok, err := compiler.WriteSettings(result, profile, outputDir); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 4. MCP — always write per-agent MCP (the per-agent accumulation has
	// agent conditionals correctly evaluated, so this includes both universal
	// and agent-specific servers). If the shared phase also wrote .mcp.json,
	// this overwrites it with the correctly filtered content.
	if profile.MCPOutput != "" && len(result.Content.MCP) > 0 {
		mcpPath := filepath.Join(outputDir, profile.MCPOutput)
		if err := os.MkdirAll(filepath.Dir(mcpPath), 0755); err != nil {
			return nil, err
		}
		var hash string
		var err error
		strategy := "deep-merge"
		if profile.MCPMergeInto {
			// Shared file (e.g., ~/.claude.json) — merge only mcpServers key.
			if err := compiler.MergeJSONFileKey(mcpPath, result.Content.MCP, "mcpServers"); err != nil {
				return nil, err
			}
			// Read back to hash the final content (merge-into modifies existing file).
			mcpBytes, err := os.ReadFile(mcpPath)
			if err != nil {
				return nil, err
			}
			hash = manifest.HashContent(mcpBytes)
			strategy = "merge-into"
		} else {
			hash, err = compiler.WriteJSONFileWithHash(mcpPath, result.Content.MCP)
			if err != nil {
				return nil, err
			}
		}
		records = append(records, manifest.FileRecord{
			Output:   profile.MCPOutput,
			Strategy: strategy,
			Hash:     hash,
		})
	}

	// 5-9. Common output (categories, bin, content blocks, config blocks, state)
	commonRecords, err := compiler.WriteCommonOutput(result, profile, outputDir)
	if err != nil {
		return nil, err
	}
	records = append(records, commonRecords...)

	return records, nil
}
