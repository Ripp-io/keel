package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

const (
	DefaultSkillBodyTokenThreshold = 5000
	DefaultSkillDescriptionChars   = 1024
)

// estimateTokens returns an estimated token count using the ~4 bytes per token heuristic.
// Uses ceiling division: (n + 3) / 4
func estimateTokens(data []byte) int {
	return estimateTokensFromSize(len(data))
}

// estimateTokensFromSize returns an estimated token count from a byte count.
func estimateTokensFromSize(n int) int {
	return (n + 3) / 4
}

// checkTokenBudget performs token budget estimation and emits warnings when
// thresholds are exceeded. All findings are warnings, never errors.
func checkTokenBudget(result *Result, layerPath string, keelExists bool, cfg *schema.KeelConfig) {
	// Resolve thresholds from config or use defaults.
	skillBodyThreshold := DefaultSkillBodyTokenThreshold
	instructionsThreshold := 0
	layerThreshold := 0

	if cfg != nil && cfg.ValidateOpts != nil && cfg.ValidateOpts.TokenBudget != nil {
		if cfg.ValidateOpts.TokenBudget.SkillBody != nil {
			skillBodyThreshold = *cfg.ValidateOpts.TokenBudget.SkillBody
		}
		if cfg.ValidateOpts.TokenBudget.Instructions != nil {
			instructionsThreshold = *cfg.ValidateOpts.TokenBudget.Instructions
		}
		if cfg.ValidateOpts.TokenBudget.Layer != nil {
			layerThreshold = *cfg.ValidateOpts.TokenBudget.Layer
		}
	}

	// Check per-skill body tokens and description length.
	checkSkillTokens(result, layerPath, keelExists, cfg, skillBodyThreshold)

	// Check instructions total (DSL layers only).
	if keelExists && cfg != nil {
		checkInstructionsTotal(result, layerPath, cfg, instructionsThreshold)
	}

	// Check layer content total (DSL layers only).
	if keelExists && cfg != nil {
		checkLayerTotal(result, layerPath, cfg, layerThreshold)
	}
}

// checkSkillTokens validates skill body token counts and description lengths.
func checkSkillTokens(result *Result, layerPath string, keelExists bool, cfg *schema.KeelConfig, bodyThreshold int) {
	skillDirs, singleSkills := discoverSkillPaths(layerPath, keelExists, cfg)

	if len(skillDirs) == 0 && len(singleSkills) == 0 {
		return
	}

	skillsChecked := 0
	allUnderThreshold := true

	// Validate parent directories (each subdirectory is a skill).
	for _, dir := range skillDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillPath := filepath.Join(dir, e.Name())
			if checkSingleSkillTokens(result, skillPath, bodyThreshold) {
				allUnderThreshold = false
			}
			skillsChecked++
		}
	}

	// Validate individually-referenced skill directories.
	for _, skillPath := range singleSkills {
		if checkSingleSkillTokens(result, skillPath, bodyThreshold) {
			allUnderThreshold = false
		}
		skillsChecked++
	}

	// If skills were checked and all are under threshold, emit a pass.
	if skillsChecked > 0 && allUnderThreshold && bodyThreshold > 0 {
		result.addPass("token budget: skill bodies within threshold")
	}
}

// checkSingleSkillTokens checks a single skill directory's body token count
// and description length. Returns true if any threshold was exceeded.
func checkSingleSkillTokens(result *Result, skillPath string, bodyThreshold int) bool {
	dirName := filepath.Base(skillPath)
	exceeded := false

	// Read SKILL.md or skill.md.
	data, err := readSkillFile(skillPath)
	if err != nil {
		return false // file not found — already reported by validateSkills
	}

	// Extract frontmatter to get description and to strip it from body.
	fm, err := merge.ExtractRawFrontmatter(data)
	if err != nil || fm == nil {
		return false // frontmatter parse error — already reported by validateSkills
	}

	// Check description length (discovery tier limit: 1024 chars).
	if desc, ok := fm["description"].(string); ok {
		if len(desc) > DefaultSkillDescriptionChars {
			result.addWarn(fmt.Sprintf("token budget: skill %q description %d chars exceeds discovery tier limit (%d)", dirName, len(desc), DefaultSkillDescriptionChars))
			exceeded = true
		}
	}

	// Strip frontmatter to get body content.
	body := stripFrontmatterContent(data)

	// Estimate body tokens (only if threshold is enabled).
	if bodyThreshold > 0 {
		bodyTokens := estimateTokens(body)
		if bodyTokens > bodyThreshold {
			result.addWarn(fmt.Sprintf("token budget: skill %q body ~%d tokens (threshold: %d)", dirName, bodyTokens, bodyThreshold))
			exceeded = true
		}
	}

	return exceeded
}

