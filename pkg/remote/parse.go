// Package remote handles classification, parsing, and resolution of remote
// layer sources (Git repos, HTTPS archives) for the Keel compiler.
package remote

import (
	"fmt"
	"strings"
)

// SourceType classifies a remote source.
type SourceType int

const (
	// SourceLocal indicates a local filesystem path (not a URL).
	SourceLocal SourceType = iota
	// SourceGit indicates a Git repository (HTTPS or SSH).
	SourceGit
	// SourceArchive indicates an HTTPS archive (.tar.gz, .tgz, .zip).
	SourceArchive
)

// String returns a human-readable name for the source type.
func (s SourceType) String() string {
	switch s {
	case SourceLocal:
		return "local"
	case SourceGit:
		return "git"
	case SourceArchive:
		return "archive"
	default:
		return "unknown"
	}
}

// ParsedSource holds the parsed components of a layer source reference.
type ParsedSource struct {
	Type     SourceType
	RawURL   string // URL without fragment
	Branch   string // Git ref (empty = default branch)
	SubPath  string // Path within repo/archive (empty = root)
	Original string // Original input string
}

// Classify examines a layer segment and returns its source type.
// It does not validate the URL or return errors -- only detects patterns.
func Classify(segment string) SourceType {
	// SSH scp-style: git@host:path
	if strings.HasPrefix(segment, "git@") {
		return SourceGit
	}

	// SSH protocol: ssh://
	if strings.HasPrefix(segment, "ssh://") {
		return SourceGit
	}

	// HTTP(S) URLs
	lower := strings.ToLower(segment)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		// Strip fragment for classification purposes
		urlPart := segment
		if idx := strings.Index(urlPart, "#"); idx >= 0 {
			urlPart = urlPart[:idx]
		}
		lowerURL := strings.ToLower(urlPart)

		// Check for .git in the URL path -> Git
		if strings.Contains(lowerURL, ".git") {
			return SourceGit
		}

		// Check for archive extensions
		if strings.HasSuffix(lowerURL, ".tar.gz") ||
			strings.HasSuffix(lowerURL, ".tgz") ||
			strings.HasSuffix(lowerURL, ".zip") {
			return SourceArchive
		}
	}

	return SourceLocal
}

// Parse parses a layer segment into its structured components.
// Returns a ParsedSource with Type == SourceLocal for non-URL segments.
// Returns an error for http:// URLs (HTTPS is required for non-Git sources).
func Parse(segment string) (ParsedSource, error) {
	// Reject ext:: remote helper protocol — allows arbitrary command execution.
	// Checked early as defense-in-depth before Classify or any URL handling.
	if strings.HasPrefix(segment, "ext::") {
		return ParsedSource{}, fmt.Errorf("rejected dangerous git remote helper URL: %s", segment)
	}

	st := Classify(segment)

	if st == SourceLocal {
		return ParsedSource{
			Type:     SourceLocal,
			Original: segment,
		}, nil
	}

	// Reject http:// URLs (require https://)
	lower := strings.ToLower(segment)
	if strings.HasPrefix(lower, "http://") {
		return ParsedSource{}, fmt.Errorf("insecure http:// URL not allowed: %q (use https://)", segment)
	}

	switch st {
	case SourceGit:
		return parseGit(segment)
	case SourceArchive:
		return parseArchive(segment)
	default:
		return ParsedSource{Type: SourceLocal, Original: segment}, nil
	}
}

// parseGit parses a Git URL with optional fragment (#branch:path).
func parseGit(segment string) (ParsedSource, error) {
	ps := ParsedSource{
		Type:     SourceGit,
		Original: segment,
	}

	// Split on first '#' to separate URL from fragment
	rawURL := segment
	fragment := ""
	if idx := strings.Index(segment, "#"); idx >= 0 {
		rawURL = segment[:idx]
		fragment = segment[idx+1:]
	}

	ps.RawURL = rawURL

	// Parse fragment: branch:path
	// - No fragment         -> Branch="", SubPath=""
	// - "#main"             -> Branch="main", SubPath=""
	// - "#main:layers/base" -> Branch="main", SubPath="layers/base"
	// - "#:layers/base"     -> Branch="", SubPath="layers/base"
	if fragment != "" {
		// Split on first ':' within the fragment
		if colonIdx := strings.Index(fragment, ":"); colonIdx >= 0 {
			ps.Branch = fragment[:colonIdx]
			ps.SubPath = fragment[colonIdx+1:]
		} else {
			ps.Branch = fragment
		}
	}

	return ps, nil
}

// parseArchive parses an HTTPS archive URL.
func parseArchive(segment string) (ParsedSource, error) {
	return ParsedSource{
		Type:     SourceArchive,
		RawURL:   segment,
		Original: segment,
	}, nil
}

// SplitLayerSegments splits a colon-separated layer string into individual
// segments, correctly handling URLs that contain colons (HTTPS, SSH scp-style,
// and fragment syntax).
//
// The key rules:
//   - https:// and http:// consume everything through the URL (including fragment)
//   - ssh:// consumes everything through the URL (including fragment)
//   - git@ triggers SSH scp-style parsing: git@host:path[#fragment]
//   - Colons after '#' in a fragment belong to the fragment, not segment separators
//   - Empty and whitespace-only segments are dropped
func SplitLayerSegments(input string) []string {
	if input == "" {
		return nil
	}

	var segments []string
	i := 0
	n := len(input)

	for i < n {
		// Skip leading colons (empty segments)
		if input[i] == ':' {
			i++
			continue
		}

		segStart := i

		// Detect what kind of segment starts here
		remaining := input[i:]

		if looksLikeSchemeURL(remaining) {
			// Consume a scheme-based URL: https://, http://, ssh://
			i = consumeSchemeURL(input, i)
		} else if looksLikeSSHScp(remaining) {
			// Consume an SSH scp-style URL: git@host:org/repo[#fragment]
			i = consumeSSHScp(input, i)
		} else {
			// Consume a plain local path (until next colon)
			i = consumeLocalPath(input, i)
		}

		seg := strings.TrimSpace(input[segStart:i])
		if seg != "" {
			segments = append(segments, seg)
		}
	}

	return segments
}

