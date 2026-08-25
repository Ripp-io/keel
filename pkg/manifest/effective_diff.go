package manifest

import (
	"sort"
)

// EffectiveStateDiff represents the differences between two effective states.
type EffectiveStateDiff struct {
	Permissions  *PermissionsDiff  `json:"permissions,omitempty"  yaml:"permissions,omitempty"`
	MCPServers   *ListDiff         `json:"mcp_servers,omitempty"  yaml:"mcp_servers,omitempty"`
	Hooks        *ListDiff         `json:"hooks,omitempty"        yaml:"hooks,omitempty"`
	Settings     []SettingChange   `json:"settings,omitempty"     yaml:"settings,omitempty"`
	Instructions *InstructionsDiff `json:"instructions,omitempty" yaml:"instructions,omitempty"`
	Categories   []CategoryDiff    `json:"categories,omitempty"   yaml:"categories,omitempty"`
}

// PermissionsDiff captures permission list changes.
type PermissionsDiff struct {
	AllowAdded   []string `json:"allow_added,omitempty"   yaml:"allow_added,omitempty"`
	AllowRemoved []string `json:"allow_removed,omitempty" yaml:"allow_removed,omitempty"`
	DenyAdded    []string `json:"deny_added,omitempty"    yaml:"deny_added,omitempty"`
	DenyRemoved  []string `json:"deny_removed,omitempty"  yaml:"deny_removed,omitempty"`
}

// ListDiff captures added/removed items in a list.
type ListDiff struct {
	Added   []string `json:"added,omitempty"   yaml:"added,omitempty"`
	Removed []string `json:"removed,omitempty" yaml:"removed,omitempty"`
}

// SettingChange records a setting value change.
type SettingChange struct {
	Key  string `json:"key"            yaml:"key"`
	From any    `json:"from,omitempty" yaml:"from,omitempty"`
	To   any    `json:"to,omitempty"   yaml:"to,omitempty"`
}

// InstructionsDiff captures instruction size changes.
type InstructionsDiff struct {
	TokenDelta int `json:"token_delta" yaml:"token_delta"`
	ByteDelta  int `json:"byte_delta"  yaml:"byte_delta"`
}

// CategoryDiff captures file count changes in a category.
type CategoryDiff struct {
	Name       string `json:"name"        yaml:"name"`
	CountDelta int    `json:"count_delta" yaml:"count_delta"`
}

// DiffEffectiveState compares two effective states and returns the differences.
// Returns nil if no differences detected.
func DiffEffectiveState(compiled, current *EffectiveState) *EffectiveStateDiff {
	if compiled == nil && current == nil {
		return nil
	}

	diff := &EffectiveStateDiff{}
	hasChanges := false

	// Diff permissions
	if permDiff := diffPermissions(compiled, current); permDiff != nil {
		diff.Permissions = permDiff
		hasChanges = true
	}

	// Diff MCP servers
	if mcpDiff := diffMCPServers(compiled, current); mcpDiff != nil {
		diff.MCPServers = mcpDiff
		hasChanges = true
	}

	// Diff hooks
	if hookDiff := diffHooks(compiled, current); hookDiff != nil {
		diff.Hooks = hookDiff
		hasChanges = true
	}

	// Diff settings
	if settingChanges := diffSettings(compiled, current); len(settingChanges) > 0 {
		diff.Settings = settingChanges
		hasChanges = true
	}

	// Diff instructions
	if instrDiff := diffInstructions(compiled, current); instrDiff != nil {
		diff.Instructions = instrDiff
		hasChanges = true
	}

	// Diff categories
	if catDiff := diffCategories(compiled, current); len(catDiff) > 0 {
		diff.Categories = catDiff
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}

	return diff
}

// HasChanges returns true if the diff contains any changes.
func (d *EffectiveStateDiff) HasChanges() bool {
	if d == nil {
		return false
	}
	return d.Permissions != nil ||
		d.MCPServers != nil ||
		d.Hooks != nil ||
		len(d.Settings) > 0 ||
		d.Instructions != nil ||
		len(d.Categories) > 0
}

