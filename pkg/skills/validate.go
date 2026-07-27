// Package skills provides Agent Skills (agentskills.io) validation and generation.
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"golang.org/x/text/unicode/norm"
)

// Warning represents a single Agent Skills validation finding.
type Warning struct {
	SkillDir string // directory name (e.g., "security-review")
	Field    string // which field ("name", "description", "frontmatter", "compatibility", "metadata")
	Message  string // human-readable message
}

// allowedFields is the set of frontmatter keys permitted by the Agent Skills
// spec plus Keel governance fields. This map must not be modified after init.
var allowedFields = map[string]bool{
	// Agent Skills spec fields
	"name":          true,
	"description":   true,
	"license":       true,
	"allowed-tools": true,
	"metadata":      true,
	"compatibility": true,
	// Claude Code extension fields
	"disable-model-invocation": true,
	"user-invocable":           true,
	"context":                  true,
	"agent":                    true,
	"hooks":                    true,
	"paths":                    true,
	"arguments":                true,
	"argument-hint":            true,
	"when_to_use":              true,
	"model":                    true,
	"effort":                   true,
	"shell":                    true,
	// Keel governance fields (tolerated, not warned)
	"force": true,
	"scope": true,
	"merge": true,
}

// ValidateDir validates a single skill directory against the Agent Skills spec.
// Returns warnings (empty slice = fully compliant). Never returns errors for
// validation failures — all findings are warnings.
func ValidateDir(skillDirPath string) []Warning {
	dirName := filepath.Base(skillDirPath)

	// Rule 1: SKILL.md must exist
	data, err := readSkillFile(skillDirPath)
	if err != nil {
		return []Warning{{
			SkillDir: dirName,
			Field:    "frontmatter",
			Message:  "SKILL.md (or skill.md) not found",
		}}
	}

	// Rule 2: Frontmatter must be parseable
	fm, err := merge.ExtractRawFrontmatter(data)
	if err != nil {
		return []Warning{{
			SkillDir: dirName,
			Field:    "frontmatter",
			Message:  fmt.Sprintf("frontmatter parse error: %v", err),
		}}
	}
	if fm == nil {
		return []Warning{{
			SkillDir: dirName,
			Field:    "frontmatter",
			Message:  "no frontmatter found",
		}}
	}

	// Run remaining checks
	return validateFrontmatter(fm, dirName)
}

// ValidateAll validates all subdirectories under a skills/ parent directory.
// Each immediate subdirectory is treated as a skill directory.
func ValidateAll(skillsDirPath string) []Warning {
	entries, err := os.ReadDir(skillsDirPath)
	if err != nil {
		return nil
	}

	var all []Warning
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(skillsDirPath, entry.Name())
		all = append(all, ValidateDir(dirPath)...)
	}
	return all
}

// readSkillFile reads SKILL.md (preferred) or skill.md from the given directory.
// Returns the file contents or an error if neither exists.
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

	return nil, fmt.Errorf("neither SKILL.md nor skill.md found in %q", dirPath)
}

// validateFrontmatter runs rules 3-12 against a parsed frontmatter map.
func validateFrontmatter(fm map[string]any, dirName string) []Warning {
	var warnings []Warning

	// Name checks: if required check fails, skip format and match checks
	// since they would produce redundant noise.
	nameWarnings := checkNameRequired(fm, dirName)
	if len(nameWarnings) > 0 {
		warnings = append(warnings, nameWarnings...)
	} else {
		warnings = append(warnings, checkNameMaxLen(fm, dirName)...)
		warnings = append(warnings, checkNameFormat(fm, dirName)...)
		warnings = append(warnings, checkNameMatchesDir(fm, dirName)...)
	}

	// Description checks
	descWarnings := checkDescriptionRequired(fm, dirName)
	if len(descWarnings) > 0 {
		warnings = append(warnings, descWarnings...)
	} else {
		warnings = append(warnings, checkDescriptionMaxLen(fm, dirName)...)
	}

	warnings = append(warnings, checkCompatibilityMaxLen(fm, dirName)...)
	warnings = append(warnings, checkUnexpectedFields(fm, dirName)...)
	warnings = append(warnings, checkMetadataType(fm, dirName)...)

	return warnings
}

// checkNameRequired verifies that "name" exists and is a non-empty string.
func checkNameRequired(fm map[string]any, dirName string) []Warning {
	v, ok := fm["name"]
	if !ok {
		return []Warning{{SkillDir: dirName, Field: "name", Message: "required field \"name\" is missing"}}
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return []Warning{{SkillDir: dirName, Field: "name", Message: "\"name\" must be a non-empty string"}}
	}
	return nil
}

// checkNameMaxLen verifies that the name does not exceed 64 characters after NFKC normalization.
func checkNameMaxLen(fm map[string]any, dirName string) []Warning {
	name, ok := getStringField(fm, "name")
	if !ok {
		return nil // handled by checkNameRequired
	}
	normalized := norm.NFKC.String(name)
	if utf8.RuneCountInString(normalized) > 64 {
		return []Warning{{SkillDir: dirName, Field: "name", Message: "\"name\" exceeds maximum of 64 characters"}}
	}
	return nil
}