// looksLikeSchemeURL returns true if s starts with https://, http://, or ssh://.
func looksLikeSchemeURL(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "ssh://")
}

// looksLikeSSHScp returns true if s starts with a git@ SSH scp-style pattern.
// Pattern: user@host:path (where user contains only alnum/dot/dash/underscore,
// and the segment does not start with '/').
func looksLikeSSHScp(s string) bool {
	if !strings.HasPrefix(s, "git@") {
		return false
	}
	// Must have a colon after the host portion
	rest := s[4:] // after "git@"
	colonIdx := strings.Index(rest, ":")
	if colonIdx <= 0 {
		// No colon or colon immediately after @, not a valid scp-style URL
		return false
	}
	return true
}

// consumeSchemeURL advances past a scheme-based URL (https://, http://, ssh://).
// The URL continues until we hit a colon that starts a new segment. A colon
// that is part of the scheme (://) or inside a fragment (#branch:path) is not
// a separator.
func consumeSchemeURL(input string, start int) int {
	i := start

	// Skip past the scheme (e.g., "https://")
	schemeEnd := strings.Index(input[i:], "://")
	if schemeEnd < 0 {
		// Should not happen given looksLikeSchemeURL guard, but be safe
		return consumeLocalPath(input, start)
	}
	i += schemeEnd + 3 // past "://"

	// Now consume the rest of the URL. We need to track whether we are
	// inside a fragment (after #). Colons inside fragments are part of the
	// fragment, not separators.
	inFragment := false
	fragmentColonSeen := false

	for i < len(input) {
		ch := input[i]

		if ch == '#' {
			inFragment = true
			fragmentColonSeen = false
			i++
			continue
		}

		if ch == ':' {
			if !inFragment {
				// Colon outside fragment: this is a segment separator.
				// The URL ends here.
				break
			}
			// Inside fragment: first colon separates branch from path.
			// After the first colon in the fragment, subsequent characters
			// are the subpath. We need to decide when the subpath ends and
			// a new segment begins.
			if !fragmentColonSeen {
				// This is the branch:path separator within the fragment.
				fragmentColonSeen = true
				i++
				continue
			}
			// Second colon after fragment: this could be either part of
			// the subpath or a new segment boundary. We look ahead to
			// determine if what follows looks like a new segment (a path
			// or URL) rather than a continuation of the subpath.
			if i+1 < len(input) && looksLikeNewSegment(input[i+1:]) {
				break
			}
			i++
			continue
		}

		i++
	}

	return i
}

// consumeSSHScp advances past an SSH scp-style URL (git@host:org/repo[#fragment]).
// The first colon after git@host is part of the SSH URL (host:path separator).
// Subsequent colons are either part of a fragment or segment separators.
func consumeSSHScp(input string, start int) int {
	i := start

	// Find the host:path colon (first colon after "git@")
	rest := input[i+4:] // after "git@"
	hostColonIdx := strings.Index(rest, ":")
	if hostColonIdx < 0 {
		// No colon at all, consume to end
		return len(input)
	}
	// Advance past git@host:
	i += 4 + hostColonIdx + 1

	// Now we are in the path portion. Consume until we hit a '#' (fragment)
	// or a ':' that starts a new segment.
	inFragment := false
	fragmentColonSeen := false

	for i < len(input) {
		ch := input[i]

		if ch == '#' {
			inFragment = true
			fragmentColonSeen = false
			i++
			continue
		}

		if ch == ':' {
			if !inFragment {
				// Colon outside fragment in the path portion:
				// this is a new segment separator.
				break
			}
			if !fragmentColonSeen {
				// First colon inside fragment: branch:path separator.
				fragmentColonSeen = true
				i++
				continue
			}
			// Subsequent colon inside fragment: check if new segment.
			if i+1 < len(input) && looksLikeNewSegment(input[i+1:]) {
				break
			}
			i++
			continue
		}

		i++
	}

	return i
}

// consumeLocalPath advances past a simple local path segment (no URL).
// Consumes until the next colon.
func consumeLocalPath(input string, start int) int {
	idx := strings.Index(input[start:], ":")
	if idx < 0 {
		return len(input)
	}
	return start + idx
}

// looksLikeNewSegment returns true if the text after a colon looks like a new
// layer segment rather than a continuation of the current fragment subpath.
// Only called for the second colon after a '#' fragment marker (the first
// colon is always the branch/path separator).
//
// In practice, colons almost never appear in directory or file names used as
// fragment subpaths. The colon-separated layer syntax (local/a:git@host:...)
// means a second colon after the fragment's branch:path separator reliably
// indicates a new segment boundary. The only exception is an empty remainder
// (trailing colon), which is not a new segment.
//
// If colons-in-subpath support becomes necessary, this function would need
// to distinguish bare path components from layer references — but that
// ambiguity is inherent in the colon-separated syntax and would require an
// escape mechanism rather than heuristic detection.
func looksLikeNewSegment(s string) bool {
	return len(s) > 0
}
