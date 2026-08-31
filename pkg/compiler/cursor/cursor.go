// Package cursor provides the Cursor adapter for the Keel compiler.
// It registers a compiler, native reader, and writer for the "cursor" agent
// format via init() functions that call the parent compiler package's
// registration functions.
package cursor

import (
	"fmt"
	"strings"
	"unicode"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "cursor", Files: []string{".cursorrules"}, Dirs: []string{".cursor"}})
	compiler.RegisterCompiler("cursor", Compile)
	compiler.RegisterProfileFunc("cursor", Profile)
}

// Compile produces Cursor output:
//
//	.cursor/rules/00-keel.mdc   concatenated instructions with .mdc frontmatter
//	.cursor/cli-config.json     deep-merged CLI config
//	.cursor/mcp.json            MCP config
//	.cursor/hooks.json          hooks config
//	.cursor/rules/              overlaid rule files
//	.cursor/commands/           overlaid command files
//	.cursor/skills/             overlaid skill files
//	.cursor/agents/             overlaid agent files
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Cursor.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Cursor.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.InstructionOutput = "" // Cursor doesn't read user-level rules
	p.Categories = compiler.FilterOut(p.Categories, func(c compiler.CategoryDef) string { return c.DirName }, "rules")
	// environment is project-only — remove it from ConfigOutputs
	p.ConfigOutputs = compiler.FilterOut(p.ConfigOutputs, func(b compiler.ConfigBlockDef) string { return b.Name }, "environment")
	return p
}

// ProjectProfile returns the project-level AgentProfile for Cursor.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Plugins = false     // plugins are Claude-only
	caps.AgentMemory = false // agent-memory is Claude-only
	caps.Rules = true

	return compiler.AgentProfile{
		Name:              "cursor",
		OutputRoot:        ".cursor",
		InstructionOutput: ".cursor/rules/00-keel.mdc",
		SharedInstructionOutput: "AGENTS.md",
		SharedMCPOutput:         ".mcp.json",
		// Cursor shim needs MDC wrapping — deferred until shared writer phase.
		InstructionShimTemplate: "",
		// Cursor documents both "cli.json" and "cli-config.json" in different
		// contexts. In practice, the CLI agent reads "cli-config.json" from
		// the .cursor/ directory. If a future Cursor update changes this,
		// the filename should be updated here.
		SettingsFile:       "cli-config.json",
		SettingsLockKey:    "cli-config.json",
		SettingsCandidates: []string{"cli-config.json", "settings.cursor.json", "cli.json"},
		MCPOutput:          ".cursor/mcp.json",
		SandboxOutput:      ".cursor/sandbox.json",
		HooksInSettings:    false,
		Categories: append(compiler.BaseCategories(),
			compiler.CategoryDef{DirName: "rules", LockCategory: "rule"},
		),
		InstructionCandidates: []string{
			"instructions.md",
			"INSTRUCTIONS.md",
			"instructions.mdc",
			"AGENTS.md",
		},
		HookKeyTransform:      HookKeyTransform,
		ToolNameTransform:     compiler.IdentityTransform(), // Claude and Cursor share tool names
		MCPNativeLockCategory: "mcp-file",
		ContentOutputs:        []compiler.ContentBlockDef{},
		ConfigOutputs: []compiler.ConfigBlockDef{
			{Name: "environment", OutputPath: ".cursor/environment.json"},
		},
		Capabilities:     caps,
		SettingsDefaults: map[string]any{"version": float64(1)},
		SettingsAssembler: func(settings map[string]any, result *compiler.AccumulatedResult) {
			// Permissions: flat keys with (*) suffix
			if len(result.Content.Permissions) > 0 {
				if allowRaw, ok := result.Content.Permissions["allow"]; ok {
					if translated := translatePermissionSlice(allowRaw); len(translated) > 0 {
						settings["permissions.allow"] = translated
					}
				}
				if denyRaw, ok := result.Content.Permissions["deny"]; ok {
					if translated := translatePermissionSlice(denyRaw); len(translated) > 0 {
						settings["permissions.deny"] = translated
					}
				}
			}

			// Attribution: boolean flags
			if result.Content.Attribution != nil {
				attr := make(map[string]any)
				if result.Content.Attribution.CommitEnabled {
					attr["attributeCommitsToAgent"] = true
				}
				if result.Content.Attribution.PREnabled {
					attr["attributePRsToAgent"] = true
				}
				if len(attr) > 0 {
					settings["attribution"] = attr
				}
			}
		},
	}
}

// translatePermissionSlice converts a permission slice to Cursor format.
func translatePermissionSlice(permsRaw any) []any {
	switch perms := permsRaw.(type) {
	case []any:
		var translated []any
		for _, p := range perms {
			if s, ok := p.(string); ok {
				translated = append(translated, TranslatePermissionToCursor(s))
			}
		}
		return translated
	default:
		return nil
	}
}

// --- Cursor hook event rename tables ---

// ClaudeToCursorHookEvents maps IR canonical (PascalCase) event names to
// Cursor-native names where the name differs beyond simple case conversion.
var ClaudeToCursorHookEvents = map[string]string{
	"UserPromptSubmit": "beforeSubmitPrompt",
}

// ToClaudeHookEvents is the inverse mapping: Cursor-native event
// names to IR canonical (PascalCase) form.
var ToClaudeHookEvents = map[string]string{
	"beforeSubmitPrompt": "UserPromptSubmit",
}

// HookKeyTransform converts an IR canonical (PascalCase) hook event
// name to Cursor's native form. It checks the explicit rename table first,
// then falls back to simple pascalToCamel case conversion.
func HookKeyTransform(s string) string {
	if renamed, ok := ClaudeToCursorHookEvents[s]; ok {
		return renamed
	}
	return PascalToCamel(s)
}

// --- Cursor-specific helpers ---

// WrapMDC wraps instruction content in .mdc frontmatter.
func WrapMDC(content string) string {
	return fmt.Sprintf(`---
description: "Keel workspace instructions (assembled from config layers)"
alwaysApply: true
---
%s`, content)
}

// TranslatePermissionToCursor converts Claude permission format to Cursor.
// Bare names get (*) appended; already-parameterized forms pass through.
func TranslatePermissionToCursor(perm string) string {
	if strings.Contains(perm, "(") {
		return perm
	}
	return perm + "(*)"
}

// PascalToCamel converts PascalCase to camelCase.
// PreToolUse -> preToolUse, PostToolUse -> postToolUse
func PascalToCamel(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
