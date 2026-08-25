package compiler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
)

// WriteCategory creates a category directory under outputDir and writes
// each CategoryFile into it. Directory entries are created with MkdirAll;
// file entries are written with their Content bytes.
func WriteCategory(outputDir, categoryName string, files []CategoryFile) ([]manifest.FileRecord, error) {
	catDir := filepath.Join(outputDir, categoryName)
	if err := os.MkdirAll(catDir, 0755); err != nil {
		return nil, fmt.Errorf("creating category directory %s: %w", categoryName, err)
	}

	var records []manifest.FileRecord
	for _, f := range files {
		target := filepath.Join(catDir, f.RelPath)
		// Path containment check — defense-in-depth against traversal via crafted RelPath
		if !pathutil.ContainsPath(catDir, target) {
			return nil, fmt.Errorf("category file %q escapes output directory", f.RelPath)
		}
		if f.IsDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return nil, fmt.Errorf("creating subdirectory %s: %w", f.RelPath, err)
			}
			continue
		}
		// Ensure parent directories exist for nested files.
		if parent := filepath.Dir(target); parent != catDir {
			if err := os.MkdirAll(parent, 0755); err != nil {
				return nil, fmt.Errorf("creating parent directory for %s: %w", f.RelPath, err)
			}
		}
		if err := os.WriteFile(target, f.Content, 0644); err != nil {
			return nil, fmt.Errorf("writing file %s: %w", f.RelPath, err)
		}
		// Per-file record with content hash for drift detection
		records = append(records, manifest.FileRecord{
			Output:   filepath.Join(categoryName, f.RelPath),
			Strategy: "overlay",
			Hash:     manifest.HashContent(f.Content),
		})
	}
	return records, nil
}

// WriteJSONFile marshals data as indented JSON and writes it to path
// with a trailing newline. Uses atomic write (temp file + rename) to
// prevent corruption on crash.
func WriteJSONFile(path string, data any) error {
	_, err := WriteJSONFileWithHash(path, data)
	return err
}

// WriteJSONFileWithHash writes a JSON file and returns the content hash.
// This avoids the read-back pattern where callers write then immediately
// re-read just to compute a hash.
func WriteJSONFileWithHash(path string, data any) (string, error) {
	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling JSON for %s: %w", filepath.Base(path), err)
	}
	formatted = append(formatted, '\n')

	// Write to temp file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".keel-write-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file for %s: %w", path, err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to clean up temp file", "path", tmpPath, "error", err)
		}
	}()

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

// WriteBinFiles writes executable files to .keel/bin/ under outputDir.
// Returns a FileRecord for each file written with its content hash.
// Shared by both Claude and Cursor writers.
func WriteBinFiles(bins []BinFile, outputDir string) ([]manifest.FileRecord, error) {
	if len(bins) == 0 {
		return nil, nil
	}

	var records []manifest.FileRecord
	binDir := filepath.Join(outputDir, ".keel", "bin")

	for _, bf := range bins {
		dst := filepath.Join(binDir, bf.RelPath)
		if !pathutil.ContainsPath(binDir, dst) {
			return nil, fmt.Errorf("bin file %q escapes output directory", bf.RelPath)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return nil, err
		}
		// Bin files are always written with 0755 (executable) permissions,
		// matching the old pipeline's copyBinFile behavior.
		if err := os.WriteFile(dst, bf.Content, 0755); err != nil {
			return nil, err
		}

		// Create a FileRecord for each individual bin file with its hash.
		outputPath := filepath.Join(".keel", "bin", bf.RelPath)
		records = append(records, manifest.FileRecord{
			Output:   outputPath,
			Strategy: "overlay",
			Hash:     manifest.HashContent(bf.Content),
		})
	}

	return records, nil
}

// WriteHookScripts writes hook script files to .keel/hooks/ under outputDir.
// Same pattern as WriteBinFiles but in a different output directory.
func WriteHookScripts(scripts []BinFile, outputDir string) ([]manifest.FileRecord, error) {
	if len(scripts) == 0 {
		return nil, nil
	}

	var records []manifest.FileRecord
	hooksDir := filepath.Join(outputDir, ".keel", "hooks")

	for _, sf := range scripts {
		dst := filepath.Join(hooksDir, sf.RelPath)
		if !pathutil.ContainsPath(hooksDir, dst) {
			return nil, fmt.Errorf("hook script %q escapes output directory", sf.RelPath)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return nil, err
		}
		mode := sf.Mode
		if mode == 0 {
			mode = 0755
		}
		if err := os.WriteFile(dst, sf.Content, mode); err != nil {
			return nil, err
		}

		outputPath := filepath.Join(".keel", "hooks", sf.RelPath)
		records = append(records, manifest.FileRecord{
			Output:   outputPath,
			Strategy: "overlay",
			Hash:     manifest.HashContent(sf.Content),
		})
	}

	return records, nil
}

