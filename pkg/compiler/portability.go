package compiler

import (
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

// portableFields defines frontmatter fields that are portable across all agents.
// Fields NOT in this set for a given category are agent-specific and will
// generate warnings during cross-format compilation.
//
// Source of truth: docs/content/docs/reference/compatibility-matrix.md § Section 4: Frontmatter Fields
// Verified by TestPortabilityMatchesMatrix_FrontmatterFields
//
// Note: Keel governance fields (force, scope, merge) are excluded because
// they are stripped during compilation and are not agent-facing.
var portableFields = map[string]map[string]bool{
	"agent": {
		"name": true, "description": true, "tools": true,
		"disallowedTools": true, "model": true, "permissionMode": true,
		"maxTurns": true, "skills": true, "mcpServers": true,
		"hooks": true, "memory": true, "background": true,
		"effort": true, "isolation": true, "color": true,
		"initialPrompt": true,
	},
	"skill": {
		"name": true, "description": true, "license": true,
		"allowed-tools": true, "metadata": true, "compatibility": true,
		"disable-model-invocation": true, "paths": true,
	},
	"command": {
		// Commands have NO portable frontmatter fields.
		// Claude supports frontmatter in commands; Cursor does not.
		// An empty map means ALL frontmatter fields trigger a warning.
	},
	"rule": {
		// Rules exist in both Claude (.claude/rules/*.md with "paths" frontmatter)
		// and Cursor (.cursor/rules/*.mdc with "alwaysApply", "globs", "description").
		// No frontmatter fields are directly portable — the concepts map loosely
		// but use different field names and semantics.
		// "description" is portable as a concept but Claude rules don't use it.
	},
}

// portableHookEvents lists hook events supported by all agents.
// Source of truth: docs/content/docs/reference/compatibility-matrix.md § Section 2: Hook Events
// Verified by TestPortabilityMatchesMatrix_HookEvents
// Last verified: May 2026.
//
// Event names use IR canonical form (PascalCase). Events not in this set are
// agent-specific and will generate warnings during cross-format compilation.
//
// Note on handler fields: Cursor-only handler fields like "loop_limit" and
// "failClosed" are nested inside handler objects and pass through deep-merge
// as-is. They are harmless in other agents (unknown fields are ignored).
//
// Note on input shape differences: Hook scripts receive different JSON input
// depending on which agent invokes them. For example, Claude's SubagentStart
// provides agent_id + agent_type, while Cursor provides richer fields
// (subagent_id, task, is_parallel_worker, etc.). This is a hook-script-level
// concern, not a compiler concern.
var portableHookEvents = map[string]bool{
	"PreToolUse":         true,
	"PostToolUse":        true,
	"PostToolUseFailure": true,
	"UserPromptSubmit":   true, // renamed to beforeSubmitPrompt in Cursor
	"Stop":               true,
	"SubagentStart":      true,
	"SubagentStop":       true,
	"SessionStart":       true,
	"SessionEnd":         true,
	"PreCompact":         true,
}

// isPortableHookEvent returns true if the event (in IR canonical PascalCase)
// is supported by all agents. Agent-specific events return false.
func isPortableHookEvent(event string) bool {
	return portableHookEvents[event]
}

// portableHandlerTypes lists hook handler types supported by all agents.
// Source of truth: docs/content/docs/reference/compatibility-matrix.md § Section 3: Hook Handler Types
// Verified by TestPortabilityMatchesMatrix_HandlerTypes
//
// Types not in this set are agent-specific and will generate warnings.
var portableHandlerTypes = map[string]bool{
	"command": true,
	"prompt":  true,
}

// extractNonPortableHandlerTypes inspects hook entries for handler type
// fields and returns any non-portable types found. Hook entries follow
// the structure: [{"matcher": "...", "hooks": [{"type": "command", ...}]}]
// or the flat handler format: [{"type": "command", "command": "..."}].
func extractNonPortableHandlerTypes(entries any) []string {
	arr, ok := entries.([]any)
	if !ok {
		return nil
	}

	seen := make(map[string]bool)
	for _, entry := range arr {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		// Check nested "hooks" array (Claude format: entry.hooks[].type)
		if hooksRaw, ok := entryMap["hooks"]; ok {
			if hooks, ok := hooksRaw.([]any); ok {
				for _, h := range hooks {
					if handler, ok := h.(map[string]any); ok {
						if t, ok := handler["type"].(string); ok && !portableHandlerTypes[t] {
							seen[t] = true
						}
					}
				}
			}
		}

		// Check flat format (Cursor format: entry.type)
		if t, ok := entryMap["type"].(string); ok && !portableHandlerTypes[t] {
			seen[t] = true
		}
	}

	var result []string
	for t := range seen {
		result = append(result, t)
	}
	sort.Strings(result)
	return result
}

// frontmatterTranslations maps source→target agent pairs to field renames.
// Each inner map is sourceField → targetField. Fields are renamed in-place
// during cross-format compilation so the target agent can read them natively.
var frontmatterTranslations = map[string]map[string]map[string]string{
	// Claude → Cursor: "background" becomes "is_background"
	"claude": {
		"cursor": {
			"background": "is_background",
		},
	},
	// Cursor → Claude: "is_background" becomes "background"
	"cursor": {
		"claude": {
			"is_background": "background",
		},
	},
}

// renameFrontmatterFields renames YAML frontmatter keys according to fieldMap.
// Returns the modified content and whether any changes were made.
func renameFrontmatterFields(data []byte, fieldMap map[string]string) ([]byte, bool) {
	rawYAML, err := merge.ExtractRawFrontmatter(data)
	if err != nil || rawYAML == nil {
		return data, false
	}

	// Check if any field needs renaming.
	needsRename := false
	for sourceField := range fieldMap {
		if _, ok := rawYAML[sourceField]; ok {
			needsRename = true
			break
		}
	}
	if !needsRename {
		return data, false
	}

	// Rename fields in the raw map.
	for sourceField, targetField := range fieldMap {
		if val, ok := rawYAML[sourceField]; ok {
			delete(rawYAML, sourceField)
			rawYAML[targetField] = val
		}
	}

	// Rebuild the file with renamed frontmatter.
	return merge.ReplaceFrontmatter(data, rawYAML)
}

// emitFrontmatterWarnings checks category files for frontmatter fields that
// may not be supported by the target agent. Only runs during cross-format
// compilation (sourceAgent != targetAgent).
func emitFrontmatterWarnings(ir *LayerIR, targetAgent, layerLabel string) {
	if ir.SourceAgent == targetAgent || ir.SourceAgent == "" {
		return // same-agent compile, no portability concerns
	}

	for catName, files := range ir.Categories {
		// Map category directory names to portability categories:
		// "agents" -> "agent", "skills" -> "skill", "commands" -> "command"
		portCat := categoryToPortabilityKey(catName)
		portable, hasPortDef := portableFields[portCat]

		for _, f := range files {
			if f.IsDir || !merge.IsMarkdownFile(f.RelPath) {
				continue
			}

			rawFM, err := merge.ExtractRawFrontmatter(f.Content)
			if err != nil || rawFM == nil {
				continue
			}

			// Check for non-portable fields
			var nonPortable []string
			for field := range rawFM {
				if keelGovernanceField(field) {
					continue // Keel fields are stripped, not a portability concern
				}
				if hasPortDef && !portable[field] {
					nonPortable = append(nonPortable, field)
				}
			}

			if len(nonPortable) > 0 {
				sort.Strings(nonPortable)
				slog.Warn("frontmatter contains fields not supported by target agent",
					"file", filepath.Join(catName, f.RelPath),
					"fields", strings.Join(nonPortable, ", "),
					"target", targetAgent,
					"layer", layerLabel)
			}
		}
	}

	// Special case: warn if commands have ANY frontmatter when targeting Cursor
	if targetAgent == "cursor" {
		cmdFiles, ok := ir.Categories["commands"]
		if !ok {
			return
		}
		for _, f := range cmdFiles {
			if f.IsDir || !merge.IsMarkdownFile(f.RelPath) {
				continue
			}
			rawFM, err := merge.ExtractRawFrontmatter(f.Content)
			if err != nil || rawFM == nil {
				continue
			}
			// Filter out Keel governance fields
			hasToolFields := false
			for field := range rawFM {
				if !keelGovernanceField(field) {
					hasToolFields = true
					break
				}
			}
			if hasToolFields {
				slog.Warn("command frontmatter is not supported by Cursor, fields will be ignored",
					"file", filepath.Join("commands", f.RelPath),
					"target", targetAgent,
					"layer", layerLabel)
			}
		}
	}
}

// categoryToPortabilityKey maps category directory names to portability table keys.
func categoryToPortabilityKey(catDirName string) string {
	switch catDirName {
	case "agents":
		return "agent"
	case "skills":
		return "skill"
	case "commands":
		return "command"
	default:
		return catDirName
	}
}

// keelGovernanceField returns true for Keel policy fields that are stripped
// during compilation and are not an agent portability concern.
func keelGovernanceField(field string) bool {
	switch field {
	case "force", "scope", "merge":
		return true
	default:
		return false
	}
}
