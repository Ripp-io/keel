// Package policy defines the governance types shared across keel packages.
// These types are extracted here so that data-definition packages (schema)
// do not depend on the merge engine.
package policy

// MergeStrategy defines how a config item handles conflicts across layers.
type MergeStrategy string

const (
	// MergeStrategyDeepMerge recursively merges objects (default for JSON).
	MergeStrategyDeepMerge MergeStrategy = "deep-merge"
	// MergeStrategyReplace replaces the entire value (last writer wins).
	MergeStrategyReplace MergeStrategy = "replace"
	// MergeStrategyAdditive allows additions but prevents removals.
	MergeStrategyAdditive MergeStrategy = "additive"
)

// IsValidStrategy reports whether s is a recognized merge strategy value.
func IsValidStrategy(s string) bool {
	switch MergeStrategy(s) {
	case MergeStrategyDeepMerge, MergeStrategyReplace, MergeStrategyAdditive:
		return true
	default:
		return false
	}
}

// KeelPolicy is the unified governance metadata that can appear in:
//   - JSON files as `_keel: { force: true, merge: "replace" }`
//   - Markdown frontmatter as `force: true` / `scope: path/to/dir`
//
// Both sources parse into the same struct. Fields:
//   - Force: prevents downstream layers from overriding this item
//   - Merge: controls how conflicts are resolved (default: deep-merge)
//   - Scope: limits applicability to a path prefix (instructions, rules)
type KeelPolicy struct {
	Force *bool         `json:"force,omitempty" yaml:"force,omitempty"`
	Merge MergeStrategy `json:"merge,omitempty" yaml:"merge,omitempty"`
	Scope string        `json:"scope,omitempty" yaml:"scope,omitempty"`
}