// readSkillFile reads SKILL.md (preferred) or skill.md from the given directory.
func readSkillFile(dirPath string) ([]byte, error) {
	preferred := filepath.Join(dirPath, "SKILL.md")
	data, err := os.ReadFile(preferred)
	if err == nil {
		return data, nil
	}

	fallback := filepath.Join(dirPath, "skill.md")
	data, err = os.ReadFile(fallback)
	if err == nil {
		return data, nil
	}

	return nil, fmt.Errorf("neither SKILL.md nor skill.md found")
}

// stripFrontmatterContent removes frontmatter from markdown content and returns the body.
func stripFrontmatterContent(data []byte) []byte {
	// For token estimation, we need to strip ALL frontmatter (not just Keel fields).
	// We want to count only the markdown body that gets sent to the LLM.
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return data // no frontmatter
	}

	// Find closing delimiter.
	rest := normalized[4:] // skip opening "---\n"
	closingIdx := strings.Index(rest, "\n---\n")
	if closingIdx == -1 {
		// Check if it ends with \n--- at EOF.
		if strings.HasSuffix(rest, "\n---") {
			closingIdx = len(rest) - 4
		} else {
			return data // no valid frontmatter
		}
	}

	// Body starts after the closing "---\n".
	bodyStart := closingIdx + 5
	if bodyStart > len(rest) {
		return []byte{} // empty body
	}
	return []byte(rest[bodyStart:])
}

// checkInstructionsTotal estimates total instruction token count.
func checkInstructionsTotal(result *Result, layerPath string, cfg *schema.KeelConfig, threshold int) {
	merged := schema.MergeWithDefaults(cfg)
	var totalSize int64

	// Walk all instruction file refs.
	for _, ref := range merged.Instructions {
		target := ref.Source
		if target == "" {
			continue
		}
		fullPath := filepath.Join(layerPath, target)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		if !info.IsDir() {
			totalSize += info.Size()
			continue
		}

		// Walk directory for markdown files.
		_ = filepath.Walk(fullPath, func(path string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if !fi.IsDir() && strings.HasSuffix(strings.ToLower(fi.Name()), ".md") {
				totalSize += fi.Size()
			}
			return nil
		})
	}

	totalTokens := estimateTokensFromSize(int(totalSize))

	if threshold > 0 {
		if totalTokens > threshold {
			result.addWarn(fmt.Sprintf("token budget: instructions total ~%d tokens (threshold: %d)", totalTokens, threshold))
		} else {
			result.addPass(fmt.Sprintf("token budget: instructions total ~%d tokens (within threshold)", totalTokens))
		}
	} else {
		// Threshold disabled (0) — just report the estimate.
		result.addPass(fmt.Sprintf("token budget: instructions total ~%d tokens", totalTokens))
	}
}

// checkLayerTotal estimates total layer content token count (instructions + commands + agents + skills + rules).
func checkLayerTotal(result *Result, layerPath string, cfg *schema.KeelConfig, threshold int) {
	merged := schema.MergeWithDefaults(cfg)
	var totalSize int64

	// Collect all content categories.
	categories := [][]schema.FileRef{
		merged.Instructions,
		merged.Commands,
		merged.Agents,
		merged.Skills,
		merged.Rules,
	}

	for _, refs := range categories {
		for _, ref := range refs {
			target := ref.Source
			if target == "" {
				continue
			}
			fullPath := filepath.Join(layerPath, target)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}

			if !info.IsDir() {
				totalSize += info.Size()
				continue
			}

			// Walk directory.
			_ = filepath.Walk(fullPath, func(path string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return nil
				}
				if !fi.IsDir() {
					totalSize += fi.Size()
				}
				return nil
			})
		}
	}

	totalTokens := estimateTokensFromSize(int(totalSize))

	if threshold > 0 {
		if totalTokens > threshold {
			result.addWarn(fmt.Sprintf("token budget: layer content total ~%d tokens (threshold: %d)", totalTokens, threshold))
		} else {
			result.addPass(fmt.Sprintf("token budget: layer content total ~%d tokens (within threshold)", totalTokens))
		}
	} else {
		// Threshold disabled (0) — just report the estimate.
		result.addPass(fmt.Sprintf("token budget: layer content total ~%d tokens", totalTokens))
	}
}