// diffPermissions compares permission lists.
func diffPermissions(compiled, current *EffectiveState) *PermissionsDiff {
	var compiledPerms, currentPerms *EffectivePermissions
	if compiled != nil {
		compiledPerms = compiled.Permissions
	}
	if current != nil {
		currentPerms = current.Permissions
	}

	if compiledPerms == nil && currentPerms == nil {
		return nil
	}

	diff := &PermissionsDiff{}
	hasChanges := false

	// Diff allow lists
	var compiledAllow, currentAllow []string
	if compiledPerms != nil {
		compiledAllow = compiledPerms.Allow
	}
	if currentPerms != nil {
		currentAllow = currentPerms.Allow
	}

	added, removed := diffStringLists(compiledAllow, currentAllow)
	if len(added) > 0 {
		diff.AllowAdded = added
		hasChanges = true
	}
	if len(removed) > 0 {
		diff.AllowRemoved = removed
		hasChanges = true
	}

	// Diff deny lists
	var compiledDeny, currentDeny []string
	if compiledPerms != nil {
		compiledDeny = compiledPerms.Deny
	}
	if currentPerms != nil {
		currentDeny = currentPerms.Deny
	}

	added, removed = diffStringLists(compiledDeny, currentDeny)
	if len(added) > 0 {
		diff.DenyAdded = added
		hasChanges = true
	}
	if len(removed) > 0 {
		diff.DenyRemoved = removed
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}

	return diff
}

// diffNamedItems extracts string names from two typed slices of the compiled and
// current EffectiveState, then returns the added/removed diff. Collapses the
// shared pattern used by MCP server and hook diffing.
func diffNamedItems[T any](
	compiled, current *EffectiveState,
	getSlice func(*EffectiveState) []T,
	getName func(T) string,
) *ListDiff {
	var compiledItems, currentItems []T
	if compiled != nil {
		compiledItems = getSlice(compiled)
	}
	if current != nil {
		currentItems = getSlice(current)
	}

	compiledNames := make([]string, len(compiledItems))
	for i, item := range compiledItems {
		compiledNames[i] = getName(item)
	}

	currentNames := make([]string, len(currentItems))
	for i, item := range currentItems {
		currentNames[i] = getName(item)
	}

	added, removed := diffStringLists(compiledNames, currentNames)
	if len(added) == 0 && len(removed) == 0 {
		return nil
	}

	return &ListDiff{
		Added:   added,
		Removed: removed,
	}
}

// diffMCPServers compares MCP server lists.
func diffMCPServers(compiled, current *EffectiveState) *ListDiff {
	return diffNamedItems(compiled, current,
		func(s *EffectiveState) []EffectiveMCPServer { return s.MCPServers },
		func(s EffectiveMCPServer) string { return s.Name },
	)
}

// diffHooks compares hook event lists.
func diffHooks(compiled, current *EffectiveState) *ListDiff {
	return diffNamedItems(compiled, current,
		func(s *EffectiveState) []EffectiveHook { return s.Hooks },
		func(h EffectiveHook) string { return h.Event },
	)
}

// diffSettings compares setting key-value pairs.
func diffSettings(compiled, current *EffectiveState) []SettingChange {
	var compiledSettings, currentSettings map[string]any
	if compiled != nil {
		compiledSettings = compiled.Settings
	}
	if current != nil {
		currentSettings = current.Settings
	}

	var changes []SettingChange

	// Find all unique keys
	allKeys := make(map[string]bool)
	for k := range compiledSettings {
		allKeys[k] = true
	}
	for k := range currentSettings {
		allKeys[k] = true
	}

	for key := range allKeys {
		compiledVal, compiledExists := compiledSettings[key]
		currentVal, currentExists := currentSettings[key]

		if !compiledExists && currentExists {
			// Added
			changes = append(changes, SettingChange{
				Key: key,
				To:  currentVal,
			})
		} else if compiledExists && !currentExists {
			// Removed
			changes = append(changes, SettingChange{
				Key:  key,
				From: compiledVal,
			})
		} else if !settingValuesEqual(compiledVal, currentVal) {
			// Changed
			changes = append(changes, SettingChange{
				Key:  key,
				From: compiledVal,
				To:   currentVal,
			})
		}
	}

	// Sort by key for deterministic output
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Key < changes[j].Key
	})

	return changes
}

