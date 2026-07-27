package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findRepoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func parseMatrixTable(content, sectionHeader string) ([]map[string]string, error) {
	lines := strings.Split(content, "\n")
	sectionIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") && strings.Contains(trimmed, sectionHeader) {
			sectionIdx = i
			break
		}
	}
	if sectionIdx < 0 {
		return nil, fmt.Errorf("section header %q not found", sectionHeader)
	}
	headerIdx := -1
	for i := sectionIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "|") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("no table found after section %q", sectionHeader)
	}
	columns := parseTableRow(lines[headerIdx])
	if len(columns) == 0 {
		return nil, fmt.Errorf("empty table header in section %q", sectionHeader)
	}
	dataStart := headerIdx + 1
	if dataStart < len(lines) {
		trimmed := strings.TrimSpace(lines[dataStart])
		if strings.Contains(trimmed, "---") {
			dataStart++
		}
	}
	var rows []map[string]string
	for i := dataStart; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "## ") || !strings.Contains(trimmed, "|") {
			break
		}
		cells := parseTableRow(lines[i])
		row := make(map[string]string)
		for j, col := range columns {
			if j < len(cells) {
				row[col] = cells[j]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func TestPortabilityMatchesMatrix_HookEvents(t *testing.T) {
	t.Parallel()
	root := findRepoRoot()
	if root == "" {
		t.Skip("cannot find repo root (go.mod)")
	}
	matrixPath := filepath.Join(root, "docs", "content", "docs", "reference", "compatibility-matrix.md")
	content, err := os.ReadFile(matrixPath)
	require.NoError(t, err, "reading matrix file")
	rows, err := parseMatrixTable(string(content), "Section 2: Hook Events")
	require.NoError(t, err, "parsing hook events table")
	matrixPortable := make(map[string]bool)
	for _, row := range rows {
		if row["Portable"] != "✅" {
			continue
		}
		irEvent := row["IR Event (PascalCase)"]
		if fields := strings.Fields(irEvent); len(fields) > 0 {
			matrixPortable[fields[0]] = true
		}
	}
	var matrixOnly []string
	for event := range matrixPortable {
		if !portableHookEvents[event] {
			matrixOnly = append(matrixOnly, event)
		}
	}
	sort.Strings(matrixOnly)
	for _, event := range matrixOnly {
		assert.Fail(t, "matrix claims event is portable but portableHookEvents is missing it", event)
	}
	var codeOnly []string
	for event := range portableHookEvents {
		if !matrixPortable[event] {
			codeOnly = append(codeOnly, event)
		}
	}
	sort.Strings(codeOnly)
	for _, event := range codeOnly {
		assert.Fail(t, "portableHookEvents has event but matrix doesn't mark it portable", event)
	}
}

func TestPortabilityMatchesMatrix_HandlerTypes(t *testing.T) {
	t.Parallel()
	root := findRepoRoot()
	if root == "" {
		t.Skip("cannot find repo root (go.mod)")
	}
	matrixPath := filepath.Join(root, "docs", "content", "docs", "reference", "compatibility-matrix.md")
	content, err := os.ReadFile(matrixPath)
	require.NoError(t, err, "reading matrix file")
	rows, err := parseMatrixTable(string(content), "Section 3: Hook Handler Types")
	require.NoError(t, err, "parsing handler types table")
	matrixPortable := make(map[string]bool)
	for _, row := range rows {
		if row["Portable"] != "✅" {
			continue
		}
		handlerType := strings.TrimSpace(row["Type"])
		if handlerType != "" {
			matrixPortable[handlerType] = true
		}
	}
	var matrixOnly []string
	for ht := range matrixPortable {
		if !portableHandlerTypes[ht] {
			matrixOnly = append(matrixOnly, ht)
		}
	}
	sort.Strings(matrixOnly)
	for _, ht := range matrixOnly {
		assert.Fail(t, "matrix claims handler type is portable but portableHandlerTypes is missing it", ht)
	}
	var codeOnly []string
	for ht := range portableHandlerTypes {
		if !matrixPortable[ht] {
			codeOnly = append(codeOnly, ht)
		}
	}
	sort.Strings(codeOnly)
	for _, ht := range codeOnly {
		assert.Fail(t, "portableHandlerTypes has type but matrix doesn't mark it portable", ht)
	}
}

func TestPortabilityMatchesMatrix_FrontmatterFields(t *testing.T) {
	t.Parallel()
	root := findRepoRoot()
	if root == "" {
		t.Skip("cannot find repo root (go.mod)")
	}
	matrixPath := filepath.Join(root, "docs", "content", "docs", "reference", "compatibility-matrix.md")
	content, err := os.ReadFile(matrixPath)
	require.NoError(t, err, "reading matrix file")
	rows, err := parseMatrixTable(string(content), "Section 4: Frontmatter Fields by Category")
	require.NoError(t, err, "parsing frontmatter fields table")
	matrixPortable := make(map[string]map[string]bool)
	for _, row := range rows {
		if row["Portable"] != "✅" {
			continue
		}
		category := strings.TrimSpace(row["Category"])
		field := strings.TrimSpace(row["Field"])
		if category == "" || field == "" {
			continue
		}
		if matrixPortable[category] == nil {
			matrixPortable[category] = make(map[string]bool)
		}
		matrixPortable[category][field] = true
	}
	matrixPortable = collectFrontmatterPortableFields(t, string(content))
	allCategories := make(map[string]bool)
	for cat := range matrixPortable {
		allCategories[cat] = true
	}
	for cat := range portableFields {
		allCategories[cat] = true
	}
	for cat := range allCategories {
		matrixFields := matrixPortable[cat]
		codeFields := portableFields[cat]
		var matrixOnly []string
		for field := range matrixFields {
			if codeFields == nil || !codeFields[field] {
				matrixOnly = append(matrixOnly, field)
			}
		}
		sort.Strings(matrixOnly)
		for _, field := range matrixOnly {
			assert.Failf(t, "matrix claims field is portable but portableFields is missing it", "category %q, field %q", cat, field)
		}
		var codeOnly []string
		for field := range codeFields {
			if matrixFields == nil || !matrixFields[field] {
				codeOnly = append(codeOnly, field)
			}
		}
		sort.Strings(codeOnly)
		for _, field := range codeOnly {
			assert.Failf(t, "portableFields has field but matrix doesn't mark it portable", "category %q, field %q", cat, field)
		}
	}
}

// --- Frontmatter translation tests ---

func TestTranslateCategoryFrontmatter_BackgroundToIsBackground(t *testing.T) {
	t.Parallel()

	ir := &LayerIR{
		SourceAgent: "claude",
		Categories: map[string][]CategoryFile{
			"agents": {
				{
					RelPath: "worker.md",
					Content: []byte("---\nname: worker\nbackground: true\nmodel: sonnet\n---\nDo background work.\n"),
				},
			},
		},
	}

	foldConfig := FoldConfig{Categories: []CategoryDef{{DirName: "agents", LockCategory: "agent"}}}
	state := NewAccumulationState(foldConfig, CompileOptions{}, "/tmp", 1)
	_ = state.Fold(ir, "test")
	state.ApplyGovernance()
	state.TranslateFrontmatter("cursor")
	result := state.Result()

	content := string(result.Content.Categories["agents"][0].Content)
	assert.Contains(t, content, "is_background:")
	assert.Contains(t, content, "name: worker")
	assert.Contains(t, content, "model: sonnet")
	assert.Contains(t, content, "Do background work.")

	// Verify the original "background:" key (without "is_" prefix) is gone.
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "background:") {
			t.Errorf("found bare 'background:' key that should have been renamed: %q", trimmed)
		}
	}
}

