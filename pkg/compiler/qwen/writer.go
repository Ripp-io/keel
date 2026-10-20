package qwen

import (
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("qwen", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Qwen Code's native format.
// Output paths are determined by the AgentProfile (scope-dependent).
type Writer struct{}

func (w *Writer) Write(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string, _ compiler.CompileOptions) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord
	agentDir := filepath.Join(outputDir, profile.OutputRoot)

	// 1. Create output directories.
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return nil, err
	}

	// 2. Instructions (QWEN.md in single-agent mode, shim in multi-agent mode)
	if rec, ok, err := compiler.WriteInstructions(result, profile, outputDir, nil); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 3. Settings — result.Settings is already fully assembled (with embedded MCP)
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
