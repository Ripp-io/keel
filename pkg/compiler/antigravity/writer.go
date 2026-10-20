package antigravity

import (
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func init() {
	compiler.RegisterWriter("antigravity", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Antigravity CLI's native format.
// Output paths are determined by the AgentProfile (scope-dependent).
type Writer struct{}

func (w *Writer) Write(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string, opts compiler.CompileOptions) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord

	// 1. Create output directories
	agentDir := filepath.Join(outputDir, profile.OutputRoot)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return nil, err
	}

	// Settings directory: .gemini/antigravity-cli/
	settingsDir := filepath.Join(agentDir, "antigravity-cli")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return nil, err
	}

	// For project scope, create .agents/ directory
	// For user scope, create .gemini/config/ directory (for MCP)
	if opts.Scope == compiler.ScopeProject {
		if err := os.MkdirAll(filepath.Join(outputDir, ".agents"), 0755); err != nil {
			return nil, err
		}
	} else {
		if err := os.MkdirAll(filepath.Join(agentDir, "config"), 0755); err != nil {
			return nil, err
		}
	}

	// 2. Instructions
	if rec, ok, err := compiler.WriteInstructions(result, profile, outputDir, nil); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 3. Settings — write to .gemini/antigravity-cli/settings.json
	if rec, ok, err := w.writeSettings(result, profile, outputDir); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 4. MCP — write to separate file (.agents/mcp_config.json or .gemini/config/mcp_config.json)
	if rec, ok, err := w.writeMCP(result, profile, outputDir, opts); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 5. Categories — Antigravity uses .agents/ instead of .gemini/ for categories
	categoryRecords, err := w.writeCategories(result, profile, outputDir, opts)
	if err != nil {
		return nil, err
	}
	records = append(records, categoryRecords...)

	// 6. Bin files
	binRecords, err := compiler.WriteBinFiles(result.Content.Bin, outputDir)
	if err != nil {
		return nil, err
	}
	records = append(records, binRecords...)

	// 7. Hook scripts
	hookRecords, err := compiler.WriteHookScripts(result.Content.HookScripts, outputDir)
	if err != nil {
		return nil, err
	}
	records = append(records, hookRecords...)

	return records, nil
}

// writeSettings writes the settings file to .gemini/antigravity-cli/settings.json.
func (w *Writer) writeSettings(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string) (manifest.FileRecord, bool, error) {
	if result.Content.SkipSettings || len(result.Content.Settings) == 0 {
		return manifest.FileRecord{}, false, nil
	}

	// Settings go in .gemini/antigravity-cli/settings.json
	settingsPath := filepath.Join(outputDir, profile.OutputRoot, "antigravity-cli", profile.SettingsFile)

	hash, err := compiler.WriteJSONFileWithHash(settingsPath, result.Content.Settings)
	if err != nil {
		return manifest.FileRecord{}, false, err
	}

	return manifest.FileRecord{
		Output:   filepath.Join(profile.OutputRoot, "antigravity-cli", profile.SettingsFile),
		Strategy: "deep-merge",
		Hash:     hash,
	}, true, nil
}

// writeMCP writes the MCP config file to the appropriate location based on scope.
func (w *Writer) writeMCP(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string, opts compiler.CompileOptions) (manifest.FileRecord, bool, error) {
	if len(result.Content.MCP) == 0 {
		return manifest.FileRecord{}, false, nil
	}

	// MCP path depends on scope
	var mcpPath string
	var outputPath string
	if opts.Scope == compiler.ScopeProject {
		// Project: .agents/mcp_config.json
		mcpPath = filepath.Join(outputDir, ".agents", "mcp_config.json")
		outputPath = ".agents/mcp_config.json"
	} else {
		// User: .gemini/config/mcp_config.json
		mcpPath = filepath.Join(outputDir, profile.OutputRoot, "config", "mcp_config.json")
		outputPath = filepath.Join(profile.OutputRoot, "config", "mcp_config.json")
	}

	if err := os.MkdirAll(filepath.Dir(mcpPath), 0755); err != nil {
		return manifest.FileRecord{}, false, err
	}

	hash, err := compiler.WriteJSONFileWithHash(mcpPath, result.Content.MCP)
	if err != nil {
		return manifest.FileRecord{}, false, err
	}

	return manifest.FileRecord{
		Output:   outputPath,
		Strategy: "deep-merge",
		Hash:     hash,
	}, true, nil
}

// writeCategories writes category files to .agents/ (project) or .gemini/ (user).
// Antigravity uses .agents/ for categories in project scope.
func (w *Writer) writeCategories(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string, opts compiler.CompileOptions) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord

	// Determine category base directory based on scope
	var categoryBase string
	var categoryPrefix string
	if opts.Scope == compiler.ScopeProject {
		// Project scope: .agents/
		categoryBase = filepath.Join(outputDir, ".agents")
		categoryPrefix = ".agents"
	} else {
		// User scope: .gemini/ (following user profile pattern)
		categoryBase = filepath.Join(outputDir, profile.OutputRoot)
		categoryPrefix = profile.OutputRoot
	}

	// Write categories
	for _, cat := range profile.Categories {
		files, ok := result.Content.Categories[cat.DirName]
		if !ok || len(files) == 0 {
			continue
		}
		catRecords, err := compiler.WriteCategory(categoryBase, cat.DirName, files)
		if err != nil {
			return nil, err
		}
		// Prefix each record with the category base
		for i := range catRecords {
			catRecords[i].Output = filepath.Join(categoryPrefix, catRecords[i].Output)
		}
		records = append(records, catRecords...)
	}

	return records, nil
}