func TestTranslateCategoryFrontmatter_IsBackgroundToBackground(t *testing.T) {
	t.Parallel()

	ir := &LayerIR{
		SourceAgent: "cursor",
		Categories: map[string][]CategoryFile{
			"agents": {
				{
					RelPath: "worker.md",
					Content: []byte("---\nname: worker\nis_background: true\nmodel: opus\n---\nBackground agent.\n"),
				},
			},
		},
	}

	foldConfig := FoldConfig{Categories: []CategoryDef{{DirName: "agents", LockCategory: "agent"}}}
	state := NewAccumulationState(foldConfig, CompileOptions{}, "/tmp", 1)
	_ = state.Fold(ir, "test")
	state.ApplyGovernance()
	state.TranslateFrontmatter("claude")
	result := state.Result()

	content := string(result.Content.Categories["agents"][0].Content)
	assert.Contains(t, content, "background:")
	assert.NotContains(t, content, "is_background:")
	assert.Contains(t, content, "name: worker")
	assert.Contains(t, content, "Background agent.")
}

func TestTranslateCategoryFrontmatter_SameAgent_NoChange(t *testing.T) {
	t.Parallel()

	originalContent := []byte("---\nname: worker\nbackground: true\n---\nContent.\n")
	ir := &LayerIR{
		SourceAgent: "claude",
		Categories: map[string][]CategoryFile{
			"agents": {
				{
					RelPath: "worker.md",
					Content: originalContent,
				},
			},
		},
	}

	foldConfig := FoldConfig{Categories: []CategoryDef{{DirName: "agents", LockCategory: "agent"}}}
	state := NewAccumulationState(foldConfig, CompileOptions{}, "/tmp", 1)
	_ = state.Fold(ir, "test")
	state.ApplyGovernance()
	state.TranslateFrontmatter("claude")
	result := state.Result()

	// Should be unchanged
	assert.Equal(t, string(originalContent), string(result.Content.Categories["agents"][0].Content))
}

