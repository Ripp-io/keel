package compiler

import "strings"

// allowedEnvVars lists environment variable names that are safe to pass
// through to dependency install commands. Using an allowlist (rather than
// blocklist) provides defense-in-depth — unknown variables are excluded
// by default, preventing accidental credential leakage.
var allowedEnvVars = map[string]bool{
	// System and user identity
	"HOME":    true,
	"USER":    true,
	"LOGNAME": true,
	"PATH":    true, // Modified with targetDir prepended
	"TMPDIR":  true,
	"TEMP":    true,
	"TMP":     true,
	"SHELL":   true,

	// Locale settings
	"LANG":     true,
	"LC_ALL":   true,
	"LC_CTYPE": true,

	// Go toolchain
	"GOPATH":        true,
	"GOROOT":        true,
	"GOBIN":         true,
	"GOPROXY":       true,
	"GONOSUMCHECK":  true,
	"GOFLAGS":       true,
	"GONOPROXY":     true,
	"GONOSUMDB":     true,
	"GOPRIVATE":     true,

	// Node.js toolchain
	"NPM_CONFIG_PREFIX": true,
	"NODE_PATH":         true,

	// Rust toolchain
	"CARGO_HOME":  true,
	"RUSTUP_HOME": true,

	// Python toolchain
	"PIP_INDEX_URL":            true,
	"PIPENV_VENV_IN_PROJECT":   true,

	// Proxy configuration
	"HTTP_PROXY":  true,
	"HTTPS_PROXY": true,
	"NO_PROXY":    true,
	"http_proxy":  true,
	"https_proxy": true,
	"no_proxy":    true,

	// TLS configuration
	"SSL_CERT_FILE": true,
	"SSL_CERT_DIR":  true,
	"CURL_CA_BUNDLE": true,

	// XDG Base Directory Specification
	"XDG_CACHE_HOME":  true,
	"XDG_CONFIG_HOME": true,
	"XDG_DATA_HOME":   true,
}

// sanitizeEnv returns a filtered copy of env containing only safe environment
// variables from the allowlist. This prevents layer-controlled install commands
// from accessing API keys, tokens, and credentials. Variables starting with
// KEEL_ are also allowed through.
func sanitizeEnv(env []string) []string {
	result := make([]string, 0, len(env))
	for _, e := range env {
		if isAllowedEnvVar(e) {
			result = append(result, e)
		}
	}
	return result
}

// isAllowedEnvVar returns true if the environment variable entry (in
// "KEY=value" format) is on the allowlist or matches an allowed prefix.
// The check is case-sensitive to match Go's os.Environ() behavior.
func isAllowedEnvVar(entry string) bool {
	eqIdx := strings.IndexByte(entry, '=')
	if eqIdx < 0 {
		return false
	}
	key := entry[:eqIdx]

	// Check explicit allowlist
	if allowedEnvVars[key] {
		return true
	}

	// Allow KEEL_* variables (Keel-specific configuration)
	if strings.HasPrefix(key, "KEEL_") {
		return true
	}

	return false
}
