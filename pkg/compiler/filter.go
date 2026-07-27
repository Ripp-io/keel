package compiler

import (
	"log/slog"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// applyFilter applies a layer filter to a LayerIR, modifying it in place.
// If filter is nil, this is a no-op. Filtering is applied after reading
// but before folding into the accumulator.
//
// PLANNED: Not yet wired into the compilation pipeline.
// Wire into CompileWithProfile Fold loop: call applyFilter(ir, layer.Filter)
// after reading but before folding. The Layer struct already has a Filter field.
func applyFilter(ir *LayerIR, filter *config.LayerFilter) {
	if filter == nil {
		return
	}

	// Map section names to IR field names and handlers.
	// We process each section mentioned in the filter.
	for sectionName, sectionFilter := range filter.Sections {
		switch sectionName {
		case "instructions":
			filterInstructionBlocks(&ir.Instructions, sectionFilter)
		case "skills":
			filterCategoryFiles(ir.Categories, "skills", sectionFilter)
		case "commands":
			filterCategoryFiles(ir.Categories, "commands", sectionFilter)
		case "agents":
			filterCategoryFiles(ir.Categories, "agents", sectionFilter)
		case "plugins":
			filterCategoryFiles(ir.Categories, "plugins", sectionFilter)
		case "agent-memory":
			filterCategoryFiles(ir.Categories, "agent-memory", sectionFilter)
		case "rules":
			filterCategoryFiles(ir.Categories, "rules", sectionFilter)
		case "snippets":
			filterSnippets(ir.Snippets, sectionFilter)
		case "loop":
			if blocks, ok := ir.ContentBlocks["loop"]; ok {
				filterInstructionBlocks(&blocks, sectionFilter)
				ir.ContentBlocks["loop"] = blocks
			}
		case "worktree-include":
			if blocks, ok := ir.ContentBlocks["worktree-include"]; ok {
				filterInstructionBlocks(&blocks, sectionFilter)
				ir.ContentBlocks["worktree-include"] = blocks
			}
		default:
			// Unknown or non-filterable section — warn and ignore.
			slog.Warn("layer filter references unsupported or non-filterable section",
				"section", sectionName,
				"layer", ir.LayerPath)
		}
	}
}

// filterInstructionBlocks filters a slice of InstructionBlock by filename.
func filterInstructionBlocks(blocks *[]InstructionBlock, sf *config.SectionFilter) {
	if sf.Disabled {
		*blocks = nil
		return
	}

	if len(sf.Include) == 0 && len(sf.Exclude) == 0 {
		// No patterns specified — no-op.
		return
	}

	filtered := make([]InstructionBlock, 0, len(*blocks))
	for _, block := range *blocks {
		if matchFilter(block.Filename, sf) {
			filtered = append(filtered, block)
		}
	}
	*blocks = filtered
}

// filterCategoryFiles filters files within a category by their RelPath.
func filterCategoryFiles(categories map[string][]CategoryFile, category string, sf *config.SectionFilter) {
	if categories == nil {
		return
	}

	if sf.Disabled {
		delete(categories, category)
		return
	}

	files, exists := categories[category]
	if !exists {
		return
	}

	if len(sf.Include) == 0 && len(sf.Exclude) == 0 {
		// No patterns specified — no-op.
		return
	}

	filtered := make([]CategoryFile, 0, len(files))
	for _, file := range files {
		if matchFilter(file.RelPath, sf) {
			filtered = append(filtered, file)
		}
	}

	if len(filtered) == 0 {
		delete(categories, category)
	} else {
		categories[category] = filtered
	}
}

// filterSnippets filters snippets by their names (map keys).
func filterSnippets(snippets map[string][]byte, sf *config.SectionFilter) {
	if sf.Disabled {
		for k := range snippets {
			delete(snippets, k)
		}
		return
	}

	if len(sf.Include) == 0 && len(sf.Exclude) == 0 {
		// No patterns specified — no-op.
		return
	}

	for name := range snippets {
		if !matchFilter(name, sf) {
			delete(snippets, name)
		}
	}
}

// matchFilter determines if name matches the include/exclude patterns in sf.
// Returns true if the name should be kept.
//
// Logic:
//   - If include is non-empty: name must match at least one include pattern
//   - If exclude is non-empty: name must not match any exclude pattern
//   - If both are present: include first, then exclude
//   - If neither is present: always true (no filtering)
func matchFilter(name string, sf *config.SectionFilter) bool {
	// First apply include filter (if present).
	if len(sf.Include) > 0 {
		matched := false
		for _, pattern := range sf.Include {
			if globMatch(pattern, name) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Then apply exclude filter (if present).
	if len(sf.Exclude) > 0 {
		for _, pattern := range sf.Exclude {
			if globMatch(pattern, name) {
				return false
			}
		}
	}

	return true
}

// globMatch performs filepath.Match with error handling.
// Returns false if the pattern is malformed.
func globMatch(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		// Pattern is malformed — log and treat as no match.
		slog.Warn("invalid glob pattern in filter", "pattern", pattern, "error", err)
		return false
	}
	return matched
}
