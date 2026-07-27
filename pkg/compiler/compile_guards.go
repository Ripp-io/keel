package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"log/slog"
)

// detectInstructionConflicts checks if the instruction output file exists
// on disk and was user-edited (not written by keel, or written by keel but
// modified since). If so, sets accumulated.SkipInstructions to prevent
// overwriting user changes. Logs a warning when conflicts are detected.
func detectInstructionConflicts(accumulated *AccumulatedResult, oldManifest *manifest.CompiledManifest, profile AgentProfile, outputDir string, forceInstructions bool) {
	if profile.InstructionOutput == "" || accumulated.Content.Instructions == "" || forceInstructions {
		return
	}

	instrPath := filepath.Join(outputDir, profile.InstructionOutput)
	existingData, err := pathutil.SafeReadFile(instrPath)
	if err != nil {
		return // File doesn't exist, no conflict
	}

	diskHash := manifest.HashContent(existingData)

	// Check manifest for what keel last wrote — if the disk hash matches
	// the manifest hash, keel wrote it and the user hasn't touched it.
	keelOwned := false
	if oldManifest != nil {
		for _, f := range oldManifest.FileOps {
			if f.Output == profile.InstructionOutput && f.Hash != "" {
				if f.Hash == diskHash {
					keelOwned = true
				}
				break
			}
		}
	}

	if !keelOwned {
		accumulated.Content.SkipInstructions = true
		if oldManifest == nil {
			slog.Warn("existing instruction file found, skipping (use --force to overwrite)",
				"path", profile.InstructionOutput)
		} else {
			slog.Warn("instruction file modified since last compile, skipping (use --force to overwrite)",
				"path", profile.InstructionOutput)
		}
	}
}

// detectFirstCompileConflicts checks for pre-existing files at Keel-managed
// output paths when no manifest exists (first compile). Returns an error
// listing all conflicting files if any exist. Instruction files are excluded
// (they have separate conflict handling). Skipped when --force is set.
func detectFirstCompileConflicts(accumulated *AccumulatedResult, profile AgentProfile, outputDir string, opts CompileOptions) error {
	// Build set of paths written by the shared phase — these are ours, not user files.
	sharedPaths := make(map[string]bool)
	if opts.SharedResult != nil {
		for _, r := range opts.SharedResult.FileRecords {
			sharedPaths[r.Output] = true
		}
	}

	var conflicts []string

	// 1. Settings file
	if profile.SettingsFile != "" && !accumulated.Content.SkipSettings && len(accumulated.Content.Settings) > 0 {
		settingsPath := filepath.Join(outputDir, profile.OutputRoot, profile.SettingsFile)
		if _, err := os.Stat(settingsPath); err == nil {
			relPath := filepath.Join(profile.OutputRoot, profile.SettingsFile)
			conflicts = append(conflicts, relPath)
		}
	}

	// 2. MCP config (only if not merge-into, and not already written by shared phase)
	if len(accumulated.Content.MCP) > 0 && !profile.MCPMergeInto && profile.MCPOutput != "" && !sharedPaths[profile.MCPOutput] {
		mcpPath := filepath.Join(outputDir, profile.MCPOutput)
		if _, err := os.Stat(mcpPath); err == nil {
			conflicts = append(conflicts, profile.MCPOutput)
		}
	}

	// 3. Category files
	for _, cat := range profile.Categories {
		files, ok := accumulated.Content.Categories[cat.DirName]
		if !ok || len(files) == 0 {
			continue
		}
		for _, file := range files {
			if file.IsDir {
				continue
			}
			filePath := filepath.Join(outputDir, profile.OutputRoot, cat.DirName, file.RelPath)
			if _, err := os.Stat(filePath); err == nil {
				relPath := filepath.Join(profile.OutputRoot, cat.DirName, file.RelPath)
				conflicts = append(conflicts, relPath)
			}
		}
	}

	// 4. Content blocks
	for _, cb := range profile.ContentOutputs {
		content, ok := accumulated.Content.ContentBlocks[cb.Name]
		if !ok || content == "" {
			continue
		}
		blockPath := filepath.Join(outputDir, cb.OutputPath)
		if _, err := os.Stat(blockPath); err == nil {
			conflicts = append(conflicts, cb.OutputPath)
		}
	}

	// 5. Config blocks
	for _, cb := range profile.ConfigOutputs {
		config, ok := accumulated.Content.ConfigBlocks[cb.Name]
		if !ok || len(config) == 0 {
			continue
		}
		configPath := filepath.Join(outputDir, cb.OutputPath)
		if _, err := os.Stat(configPath); err == nil {
			conflicts = append(conflicts, cb.OutputPath)
		}
	}

	// 6. Sandbox (Cursor-specific separate file)
	if profile.SandboxOutput != "" && accumulated.Content.Sandbox != nil {
		sandboxPath := filepath.Join(outputDir, profile.SandboxOutput)
		if _, err := os.Stat(sandboxPath); err == nil {
			conflicts = append(conflicts, profile.SandboxOutput)
		}
	}

	// If any conflicts found, return error with sorted list
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		var msg strings.Builder
		msg.WriteString("first compile would overwrite existing files:\n")
		for _, path := range conflicts {
			msg.WriteString("  ")
			msg.WriteString(path)
			msg.WriteString("\n")
		}
		msg.WriteString("\nMove files to .keel/")
		msg.WriteString(profile.OutputRoot)
		msg.WriteString("/ to include as an implicit layer, or use --force to overwrite")
		return fmt.Errorf("%s", msg.String())
	}

	return nil
}

// evaluateSettingsSkip suppresses settings output for agents with separate
// settings files when no settings/permissions/agentSpecific data was
// accumulated. This prevents writing a settings file that contains only
// boilerplate (e.g. version).
//
// Called after AssembleAgentOutput, so Settings already contains agent defaults,
// agent-specific data, permissions, attribution, etc. For agents like Cursor
// with SettingsDefaults, Settings is always non-empty, so settings are always
// written. For Claude with no defaults, Settings is empty only when no meaningful
// data was accumulated.
func evaluateSettingsSkip(accumulated *AccumulatedResult, profile AgentProfile) {
	if profile.HooksInSettings {
		return // Claude embeds everything in settings, never skip
	}

	if len(accumulated.Content.Settings) == 0 {
		accumulated.Content.SkipSettings = true
	}
}
