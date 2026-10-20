// Package skills provides Agent Skills validation and security scanning.
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// SecurityFinding represents a single security issue detected in a skill file.
type SecurityFinding struct {
	SkillDir string // directory name (e.g., "security-review")
	Category string // rule category ("env-exfiltration", "shell-injection", etc.)
	Message  string // human-readable message
	Severity string // "block" or "warn"
	Line     int    // line number where pattern was found (0 = unknown)
}

// SecurityRule defines a pattern-based security check.
type SecurityRule struct {
	Category        string         // Category name
	Pattern         *regexp.Regexp // Compiled regex pattern
	Message         string         // Human-readable description
	DefaultSeverity string         // "block" or "warn"
}

// builtInRules defines all default security patterns.
// Most patterns default to "warn" — only the most dangerous default to "block".
var builtInRules = []SecurityRule{
	// env-exfiltration: patterns that access environment variables
	{
		Category:        "env-exfiltration",
		Pattern:         regexp.MustCompile(`(?i)\bprocess\.env\b`),
		Message:         "Accesses environment variables via process.env",
		DefaultSeverity: "warn",
	},
	{
		Category:        "env-exfiltration",
		Pattern:         regexp.MustCompile(`(?i)\bos\.environ\b`),
		Message:         "Accesses environment variables via os.environ",
		DefaultSeverity: "warn",
	},
	{
		Category:        "env-exfiltration",
		Pattern:         regexp.MustCompile(`(?i)\$ENV\{`),
		Message:         "Accesses environment variables via $ENV{}",
		DefaultSeverity: "warn",
	},
	{
		Category:        "env-exfiltration",
		Pattern:         regexp.MustCompile(`(?i)\bSystem\.getenv\b`),
		Message:         "Accesses environment variables via System.getenv",
		DefaultSeverity: "warn",
	},
	{
		Category:        "env-exfiltration",
		Pattern:         regexp.MustCompile(`(?i)\bprintenv\b`),
		Message:         "Accesses environment variables via printenv",
		DefaultSeverity: "warn",
	},
	{
		Category:        "env-exfiltration",
		Pattern:         regexp.MustCompile(`(?i)\benv\s*\|`),
		Message:         "Accesses environment variables via env command",
		DefaultSeverity: "warn",
	},

	// shell-injection: patterns that execute shell commands
	{
		Category:        "shell-injection",
		Pattern:         regexp.MustCompile(`\$\(`),
		Message:         "Shell command substitution via $()",
		DefaultSeverity: "block",
	},
	{
		Category:        "shell-injection",
		Pattern:         regexp.MustCompile("`[^`]*`"),
		Message:         "Shell command substitution via backticks",
		DefaultSeverity: "block",
	},
	{
		Category:        "shell-injection",
		Pattern:         regexp.MustCompile(`(?i)\beval\s*\(`),
		Message:         "Code evaluation via eval()",
		DefaultSeverity: "block",
	},
	{
		Category:        "shell-injection",
		Pattern:         regexp.MustCompile(`(?i)\bexec\s*\(`),
		Message:         "Command execution via exec()",
		DefaultSeverity: "block",
	},
	{
		Category:        "shell-injection",
		Pattern:         regexp.MustCompile(`(?i)\bsystem\s*\(`),
		Message:         "Command execution via system()",
		DefaultSeverity: "block",
	},
	{
		Category:        "shell-injection",
		Pattern:         regexp.MustCompile(`(?i)\bos\.popen\b`),
		Message:         "Command execution via os.popen",
		DefaultSeverity: "block",
	},

	// prompt-injection: patterns that manipulate AI prompts
	{
		Category:        "prompt-injection",
		Pattern:         regexp.MustCompile(`(?i)ignore\s+previous\s+instructions`),
		Message:         "Prompt injection pattern: 'ignore previous instructions'",
		DefaultSeverity: "block",
	},
	{
		Category:        "prompt-injection",
		Pattern:         regexp.MustCompile(`(?i)ignore\s+all\s+previous`),
		Message:         "Prompt injection pattern: 'ignore all previous'",
		DefaultSeverity: "block",
	},
	{
		Category:        "prompt-injection",
		Pattern:         regexp.MustCompile(`(?i)disregard\s+above`),
		Message:         "Prompt injection pattern: 'disregard above'",
		DefaultSeverity: "block",
	},
	{
		Category:        "prompt-injection",
		Pattern:         regexp.MustCompile(`<!--.*(?i)(?:ignore|disregard|override).*-->`),
		Message:         "Hidden instruction in HTML comment",
		DefaultSeverity: "block",
	},
	{
		Category:        "prompt-injection",
		Pattern:         regexp.MustCompile(`[‪-‮⁦-⁩]`),
		Message:         "Unicode direction override (potential steganography)",
		DefaultSeverity: "block",
	},

	// suspicious-urls: patterns that reference potentially dangerous URLs
	{
		Category:        "suspicious-urls",
		Pattern:         regexp.MustCompile(`(?i)\bdata:[^;]*;base64,`),
		Message:         "Data URI with base64 encoding (potential payload)",
		DefaultSeverity: "warn",
	},
	{
		Category:        "suspicious-urls",
		Pattern:         regexp.MustCompile(`(?i)\bfile://`),
		Message:         "Local file URI (file://)",
		DefaultSeverity: "warn",
	},
	{
		Category:        "suspicious-urls",
		Pattern:         regexp.MustCompile(`(?i)https?://(?:10|172\.(?:1[6-9]|2\d|3[01])|192\.168)\.`),
		Message:         "Private IP address in URL",
		DefaultSeverity: "warn",
	},
	{
		Category:        "suspicious-urls",
		Pattern:         regexp.MustCompile(`(?i)https?://(?:\d{1,3}\.){3}\d{1,3}`),
		Message:         "Public IP address in URL",
		DefaultSeverity: "warn",
	},
}