// WriteStateDir creates the .keel/state/ directory with a .gitignore.
// Shared by both Claude and Cursor writers.
func WriteStateDir(outputDir string) error {
	stateDir := filepath.Join(outputDir, ".keel", "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, ".gitignore"), []byte("*\n"), 0644)
}

// MergeJSONFileKey reads an existing JSON file, deep-merges a single top-level
// key from newData into it, and writes the result back. If the file does not
// exist, it writes newData as-is. Other keys in the existing file are preserved.
//
// This is used for shared/multi-purpose files like ~/.claude.json where Keel
// manages only the mcpServers key while preserving OAuth tokens and other content.
//
// Uses atomic write (temp file + rename) to prevent corruption on crash.
func MergeJSONFileKey(path string, newData map[string]any, mergeKey string) error {
	existing, err := pathutil.SafeReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist — write newData as-is.
			return WriteJSONFile(path, newData)
		}
		return fmt.Errorf("reading existing file %q: %w", path, err)
	}

	var existingData map[string]any
	if err := json.Unmarshal(existing, &existingData); err != nil {
		// File exists but isn't valid JSON — don't corrupt it.
		return fmt.Errorf("parsing existing file %q: %w", path, err)
	}

	// Deep-merge only the specified key.
	if newServers, ok := newData[mergeKey]; ok {
		if newMap, ok := newServers.(map[string]any); ok {
			existingServers, _ := existingData[mergeKey].(map[string]any)
			if existingServers == nil {
				existingServers = make(map[string]any)
			}
			for k, v := range newMap {
				existingServers[k] = v
			}
			existingData[mergeKey] = existingServers
		}
	}

	return WriteJSONFile(path, existingData)
}

// WriteCommonOutput handles the shared output writing pattern for all agent writers.
// Agent-specific concerns (settings assembly, instruction wrapping, sandbox/hooks placement)
// are handled by the caller before and after this function.
//
// This function writes:
// - Categories (from profile)
// - Dot-directory passthrough files
// - Bin files
// - Content blocks (loop, worktree-include)
// - Config blocks (environment)
// - State directory
//
// It returns manifest records for all written files.
func WriteCommonOutput(result *AccumulatedResult, profile AgentProfile, outputDir string) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord
	agentDir := filepath.Join(outputDir, profile.OutputRoot)

	// 1. Categories
	for _, cat := range profile.Categories {
		files, ok := result.Content.Categories[cat.DirName]
		if !ok || len(files) == 0 {
			continue
		}
		catRecords, err := WriteCategory(agentDir, cat.DirName, files)
		if err != nil {
			return nil, err
		}
		// Prefix each record with the output root.
		for i := range catRecords {
			catRecords[i].Output = filepath.Join(profile.OutputRoot, catRecords[i].Output)
		}
		records = append(records, catRecords...)
	}

	// 2. Dot-directory passthrough files (arbitrary files from native layers).
	if dotdirFiles, ok := result.Content.Categories[CategoryDotDir]; ok && len(dotdirFiles) > 0 {
		dotRecords, err := WriteCategory(agentDir, "", dotdirFiles)
		if err != nil {
			return nil, err
		}
		// Prefix each record with the output root.
		for i := range dotRecords {
			dotRecords[i].Output = filepath.Join(profile.OutputRoot, dotRecords[i].Output)
		}
		records = append(records, dotRecords...)
	}

	// 3. Bin files
	binRecords, err := WriteBinFiles(result.Content.Bin, outputDir)
	if err != nil {
		return nil, err
	}
	records = append(records, binRecords...)

	// 3b. Hook scripts — deployed to .keel/hooks/ so hook commands can
	// reference them portably via ${keel:keel.hooks.dir}.
	hookRecords, err := WriteHookScripts(result.Content.HookScripts, outputDir)
	if err != nil {
		return nil, err
	}
	records = append(records, hookRecords...)

	// 4. Content blocks (loop, worktree-include)
	for _, cb := range profile.ContentOutputs {
		content, ok := result.Content.ContentBlocks[cb.Name]
		if !ok || content == "" {
			continue
		}
		blockPath := filepath.Join(outputDir, cb.OutputPath)
		if err := os.MkdirAll(filepath.Dir(blockPath), 0755); err != nil {
			return nil, err
		}
		contentBytes := []byte(content + "\n")
		if err := os.WriteFile(blockPath, contentBytes, 0644); err != nil {
			return nil, err
		}
		records = append(records, manifest.FileRecord{
			Output:   cb.OutputPath,
			Strategy: "concatenate",
			Hash:     manifest.HashContent(contentBytes),
		})
	}

	// 5. Config blocks (environment)
	for _, cb := range profile.ConfigOutputs {
		config, ok := result.Content.ConfigBlocks[cb.Name]
		if !ok || len(config) == 0 {
			continue
		}
		configPath := filepath.Join(outputDir, cb.OutputPath)
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			return nil, err
		}
		hash, err := WriteJSONFileWithHash(configPath, config)
		if err != nil {
			return nil, err
		}
		records = append(records, manifest.FileRecord{
			Output:   cb.OutputPath,
			Strategy: "deep-merge",
			Hash:     hash,
		})
	}

	// 6. State directory
	if err := WriteStateDir(outputDir); err != nil {
		return nil, err
	}

	return records, nil
}