func TestTranslateCategoryFrontmatter_KeelDSL_NoChange(t *testing.T) {
	t.Parallel()

	// DSL layers use SourceAgent "keel" and should not be translated.
	originalContent := []byte("---\nname: worker\nbackground: true\n---\nContent.\n")
	ir := &LayerIR{
		SourceAgent: "keel",
		Categories: map[string][]CategoryFile{
			"agents": {
				{
					RelPath: "worker.md",
					Content: originalContent,
				},
			},
		},
	}

	foldConfig := FoldConfig{Categories: []CategoryDef{{DirName: "agents", LockCategory: "agent"}}}
	state := NewAccumulationState(foldConfig, CompileOptions{}, "/tmp", 1)
	_ = state.Fold(ir, "test")
	state.ApplyGovernance()
	state.TranslateFrontmatter("cursor")
	result := state.Result()

	assert.Equal(t, string(originalContent), string(result.Content.Categories["agents"][0].Content))
}

func TestTranslateCategoryFrontmatter_NonAgentCategory_NoChange(t *testing.T) {
	t.Parallel()

	// Commands don't have background/is_background, should be left alone.
	originalContent := []byte("---\nname: deploy\nbackground: true\n---\nDeploy command.\n")
	ir := &LayerIR{
		SourceAgent: "claude",
		Categories: map[string][]CategoryFile{
			"commands": {
				{
					RelPath: "deploy.md",
					Content: originalContent,
				},
			},
		},
	}

	foldConfig := FoldConfig{Categories: []CategoryDef{{DirName: "commands", LockCategory: "command"}}}
	state := NewAccumulationState(foldConfig, CompileOptions{}, "/tmp", 1)
	_ = state.Fold(ir, "test")
	state.ApplyGovernance()
	state.TranslateFrontmatter("cursor")
	result := state.Result()

	assert.Equal(t, string(originalContent), string(result.Content.Categories["commands"][0].Content))
}

func TestTranslateCategoryFrontmatter_NoFrontmatter_NoChange(t *testing.T) {
	t.Parallel()

	originalContent := []byte("No frontmatter here, just an agent description.\n")
	ir := &LayerIR{
		SourceAgent: "claude",
		Categories: map[string][]CategoryFile{
			"agents": {
				{
					RelPath: "worker.md",
					Content: originalContent,
				},
			},
		},
	}

	foldConfig := FoldConfig{Categories: []CategoryDef{{DirName: "agents", LockCategory: "agent"}}}
	state := NewAccumulationState(foldConfig, CompileOptions{}, "/tmp", 1)
	_ = state.Fold(ir, "test")
	state.ApplyGovernance()
	state.TranslateFrontmatter("cursor")
	result := state.Result()

	assert.Equal(t, string(originalContent), string(result.Content.Categories["agents"][0].Content))
}

func collectFrontmatterPortableFields(t *testing.T, content string) map[string]map[string]bool {
	t.Helper()
	lines := strings.Split(content, "\n")
	sectionStart := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") && strings.Contains(trimmed, "Section 4") {
			sectionStart = i
			break
		}
	}
	require.NotEqual(t, -1, sectionStart, "Section 4 not found in matrix")
	sectionEnd := len(lines)
	for i := sectionStart + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "## ") {
			sectionEnd = i
			break
		}
	}
	result := make(map[string]map[string]bool)
	i := sectionStart + 1
	for i < sectionEnd {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.Contains(trimmed, "|") || strings.Contains(trimmed, "---") {
			i++
			continue
		}
		if !strings.Contains(trimmed, "Category") && !strings.Contains(trimmed, "Field") {
			i++
			continue
		}
		columns := parseTableRow(lines[i])
		i++
		if i < sectionEnd {
			sep := strings.TrimSpace(lines[i])
			if strings.Contains(sep, "---") {
				i++
			}
		}
		for i < sectionEnd {
			trimmed = strings.TrimSpace(lines[i])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "|") {
				break
			}
			cells := parseTableRow(lines[i])
			row := make(map[string]string)
			for j, col := range columns {
				if j < len(cells) {
					row[col] = cells[j]
				}
			}
			if row["Portable"] == "✅" {
				category := strings.TrimSpace(row["Category"])
				field := strings.TrimSpace(row["Field"])
				if category != "" && field != "" {
					if result[category] == nil {
						result[category] = make(map[string]bool)
					}
					result[category][field] = true
				}
			}
			i++
		}
	}
	return result
}