// ScanSkillBody scans a skill's body content (post-frontmatter) for security issues.
// Returns findings sorted by line number. Frontmatter is NOT scanned.
func ScanSkillBody(skillDir string, content []byte, config *schema.SecurityConfig) []SecurityFinding {
	// Extract body content (skip frontmatter)
	body := extractBody(content)
	if len(body) == 0 {
		return nil
	}

	// Determine which rules to apply
	activeRules := filterRules(config)
	if len(activeRules) == 0 {
		return nil
	}

	var findings []SecurityFinding
	lines := strings.Split(string(body), "\n")

	for lineNum, line := range lines {
		for _, rule := range activeRules {
			if rule.Pattern.MatchString(line) {
				severity := rule.DefaultSeverity

				findings = append(findings, SecurityFinding{
					SkillDir: skillDir,
					Category: rule.Category,
					Message:  rule.Message,
					Severity: severity,
					Line:     lineNum + 1, // 1-indexed
				})
			}
		}
	}

	return findings
}

// ScanSkills scans all skill files in a layer for security issues.
// If keelExists is false, uses directory discovery. Otherwise uses keel.yml config.
func ScanSkills(layerPath string, keelExists bool, cfg *schema.KeelConfig, config *schema.SecurityConfig) []SecurityFinding {
	var skillPaths []string

	if keelExists && cfg != nil {
		// Use keel.yml-defined skill directories
		merged := schema.MergeWithDefaults(cfg)
		for _, ref := range merged.Skills {
			if ref.Source == "" {
				continue
			}
			fullPath := filepath.Join(layerPath, ref.Source)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}
			if info.IsDir() {
				// Directory reference — scan all subdirs
				entries, err := os.ReadDir(fullPath)
				if err != nil {
					continue
				}
				for _, entry := range entries {
					if entry.IsDir() {
						skillPaths = append(skillPaths, filepath.Join(fullPath, entry.Name()))
					}
				}
			}
		}
	} else {
		// Native layer — discover skills/ directory
		skillsDir := filepath.Join(layerPath, "skills")
		if info, err := os.Stat(skillsDir); err == nil && info.IsDir() {
			entries, err := os.ReadDir(skillsDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						skillPaths = append(skillPaths, filepath.Join(skillsDir, entry.Name()))
					}
				}
			}
		}
	}

	var allFindings []SecurityFinding
	for _, skillPath := range skillPaths {
		dirName := filepath.Base(skillPath)
		data, err := ReadSkillFile(skillPath)
		if err != nil {
			continue
		}
		findings := ScanSkillBody(dirName, data, config)
		allFindings = append(allFindings, findings...)
	}

	return allFindings
}

// extractBody returns the markdown content after frontmatter.
// If no frontmatter exists, returns the full content.
func extractBody(content []byte) []byte {
	fm, err := merge.ExtractRawFrontmatter(content)
	if err != nil || fm == nil {
		// No frontmatter or parse error — scan full content
		return content
	}

	// Find the end of frontmatter
	lines := strings.Split(string(content), "\n")
	delimiterCount := 0
	bodyStart := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			delimiterCount++
			if delimiterCount == 2 {
				bodyStart = i + 1
				break
			}
		}
	}

	if bodyStart >= len(lines) {
		return []byte{}
	}

	return []byte(strings.Join(lines[bodyStart:], "\n"))
}

// filterRules returns the active rules based on configuration.
// Categories are opt-out: if not explicitly disabled, they're active.
func filterRules(config *schema.SecurityConfig) []SecurityRule {
	var active []SecurityRule

	// Built-in rules (opt-out by category)
	for _, rule := range builtInRules {
		// Check if category is explicitly disabled
		if config != nil {
			switch rule.Category {
			case "env-exfiltration":
				if config.EnvExfiltration != nil && !*config.EnvExfiltration {
					continue
				}
			case "shell-injection":
				if config.ShellInjection != nil && !*config.ShellInjection {
					continue
				}
			case "prompt-injection":
				if config.PromptInjection != nil && !*config.PromptInjection {
					continue
				}
			case "suspicious-urls":
				if config.SuspiciousURLs != nil && !*config.SuspiciousURLs {
					continue
				}
			}
		}
		active = append(active, rule)
	}

	// Custom patterns (always active if defined)
	if config != nil {
		for _, cp := range config.CustomPatterns {
			if cp.Pattern == "" {
				continue
			}
			pattern, err := regexp.Compile(cp.Pattern)
			if err != nil {
				// Invalid regex — skip
				continue
			}
			severity := cp.Severity
			if severity != "block" && severity != "warn" {
				severity = "warn" // default
			}
			active = append(active, SecurityRule{
				Category:        "custom",
				Pattern:         pattern,
				Message:         cp.Message,
				DefaultSeverity: severity,
			})
		}
	}

	return active
}

// FormatFinding formats a SecurityFinding for display.
func FormatFinding(f SecurityFinding) string {
	prefix := "⚠"
	if f.Severity == "block" {
		prefix = "✗"
	}
	location := f.SkillDir
	if f.Line > 0 {
		location = fmt.Sprintf("%s:%d", f.SkillDir, f.Line)
	}
	return fmt.Sprintf("%s [%s] %s: %s", prefix, f.Category, location, f.Message)
}
