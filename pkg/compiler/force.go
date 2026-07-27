package compiler

import (
	"sort"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// forceRegistry tracks force-locked items across layers.
// Items are registered by category + key (e.g. "mcp" + "gitlab",
// "command" + "deploy.md").
//
// Not thread-safe — assumes single-threaded compilation.
type forceRegistry struct {
	locked map[string]map[string]string // category → key → source layer
}

// newForceRegistry creates an empty forceRegistry.
func newForceRegistry() *forceRegistry {
	return &forceRegistry{locked: make(map[string]map[string]string)}
}

// lock marks an item as force-locked by a layer.
func (r *forceRegistry) lock(category, key, layer string) {
	if r.locked[category] == nil {
		r.locked[category] = make(map[string]string)
	}
	r.locked[category][key] = layer
}

// isLocked reports whether the item is force-locked.
func (r *forceRegistry) isLocked(category, key string) (bool, string) {
	if inner, ok := r.locked[category]; ok {
		layer, locked := inner[key]
		return locked, layer
	}
	return false, ""
}

// isCategoryLocked checks if ANY key in the given category is locked.
// Returns true if at least one lock exists for the category, along with
// the lock key and source layer.
func (r *forceRegistry) isCategoryLocked(category string) (bool, string) {
	inner, ok := r.locked[category]
	if !ok || len(inner) == 0 {
		return false, ""
	}
	// Return info about the first entry (map iteration order is random, but
	// we only need to report that *some* lock exists)
	for key, layer := range inner {
		return true, layer + " (" + key + ")"
	}
	return false, ""
}

// locks returns all force-locked items.
func (r *forceRegistry) locks() []manifest.ForceLockRecord {
	var records []manifest.ForceLockRecord
	for category, inner := range r.locked {
		for key, layer := range inner {
			records = append(records, manifest.ForceLockRecord{
				Category: category,
				Key:      key,
				LockedBy: layer,
			})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Category != records[j].Category {
			return records[i].Category < records[j].Category
		}
		return records[i].Key < records[j].Key
	})
	return records
}