// checkNameFormat validates that the name matches the Agent Skills naming pattern:
// lowercase alphanumeric + hyphens, no leading/trailing/consecutive hyphens.
// Unicode lowercase letters are also accepted per the Agent Skills i18n support.
func checkNameFormat(fm map[string]any, dirName string) []Warning {
	name, ok := getStringField(fm, "name")
	if !ok {
		return nil
	}
	normalized := norm.NFKC.String(name)
	if !isValidSkillName(normalized) {
		return []Warning{{
			SkillDir: dirName,
			Field:    "name",
			Message:  fmt.Sprintf("\"name\" %q does not match required format (lowercase alphanumeric + hyphens, no leading/trailing/consecutive hyphens)", name),
		}}
	}
	return nil
}

// checkNameMatchesDir verifies that the NFKC-normalized name matches the
// NFKC-normalized directory basename.
func checkNameMatchesDir(fm map[string]any, dirName string) []Warning {
	name, ok := getStringField(fm, "name")
	if !ok {
		return nil
	}
	normalizedName := norm.NFKC.String(name)
	normalizedDir := norm.NFKC.String(dirName)
	if normalizedName != normalizedDir {
		return []Warning{{
			SkillDir: dirName,
			Field:    "name",
			Message:  fmt.Sprintf("\"name\" %q does not match directory name %q", name, dirName),
		}}
	}
	return nil
}

// checkDescriptionRequired verifies that "description" exists and is a non-empty string.
func checkDescriptionRequired(fm map[string]any, dirName string) []Warning {
	v, ok := fm["description"]
	if !ok {
		return []Warning{{SkillDir: dirName, Field: "description", Message: "required field \"description\" is missing"}}
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return []Warning{{SkillDir: dirName, Field: "description", Message: "\"description\" must be a non-empty string"}}
	}
	return nil
}

// checkDescriptionMaxLen verifies that the description does not exceed 1024 characters (rune count).
// Note: unlike name, the Agent Skills spec does not require NFKC normalization for description length.
func checkDescriptionMaxLen(fm map[string]any, dirName string) []Warning {
	desc, ok := getStringField(fm, "description")
	if !ok {
		return nil
	}
	if utf8.RuneCountInString(desc) > 1024 {
		return []Warning{{SkillDir: dirName, Field: "description", Message: "\"description\" exceeds maximum of 1024 characters"}}
	}
	return nil
}

// checkCompatibilityMaxLen verifies that the compatibility field, if present,
// does not exceed 500 characters (rune count).
func checkCompatibilityMaxLen(fm map[string]any, dirName string) []Warning {
	v, ok := fm["compatibility"]
	if !ok {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil // type mismatch would be caught elsewhere if needed
	}
	if utf8.RuneCountInString(s) > 500 {
		return []Warning{{SkillDir: dirName, Field: "compatibility", Message: "\"compatibility\" exceeds maximum of 500 characters"}}
	}
	return nil
}

// checkUnexpectedFields reports any frontmatter keys that are not in the allowed set.
func checkUnexpectedFields(fm map[string]any, dirName string) []Warning {
	var unexpected []string
	for key := range fm {
		if !allowedFields[key] {
			unexpected = append(unexpected, key)
		}
	}
	if len(unexpected) == 0 {
		return nil
	}
	sort.Strings(unexpected)
	return []Warning{{
		SkillDir: dirName,
		Field:    "frontmatter",
		Message:  fmt.Sprintf("unexpected field(s): %s", strings.Join(unexpected, ", ")),
	}}
}

// checkMetadataType verifies that the "metadata" value, if present, is a YAML mapping.
func checkMetadataType(fm map[string]any, dirName string) []Warning {
	v, ok := fm["metadata"]
	if !ok {
		return nil
	}
	if _, ok := v.(map[string]any); !ok {
		return []Warning{{
			SkillDir: dirName,
			Field:    "metadata",
			Message:  "\"metadata\" must be a YAML mapping (key-value pairs), not a scalar or list",
		}}
	}
	return nil
}

// isValidSkillName checks whether a NFKC-normalized name follows the Agent Skills
// naming convention: lowercase letters (including Unicode) or digits separated
// by single hyphens, with no leading or trailing hyphens.
func isValidSkillName(name string) bool {
	if name == "" {
		return false
	}

	runes := []rune(name)

	// No leading or trailing hyphens
	if runes[0] == '-' || runes[len(runes)-1] == '-' {
		return false
	}

	prevHyphen := false
	for _, r := range runes {
		if r == '-' {
			if prevHyphen {
				return false // consecutive hyphens
			}
			prevHyphen = true
			continue
		}
		prevHyphen = false

		// Accept Unicode lowercase letters and digits
		if unicode.IsDigit(r) {
			continue
		}
		if unicode.IsLetter(r) && unicode.IsLower(r) {
			continue
		}
		return false
	}
	return true
}

// getStringField safely extracts a string value from the frontmatter map.
// Returns ("", false) if the key doesn't exist or the value is not a string.
func getStringField(fm map[string]any, key string) (string, bool) {
	v, ok := fm[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}
