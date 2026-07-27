package compiler

import (
	"path/filepath"
	"strings"
)

// stripForceFromEntries removes the "force" field from hook entry objects.
func stripForceFromEntries(entries any) any {
	arr, ok := entries.([]any)
	if !ok {
		return entries
	}
	var cleaned []any
	for _, entry := range arr {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			cleaned = append(cleaned, entry)
			continue
		}
		cleanEntry := make(map[string]any, len(entryMap))
		for k, v := range entryMap {
			if k == "force" {
				continue
			}
			cleanEntry[k] = v
		}
		cleaned = append(cleaned, cleanEntry)
	}
	return cleaned
}

// mergeHookEvent concatenates already-cleaned hook entries into dst for the
// given event key. It does not call stripForceFromEntries, so it is safe for
// entries that have already been stripped.
func mergeHookEvent(dst map[string]any, eventName string, entries any) {
	existing, exists := dst[eventName]
	if !exists {
		dst[eventName] = entries
		return
	}
	existingArr, existingOk := existing.([]any)
	newArr, newOk := entries.([]any)
	if existingOk && newOk {
		dst[eventName] = append(existingArr, newArr...)
		return
	}
	dst[eventName] = entries
}

// hasForceEntry checks if any entry in a hook event's entries has "force": true.
func hasForceEntry(entries any) bool {
	arr, ok := entries.([]any)
	if !ok {
		return false
	}
	for _, entry := range arr {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if f, ok := entryMap["force"].(bool); ok && f {
			return true
		}
	}
	return false
}

// forceLockKeyForFile extracts the force-lock key from a file policy path,
// relative to the category root. If the first path component matches a known
// category, the prefix is stripped. Otherwise the basename is returned.
func forceLockKeyForFile(filePath string, categories []CategoryDef) string {
	dir := strings.SplitN(filepath.ToSlash(filePath), "/", 2)[0]
	for _, cat := range categories {
		if dir == cat.DirName {
			parts := strings.SplitN(filepath.ToSlash(filePath), "/", 2)
			if len(parts) == 2 {
				return parts[1]
			}
			return filepath.Base(filePath)
		}
	}
	return filepath.Base(filePath)
}

// categorizeFilePath determines the force-lock category for a file path
// within a native layer. It looks up the first path component against the
// provided category definitions.
func categorizeFilePath(path string, categories []CategoryDef) string {
	dir := strings.SplitN(filepath.ToSlash(path), "/", 2)[0]
	for _, cat := range categories {
		if dir == cat.DirName {
			return cat.LockCategory
		}
	}
	switch {
	case path == "settings.json" || path == "settings.local.json":
		return "settings"
	case path == "cli-config.json":
		return "settings"
	case path == "mcp.json" || path == ".mcp.json":
		return "mcp-file"
	case path == "hooks.json":
		return "hooks-file"
	default:
		return "file"
	}
}
