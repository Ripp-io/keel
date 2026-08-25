package validate

import (
	"bytes"
	"regexp"
	"strings"
)

// lintWarning describes a non-portable body-level pattern found in a markdown file.
type lintWarning struct {
	File    string // filename where the pattern was found
	Line    int    // 1-based line number
	Pattern string // short identifier for the pattern (e.g., "$ARGUMENTS", "{{var}}")
	Agent   string // "claude" or "cursor" — the agent this pattern is specific to
	Message string // human-readable description
}

// bodyPattern defines a single non-portable pattern to detect.
type bodyPattern struct {
	regex   *regexp.Regexp
	agent   string
	pattern string
	message string
}

// bodyPatterns is the ordered table of non-portable patterns.
// Order matters: more specific patterns must come before less specific ones
// to allow suppression of double-matches (e.g., $ARGUMENTS[N] before $ARGUMENTS).
var bodyPatterns = []bodyPattern{
	{regexp.MustCompile(`\$ARGUMENTS\[\d+\]`), "claude", "$ARGUMENTS[N]", "Claude-only indexed argument substitution"},
	{regexp.MustCompile(`\$ARGUMENTS`), "claude", "$ARGUMENTS", "Claude-only argument substitution"},
	// Match $0 through $9 only — single digit avoids false positives on dollar amounts ($100, $50k).
	{regexp.MustCompile(`\$[0-9](?:[^0-9]|$)`), "claude", "$N", "Claude-only positional argument"},
	{regexp.MustCompile("!`[^`]+`"), "claude", "!`cmd`", "Claude-only inline shell execution"},
	{regexp.MustCompile("^```!"), "claude", "```!", "Claude-only fenced shell execution"},
	{regexp.MustCompile(`\$\{CLAUDE_SKILL_DIR\}`), "claude", "${CLAUDE_SKILL_DIR}", "Claude-only skill directory variable"},
	{regexp.MustCompile(`\$\{CLAUDE_SESSION_ID\}`), "claude", "${CLAUDE_SESSION_ID}", "Claude-only session ID variable"},
	{regexp.MustCompile(`\$\{CLAUDE_EFFORT\}`), "claude", "${CLAUDE_EFFORT}", "Claude-only effort level variable"},
	{regexp.MustCompile(`\{\{[^}]+\}\}`), "cursor", "{{var}}", "Cursor-only Maestro template variable"},
}

// lintBodyPatterns scans markdown content for non-portable body-level patterns.
// Returns a slice of warnings (never errors). Each warning identifies the pattern
// found, the line number, and which agent it belongs to.
func lintBodyPatterns(content []byte, filename string) []lintWarning {
	if len(content) == 0 {
		return nil
	}

	lines := bytes.Split(content, []byte("\n"))
	var warnings []lintWarning

	inCodeBlock := false
	var fencePrefix string // the backtick sequence that opened the current fence
	fenceLen := 0

	for i, line := range lines {
		lineNum := i + 1
		lineStr := string(line)
		trimmed := strings.TrimSpace(lineStr)

		// Track fenced code blocks. A fence opener is 3+ backticks at the
		// start of a line (possibly with a language tag). A fence closer is
		// the same number of backticks (or more) with no other content.
		//
		// Special case: ```! is a Claude-only execution block, not a regular
		// code fence. We flag it but do NOT enter code-block mode for it.
		if !inCodeBlock {
			if ticks, isFence := parseFenceOpener(trimmed); isFence {
				// Check if this is a ```! pattern (Claude shell execution).
				if strings.HasPrefix(trimmed, "```!") {
					// Flag it as a pattern match but don't enter code block mode.
					// The content inside is executable, not documentation.
					// We still check for other patterns on this line below.
				} else {
					inCodeBlock = true
					fencePrefix = ticks
					fenceLen = len(ticks)
					continue
				}
			}
		} else {
			// Check for fence closer: same or more backticks, nothing else.
			if ticks, isFence := parseFenceOpener(trimmed); isFence && len(ticks) >= fenceLen {
				// Closer must be only backticks (no language tag).
				if trimmed == ticks || trimmed == fencePrefix {
					inCodeBlock = false
					fencePrefix = ""
					fenceLen = 0
				}
			}
			continue // skip lines inside code blocks
		}

		// Check each pattern against this line.
		matched := matchLine(lineStr, lineNum, filename)
		warnings = append(warnings, matched...)
	}

	return warnings
}

// matchLine checks a single line against all body patterns, suppressing
// double-matches (e.g., $ARGUMENTS[0] should match $ARGUMENTS[N] but not $ARGUMENTS).
func matchLine(line string, lineNum int, filename string) []lintWarning {
	var warnings []lintWarning

	// Track byte ranges already matched by more specific patterns
	// to prevent double-counting.
	type matchRange struct{ start, end int }
	var consumed []matchRange

	for _, bp := range bodyPatterns {
		locs := bp.regex.FindAllStringIndex(line, -1)
		for _, loc := range locs {
			// Check if this match overlaps with an already-consumed range.
			overlaps := false
			for _, c := range consumed {
				if loc[0] < c.end && loc[1] > c.start {
					overlaps = true
					break
				}
			}
			if overlaps {
				continue
			}

			consumed = append(consumed, matchRange{loc[0], loc[1]})
			warnings = append(warnings, lintWarning{
				File:    filename,
				Line:    lineNum,
				Pattern: bp.pattern,
				Agent:   bp.agent,
				Message: bp.message,
			})
		}
	}

	return warnings
}

// parseFenceOpener checks if a trimmed line starts with 3+ backticks.
// Returns the backtick prefix and true if it's a fence opener/closer.
func parseFenceOpener(trimmed string) (string, bool) {
	if len(trimmed) < 3 {
		return "", false
	}
	if trimmed[0] != '`' {
		return "", false
	}

	i := 0
	for i < len(trimmed) && trimmed[i] == '`' {
		i++
	}
	if i < 3 {
		return "", false
	}

	return trimmed[:i], true
}