// WriteInstructions writes the instruction file with optional content wrapping.
// The wrapper function transforms the content before writing (e.g., Cursor's MDC wrapping).
// If wrapper is nil, content is written as-is.
// Returns (FileRecord, written bool, error). written is false if instructions are skipped.
func WriteInstructions(result *AccumulatedResult, profile AgentProfile, outputDir string, wrapper func(string) string) (manifest.FileRecord, bool, error) {
	if profile.InstructionOutput == "" || result.Content.Instructions == "" || result.Content.SkipInstructions {
		return manifest.FileRecord{}, false, nil
	}
	instrPath := filepath.Join(outputDir, profile.InstructionOutput)
	if err := os.MkdirAll(filepath.Dir(instrPath), 0755); err != nil {
		return manifest.FileRecord{}, false, err
	}
	content := result.Content.Instructions
	if wrapper != nil {
		content = wrapper(content)
	}
	contentBytes := []byte(content)
	if err := os.WriteFile(instrPath, contentBytes, 0644); err != nil {
		return manifest.FileRecord{}, false, err
	}
	return manifest.FileRecord{
		Output:   profile.InstructionOutput,
		Strategy: "user-editable",
		Hash:     manifest.HashContent(contentBytes),
	}, true, nil
}

// WriteSettings writes the settings file using the pre-assembled settings map.
// Returns (FileRecord, written bool, error). written is false if settings are skipped.
func WriteSettings(result *AccumulatedResult, profile AgentProfile, outputDir string) (manifest.FileRecord, bool, error) {
	if result.Content.SkipSettings || len(result.Content.Settings) == 0 {
		return manifest.FileRecord{}, false, nil
	}
	settingsPath := filepath.Join(outputDir, profile.OutputRoot, profile.SettingsFile)
	hash, err := WriteJSONFileWithHash(settingsPath, result.Content.Settings)
	if err != nil {
		return manifest.FileRecord{}, false, err
	}
	return manifest.FileRecord{
		Output:   filepath.Join(profile.OutputRoot, profile.SettingsFile),
		Strategy: "deep-merge",
		Hash:     hash,
	}, true, nil
}

// SimpleWriter is a Writer for instructions-only agents (aider, cline, copilot).
// It writes instructions (with optional content wrapping) and common output
// (categories, bin, content blocks, config blocks, state directory).
// Agents with richer native formats (claude, cursor) implement Writer directly.
type SimpleWriter struct {
	// InstructionWrapper transforms instruction content before writing.
	// If nil, content is written as-is. Used by adapters that need
	// format-specific wrapping (e.g., Cursor's MDC frontmatter).
	InstructionWrapper func(string) string
}

func (w *SimpleWriter) Write(result *AccumulatedResult, profile AgentProfile, outputDir string, _ CompileOptions) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord

	// Create output directory for .keel.json manifest.
	agentDir := filepath.Join(outputDir, profile.OutputRoot)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return nil, err
	}

	// 1. Instructions
	if rec, ok, err := WriteInstructions(result, profile, outputDir, w.InstructionWrapper); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 2. Common output (categories, bin, content blocks, config blocks, state)
	commonRecords, err := WriteCommonOutput(result, profile, outputDir)
	if err != nil {
		return nil, err
	}
	records = append(records, commonRecords...)

	return records, nil
}
