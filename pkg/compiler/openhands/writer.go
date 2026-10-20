package openhands

import (
	"encoding/json"
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("openhands", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to OpenHands's native format.
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

	// 3. Settings — TOML output (OpenHands native format)
	if rec, ok, err := compiler.WriteTOMLSettings(result, profile, outputDir); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 4. MCP (separate file)
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

// writeMCP writes the MCP configuration to .openhands/mcp.json.
func (w *Writer) writeMCP(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string) (manifest.FileRecord, bool, error) {
	if len(result.Content.MCP) == 0 {
		return manifest.FileRecord{}, false, nil
	}

	mcpPath := filepath.Join(outputDir, profile.MCPOutput)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0755); err != nil {
		return manifest.FileRecord{}, false, err
	}

	// Write MCP configuration
	data, err := json.MarshalIndent(result.Content.MCP, "", "  ")
	if err != nil {
		return manifest.FileRecord{}, false, err
	}

	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		return manifest.FileRecord{}, false, err
	}

	record := manifest.FileRecord{
		Output:   profile.MCPOutput,
		Strategy: "deep-merge",
		Hash:     manifest.HashContent(data),
	}

	return record, true, nil
}