// settingValuesEqual compares two setting values for equality.
// Handles the fact that maps and slices can't be directly compared with ==.
func settingValuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Try direct comparison for comparable types (strings, numbers, bools)
	switch aTyped := a.(type) {
	case string, int, int64, float64, bool:
		return a == b
	case []any:
		bTyped, ok := b.([]any)
		if !ok {
			return false
		}
		if len(aTyped) != len(bTyped) {
			return false
		}
		for i := range aTyped {
			if !settingValuesEqual(aTyped[i], bTyped[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		bTyped, ok := b.(map[string]any)
		if !ok {
			return false
		}
		if len(aTyped) != len(bTyped) {
			return false
		}
		for k, v := range aTyped {
			bv, ok := bTyped[k]
			if !ok || !settingValuesEqual(v, bv) {
				return false
			}
		}
		return true
	default:
		// For unknown types, assume they're different to be safe
		return false
	}
}

// diffInstructions compares instruction sizes.
func diffInstructions(compiled, current *EffectiveState) *InstructionsDiff {
	var compiledInstr, currentInstr *EffectiveInstructions
	if compiled != nil {
		compiledInstr = compiled.Instructions
	}
	if current != nil {
		currentInstr = current.Instructions
	}

	if compiledInstr == nil && currentInstr == nil {
		return nil
	}

	var compiledBytes, compiledTokens int
	if compiledInstr != nil {
		compiledBytes = compiledInstr.Bytes
		compiledTokens = compiledInstr.TokenEstimate
	}

	var currentBytes, currentTokens int
	if currentInstr != nil {
		currentBytes = currentInstr.Bytes
		currentTokens = currentInstr.TokenEstimate
	}

	byteDelta := currentBytes - compiledBytes
	tokenDelta := currentTokens - compiledTokens

	if byteDelta == 0 && tokenDelta == 0 {
		return nil
	}

	return &InstructionsDiff{
		ByteDelta:  byteDelta,
		TokenDelta: tokenDelta,
	}
}

// diffCategories compares category file counts.
func diffCategories(compiled, current *EffectiveState) []CategoryDiff {
	var compiledCats, currentCats []EffectiveCategory
	if compiled != nil {
		compiledCats = compiled.Categories
	}
	if current != nil {
		currentCats = current.Categories
	}

	// Build maps for easy lookup
	compiledMap := make(map[string]int)
	for _, cat := range compiledCats {
		compiledMap[cat.Name] = cat.FileCount
	}

	currentMap := make(map[string]int)
	for _, cat := range currentCats {
		currentMap[cat.Name] = cat.FileCount
	}

	// Find all unique category names
	allCats := make(map[string]bool)
	for name := range compiledMap {
		allCats[name] = true
	}
	for name := range currentMap {
		allCats[name] = true
	}

	var diffs []CategoryDiff
	for name := range allCats {
		compiledCount := compiledMap[name]
		currentCount := currentMap[name]
		delta := currentCount - compiledCount

		if delta != 0 {
			diffs = append(diffs, CategoryDiff{
				Name:       name,
				CountDelta: delta,
			})
		}
	}

	// Sort by name for deterministic output
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Name < diffs[j].Name
	})

	return diffs
}

// diffStringLists compares two string lists and returns added and removed items.
// Order-independent comparison using set operations.
func diffStringLists(compiled, current []string) (added, removed []string) {
	compiledSet := make(map[string]bool, len(compiled))
	for _, item := range compiled {
		compiledSet[item] = true
	}

	currentSet := make(map[string]bool, len(current))
	for _, item := range current {
		currentSet[item] = true
	}

	// Find added (in current but not in compiled)
	for item := range currentSet {
		if !compiledSet[item] {
			added = append(added, item)
		}
	}

	// Find removed (in compiled but not in current)
	for item := range compiledSet {
		if !currentSet[item] {
			removed = append(removed, item)
		}
	}

	sort.Strings(added)
	sort.Strings(removed)

	return added, removed
}
