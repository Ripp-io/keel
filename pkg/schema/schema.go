// Package schema defines the keel.yml DSL types and parsing.
package schema

import (
	"fmt"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
	"gopkg.in/yaml.v3"
)

// KeelConfig represents a parsed keel.yml source layer (DSL input).
// It contains only the fields that authors write in configuration layers.
// For the compiled output format, see manifest.CompiledManifest.
type KeelConfig struct {
	Schema          string                `yaml:"schema,omitempty"`
	Name            string                `yaml:"name,omitempty"`
	Version         string                `yaml:"version,omitempty"`
	Description     string                `yaml:"description,omitempty"`
	Instructions    []FileRef             `yaml:"instructions,omitempty"`
	Permissions     *PermissionsConfig    `yaml:"permissions,omitempty"`
	MCP             *MCPConfig            `yaml:"mcp,omitempty"`
	Commands        []FileRef             `yaml:"commands,omitempty"`
	Agents          []FileRef             `yaml:"agents,omitempty"`
	Skills          []FileRef             `yaml:"skills,omitempty"`
	Hooks           []FileRef             `yaml:"hooks,omitempty"`
	Plugins         []FileRef             `yaml:"plugins,omitempty"`
	AgentMemory     []FileRef             `yaml:"agent-memory,omitempty"`
	Environment     []FileRef             `yaml:"environment,omitempty"`
	Rules           []FileRef             `yaml:"rules,omitempty"`
	Init            []FileRef             `yaml:"init,omitempty"`
	Snippets        []FileRef             `yaml:"snippets,omitempty"`
	Bin             []FileRef             `yaml:"bin,omitempty"`
	Loop            []FileRef             `yaml:"loop,omitempty"`
	WorktreeInclude []FileRef             `yaml:"worktree-include,omitempty"`
	Deps            map[string]DepSpec    `yaml:"deps,omitempty"`
	Settings        *Settings             `yaml:"settings,omitempty"`
	Variables       map[string]string     `yaml:"variables,omitempty"`
	Repos           map[string]Repo       `yaml:"repos,omitempty"`
	Files           map[string]FilePolicy `yaml:"files,omitempty"`
	ValidateOpts    *ValidateOptions      `yaml:"validate,omitempty"`
}

// TokenBudgetConfig controls token budget estimation thresholds.
// nil values use defaults; 0 disables the check.
type TokenBudgetConfig struct {
	SkillBody    *int `yaml:"skill-body,omitempty"`   // per-skill body token threshold (default: 5000)
	Instructions *int `yaml:"instructions,omitempty"` // total instruction tokens (default: 0 = disabled)
	Layer        *int `yaml:"layer,omitempty"`        // total layer content tokens (default: 0 = disabled)
}

// ValidateOptions controls validation behavior.
type ValidateOptions struct {
	AgentSkills *bool              `yaml:"agentskills,omitempty"`
	TokenBudget *TokenBudgetConfig `yaml:"token-budget,omitempty"`
	AgentTools  *int               `yaml:"agent-tools,omitempty"`
}

// FilePolicy defines per-file governance in v1-claude / v1-cursor layers.
type FilePolicy struct {
	Force bool   `yaml:"force"`
	Merge string `yaml:"merge,omitempty"`
}

// FileRef references a file or directory with optional policy.
// Used for instructions, commands, agents, skills, init, and permissions.
type FileRef struct {
	Source string `yaml:"source"`
	Force  bool   `yaml:"force"`
}

// MCPConfig holds MCP server definitions and governance policy.
type MCPConfig struct {
	Servers []FileRef  `yaml:"servers,omitempty"`
	Policy  *MCPPolicy `yaml:"policy,omitempty"`
}

// MCPPolicy controls which MCP servers are allowed to run.
// Based on Claude Code's settings.json MCP governance keys.
type MCPPolicy struct {
	Allowed   []string `yaml:"allowed,omitempty"`   // allowlist of server names
	Denied    []string `yaml:"denied,omitempty"`    // denylist of server names
	EnableAll *bool    `yaml:"enableAll,omitempty"` // enable all project servers without prompt
}

// PermissionsConfig holds permission rule files and merge policy.
type PermissionsConfig struct {
	Rules  []FileRef          `yaml:"rules,omitempty"`
	Policy *PermissionsPolicy `yaml:"policy,omitempty"`
}

// PermissionsPolicy controls how permissions merge across layers.
type PermissionsPolicy struct {
	Merge string `yaml:"merge,omitempty"` // "additive" or "replace" (default: replace)
}

// VoiceConfig controls voice input.
type VoiceConfig struct {
	Enabled    *bool  `yaml:"enabled,omitempty"     json:"enabled,omitempty"`
	Mode       string `yaml:"mode,omitempty"        json:"mode,omitempty"`
	AutoSubmit *bool  `yaml:"auto-submit,omitempty" json:"autoSubmit,omitempty"`
}

// SSHConfigEntry defines an SSH remote connection.
type SSHConfigEntry struct {
	ID              string `yaml:"id,omitempty"               json:"id,omitempty"`
	Name            string `yaml:"name,omitempty"             json:"name,omitempty"`
	SSHHost         string `yaml:"ssh-host,omitempty"         json:"sshHost,omitempty"`
	SSHPort         *int   `yaml:"ssh-port,omitempty"         json:"sshPort,omitempty"`
	SSHIdentityFile string `yaml:"ssh-identity-file,omitempty" json:"sshIdentityFile,omitempty"`
	StartDirectory  string `yaml:"start-directory,omitempty"  json:"startDirectory,omitempty"`
}

// FileSuggestionConfig controls file suggestion behavior.
type FileSuggestionConfig struct {
	Type    string `yaml:"type,omitempty"    json:"type,omitempty"`
	Command string `yaml:"command,omitempty" json:"command,omitempty"`
}

// StatusLineConfig controls status line display.
type StatusLineConfig struct {
	Type            string `yaml:"type,omitempty"             json:"type,omitempty"`
	Command         string `yaml:"command,omitempty"          json:"command,omitempty"`
	Padding         *int   `yaml:"padding,omitempty"          json:"padding,omitempty"`
	RefreshInterval *int   `yaml:"refresh-interval,omitempty" json:"refreshInterval,omitempty"`
}

// SpinnerVerbsConfig controls spinner verb display.
type SpinnerVerbsConfig struct {
	Mode  string   `yaml:"mode,omitempty"  json:"mode,omitempty"`
	Verbs []string `yaml:"verbs,omitempty" json:"verbs,omitempty"`
}

// SpinnerTipsOverrideConfig controls spinner tips.
type SpinnerTipsOverrideConfig struct {
	ExcludeDefault *bool    `yaml:"exclude-default,omitempty" json:"excludeDefault,omitempty"`
	Tips           []string `yaml:"tips,omitempty"            json:"tips,omitempty"`
}

// Settings holds ALL keel.yml inline settings fields. All fields are portable —
// they are available to every agent adapter. Writers ignore keys they don't
// recognize. Agent-specific settings go in separate JSON files referenced by
// the Claude/Cursor path fields.
//
// The json tags define camelCase output key names matching Claude's settings.json.
// Cursor maps these keys through its own transform. Every field MUST have a json
// tag — the schema_test.go TestSettingsJSONTags test validates this.
//
// To add a new setting: add a struct field with yaml + json tags. If it needs
// special handling during extraction (like Sandbox and Attribution, which are
// promoted to dedicated IR fields), update extractPortableSettings in reader_dsl.go.
type Settings struct {
	// ============================================================================
	// PORTABLE SETTINGS (work across all agents)
	// ============================================================================
	Model  string `yaml:"model,omitempty"  json:"model,omitempty"`
	Claude string `yaml:"claude,omitempty" json:"claude,omitempty"` // path to settings.claude.json
	Cursor string `yaml:"cursor,omitempty" json:"cursor,omitempty"` // path to settings.cursor.json

	// ============================================================================
	// AGENT/MODEL CONFIGURATION
	// ============================================================================
	Agent                 string            `yaml:"agent,omitempty"                  json:"agent,omitempty"`
	AvailableModels       []string          `yaml:"available-models,omitempty"       json:"availableModels,omitempty"`
	ModelOverrides        map[string]string `yaml:"model-overrides,omitempty"        json:"modelOverrides,omitempty"`
	EffortLevel           string            `yaml:"effort-level,omitempty"           json:"effortLevel,omitempty"`
	AlwaysThinkingEnabled *bool             `yaml:"always-thinking-enabled,omitempty" json:"alwaysThinkingEnabled,omitempty"`

	// ============================================================================
	// SANDBOX & SECURITY
	// ============================================================================
	Sandbox  *SandboxConfig  `yaml:"sandbox,omitempty"          json:"sandbox,omitempty"`
	AutoMode *AutoModeConfig `yaml:"auto-mode,omitempty"        json:"autoMode,omitempty"`
	Worktree *WorktreeConfig `yaml:"worktree,omitempty"         json:"worktree,omitempty"`

	// ============================================================================
	// HOOKS MANAGEMENT
	// ============================================================================
	DisableAllHooks        *bool    `yaml:"disable-all-hooks,omitempty"          json:"disableAllHooks,omitempty"`
	AllowManagedHooksOnly  *bool    `yaml:"allow-managed-hooks-only,omitempty"   json:"allowManagedHooksOnly,omitempty"`
	AllowedHttpHookUrls    []string `yaml:"allowed-http-hook-urls,omitempty"     json:"allowedHttpHookUrls,omitempty"`
	HttpHookAllowedEnvVars []string `yaml:"http-hook-allowed-env-vars,omitempty" json:"httpHookAllowedEnvVars,omitempty"`

	// ============================================================================
	// MCP SERVER CONTROL
	// ============================================================================
	EnabledMcpjsonServers      []string `yaml:"enabled-mcpjson-servers,omitempty"        json:"enabledMcpjsonServers,omitempty"`
	DisabledMcpjsonServers     []string `yaml:"disabled-mcpjson-servers,omitempty"       json:"disabledMcpjsonServers,omitempty"`
	AllowManagedMcpServersOnly *bool    `yaml:"allow-managed-mcp-servers-only,omitempty" json:"allowManagedMcpServersOnly,omitempty"`

	// ============================================================================
	// ENVIRONMENT & AUTHENTICATION HELPERS
	// ============================================================================
	Env                 map[string]string `yaml:"env,omitempty"              json:"env,omitempty"`
	ApiKeyHelper        string            `yaml:"api-key-helper,omitempty"         json:"apiKeyHelper,omitempty"`
	AwsCredentialExport string            `yaml:"aws-credential-export,omitempty"  json:"awsCredentialExport,omitempty"`
	AwsAuthRefresh      string            `yaml:"aws-auth-refresh,omitempty"      json:"awsAuthRefresh,omitempty"`
	GcpAuthRefresh      string            `yaml:"gcp-auth-refresh,omitempty"      json:"gcpAuthRefresh,omitempty"`
	OtelHeadersHelper   string            `yaml:"otel-headers-helper,omitempty"   json:"otelHeadersHelper,omitempty"`

	// ============================================================================
	// GIT INTEGRATION
	// ============================================================================
	IncludeGitInstructions *bool              `yaml:"include-git-instructions,omitempty" json:"includeGitInstructions,omitempty"`
	PrUrlTemplate          string             `yaml:"pr-url-template,omitempty"          json:"prUrlTemplate,omitempty"`
	Attribution            *AttributionConfig `yaml:"attribution,omitempty"      json:"attribution,omitempty"`

	// ============================================================================
	// UI & DISPLAY
	// ============================================================================
	EditorMode                 string                     `yaml:"editor-mode,omitempty"                  json:"editorMode,omitempty"`
	Tui                        *bool                      `yaml:"tui,omitempty"                          json:"tui,omitempty"`
	ViewMode                   string                     `yaml:"view-mode,omitempty"                    json:"viewMode,omitempty"`
	OutputStyle                string                     `yaml:"output-style,omitempty"                 json:"outputStyle,omitempty"`
	Language                   string                     `yaml:"language,omitempty"                     json:"language,omitempty"`
	SpinnerTipsEnabled         *bool                      `yaml:"spinner-tips-enabled,omitempty"         json:"spinnerTipsEnabled,omitempty"`
	AutoScrollEnabled          *bool                      `yaml:"auto-scroll-enabled,omitempty"          json:"autoScrollEnabled,omitempty"`
	TerminalProgressBarEnabled *bool                      `yaml:"terminal-progress-bar-enabled,omitempty" json:"terminalProgressBarEnabled,omitempty"`
	ShowTurnDuration           *bool                      `yaml:"show-turn-duration,omitempty"           json:"showTurnDuration,omitempty"`
	PreferredNotifChannel      string                     `yaml:"preferred-notif-channel,omitempty"      json:"preferredNotifChannel,omitempty"`
	SyntaxHighlightingDisabled *bool                      `yaml:"syntax-highlighting-disabled,omitempty" json:"syntaxHighlightingDisabled,omitempty"`
	PrefersReducedMotion       *bool                      `yaml:"prefers-reduced-motion,omitempty"       json:"prefersReducedMotion,omitempty"`
	SkipWebFetchPreflight      *bool                      `yaml:"skip-web-fetch-preflight,omitempty"     json:"skipWebFetchPreflight,omitempty"`
	SpinnerVerbs               *SpinnerVerbsConfig        `yaml:"spinner-verbs,omitempty"                json:"spinnerVerbs,omitempty"`
	SpinnerTipsOverride        *SpinnerTipsOverrideConfig `yaml:"spinner-tips-override,omitempty"        json:"spinnerTipsOverride,omitempty"`
	Voice                      *VoiceConfig               `yaml:"voice,omitempty"                        json:"voice,omitempty"`
	FileSuggestion             *FileSuggestionConfig      `yaml:"file-suggestion,omitempty"              json:"fileSuggestion,omitempty"`
	StatusLine                 *StatusLineConfig          `yaml:"status-line,omitempty"                  json:"statusLine,omitempty"`

	// ============================================================================
	// FEATURE FLAGS
	// ============================================================================
	AutoMemoryEnabled          *bool  `yaml:"auto-memory-enabled,omitempty"           json:"autoMemoryEnabled,omitempty"`
	AutoMemoryDirectory        string `yaml:"auto-memory-directory,omitempty"         json:"autoMemoryDirectory,omitempty"`
	FastMode                   *bool  `yaml:"fast-mode,omitempty"                     json:"fastMode,omitempty"`
	FastModePerSessionOptIn    *bool  `yaml:"fast-mode-per-session-opt-in,omitempty"  json:"fastModePerSessionOptIn,omitempty"`
	ChannelsEnabled            *bool  `yaml:"channels-enabled,omitempty"              json:"channelsEnabled,omitempty"`
	DisableAutoMode            *bool  `yaml:"disable-auto-mode,omitempty"             json:"disableAutoMode,omitempty"`
	DisableRemoteControl       *bool  `yaml:"disable-remote-control,omitempty"        json:"disableRemoteControl,omitempty"`
	DisableSkillShellExecution *bool  `yaml:"disable-skill-shell-execution,omitempty" json:"disableSkillShellExecution,omitempty"`

	// ============================================================================
	// PLUGINS
	// ============================================================================
	EnabledPlugins          map[string]any `yaml:"enabled-plugins,omitempty"          json:"enabledPlugins,omitempty"`
	PluginConfigs           map[string]any `yaml:"plugin-configs,omitempty"           json:"pluginConfigs,omitempty"`
	ExtraKnownMarketplaces  map[string]any `yaml:"extra-known-marketplaces,omitempty" json:"extraKnownMarketplaces,omitempty"`
	SkippedMarketplaces     []string       `yaml:"skipped-marketplaces,omitempty"     json:"skippedMarketplaces,omitempty"`
	SkippedPlugins          []string       `yaml:"skipped-plugins,omitempty"          json:"skippedPlugins,omitempty"`
	AllowedChannelPlugins   []string       `yaml:"allowed-channel-plugins,omitempty"  json:"allowedChannelPlugins,omitempty"`
	BlockedMarketplaces     []any          `yaml:"blocked-marketplaces,omitempty"     json:"blockedMarketplaces,omitempty"`
	StrictKnownMarketplaces []any          `yaml:"strict-known-marketplaces,omitempty" json:"strictKnownMarketplaces,omitempty"`
	PluginTrustMessage      string         `yaml:"plugin-trust-message,omitempty"     json:"pluginTrustMessage,omitempty"`

	// ============================================================================
	// SKILLS & OVERRIDES
	// ============================================================================
	SkillOverrides map[string]string `yaml:"skill-overrides,omitempty"  json:"skillOverrides,omitempty"`

	// ============================================================================
	// UPDATES & VERSIONING
	// ============================================================================
	AutoUpdatesChannel string `yaml:"auto-updates-channel,omitempty" json:"autoUpdatesChannel,omitempty"`
	MinimumVersion     string `yaml:"minimum-version,omitempty"      json:"minimumVersion,omitempty"`

	// ============================================================================
	// AUTHENTICATION & PERMISSIONS
	// ============================================================================
	ForceLoginMethod                string `yaml:"force-login-method,omitempty"                  json:"forceLoginMethod,omitempty"`
	ForceLoginOrgUUID               string `yaml:"force-login-org-uuid,omitempty"                json:"forceLoginOrgUUID,omitempty"`
	AllowManagedPermissionRulesOnly *bool  `yaml:"allow-managed-permission-rules-only,omitempty" json:"allowManagedPermissionRulesOnly,omitempty"`

	// ============================================================================
	// REMOTE CONNECTIONS
	// ============================================================================
	SSH []SSHConfigEntry `yaml:"ssh,omitempty" json:"sshConfigs,omitempty"`

	// ============================================================================
	// MISCELLANEOUS
	// ============================================================================
	DefaultShell           string   `yaml:"default-shell,omitempty"            json:"defaultShell,omitempty"`
	RespectGitignore       *bool    `yaml:"respect-gitignore,omitempty"        json:"respectGitignore,omitempty"`
	ParentSettingsBehavior string   `yaml:"parent-settings-behavior,omitempty" json:"parentSettingsBehavior,omitempty"`
	IncludeCoAuthoredBy    *bool    `yaml:"include-co-authored-by,omitempty"   json:"includeCoAuthoredBy,omitempty"`
	TeammateMode           string   `yaml:"teammate-mode,omitempty"            json:"teammateMode,omitempty"`
	CleanupPeriodDays      *int     `yaml:"cleanup-period-days,omitempty"      json:"cleanupPeriodDays,omitempty"`
	ClaudeMdExcludes       []string `yaml:"claude-md-excludes,omitempty"       json:"claudeMdExcludes,omitempty"`
	PlansDirectory         string   `yaml:"plans-directory,omitempty"          json:"plansDirectory,omitempty"`
	CompanyAnnouncements   []string `yaml:"company-announcements,omitempty"    json:"companyAnnouncements,omitempty"`
	FeedbackSurveyRate     *float64 `yaml:"feedback-survey-rate,omitempty"     json:"feedbackSurveyRate,omitempty"`
}

// SandboxConfig controls sandbox behavior for tool execution isolation.
// Portable and Claude-specific fields use json tags for Claude output.
// Cursor-only fields use json:"-" to prevent leaking into Claude output
// via json.Marshal in extractClaudeSettings.
type SandboxConfig struct {
	// --- Portable / Claude-specific ---
	Enabled                      *bool                 `yaml:"enabled,omitempty"                       json:"enabled,omitempty"`
	AutoAllowBashIfSandboxed     *bool                 `yaml:"auto-allow-bash-if-sandboxed,omitempty"  json:"autoAllowBashIfSandboxed,omitempty"`
	ExcludedCommands             []string              `yaml:"excluded-commands,omitempty"              json:"excludedCommands,omitempty"`
	Filesystem                   *SandboxFSConfig      `yaml:"filesystem,omitempty"                    json:"filesystem,omitempty"`
	Network                      *SandboxNetConfig     `yaml:"network,omitempty"                       json:"network,omitempty"`
	FailIfUnavailable            *bool                 `yaml:"fail-if-unavailable,omitempty"            json:"failIfUnavailable,omitempty"`
	AllowUnsandboxedCommands     *bool                 `yaml:"allow-unsandboxed-commands,omitempty"     json:"allowUnsandboxedCommands,omitempty"`
	EnableWeakerNestedSandbox    *bool                 `yaml:"enable-weaker-nested-sandbox,omitempty"   json:"enableWeakerNestedSandbox,omitempty"`
	EnableWeakerNetworkIsolation *bool                 `yaml:"enable-weaker-network-isolation,omitempty" json:"enableWeakerNetworkIsolation,omitempty"`
	IgnoreViolations             map[string][]string   `yaml:"ignore-violations,omitempty"              json:"ignoreViolations,omitempty"`
	Ripgrep                      *SandboxRipgrepConfig `yaml:"ripgrep,omitempty"                       json:"ripgrep,omitempty"`
	BwrapPath                    string                `yaml:"bwrap-path,omitempty"                    json:"bwrapPath,omitempty"`
	SocatPath                    string                `yaml:"socat-path,omitempty"                    json:"socatPath,omitempty"`

	// --- Cursor-only ---
	Type                     string   `yaml:"type,omitempty"                         json:"-"`
	AdditionalReadwritePaths []string `yaml:"additional-readwrite-paths,omitempty"   json:"-"`
	AdditionalReadonlyPaths  []string `yaml:"additional-readonly-paths,omitempty"    json:"-"`
	DisableTmpWrite          *bool    `yaml:"disable-tmp-write,omitempty"            json:"-"`
	EnableSharedBuildCache   *bool    `yaml:"enable-shared-build-cache,omitempty"    json:"-"`
}

// SandboxFSConfig controls filesystem access within the sandbox.
type SandboxFSConfig struct {
	AllowWrite                []string `yaml:"allow-write,omitempty"                  json:"allowWrite,omitempty"`
	DenyWrite                 []string `yaml:"deny-write,omitempty"                   json:"denyWrite,omitempty"`
	DenyRead                  []string `yaml:"deny-read,omitempty"                    json:"denyRead,omitempty"`
	AllowRead                 []string `yaml:"allow-read,omitempty"                   json:"allowRead,omitempty"`
	AllowManagedReadPathsOnly *bool    `yaml:"allow-managed-read-paths-only,omitempty" json:"allowManagedReadPathsOnly,omitempty"`
}

// SandboxNetConfig controls network access within the sandbox.
type SandboxNetConfig struct {
	// --- Portable / Claude-specific ---
	AllowedDomains          []string `yaml:"allowed-domains,omitempty"          json:"allowedDomains,omitempty"`
	DeniedDomains           []string `yaml:"denied-domains,omitempty"           json:"deniedDomains,omitempty"`
	AllowUnixSockets        []string `yaml:"allow-unix-sockets,omitempty"       json:"allowUnixSockets,omitempty"`
	AllowAllUnixSockets     *bool    `yaml:"allow-all-unix-sockets,omitempty"   json:"allowAllUnixSockets,omitempty"`
	AllowLocalBinding       *bool    `yaml:"allow-local-binding,omitempty"      json:"allowLocalBinding,omitempty"`
	AllowMachLookup         []string `yaml:"allow-mach-lookup,omitempty"        json:"allowMachLookup,omitempty"`
	HttpProxyPort           *int     `yaml:"http-proxy-port,omitempty"          json:"httpProxyPort,omitempty"`
	SocksProxyPort          *int     `yaml:"socks-proxy-port,omitempty"         json:"socksProxyPort,omitempty"`
	AllowManagedDomainsOnly *bool    `yaml:"allow-managed-domains-only,omitempty" json:"allowManagedDomainsOnly,omitempty"`

	// --- Cursor-only ---
	DefaultPolicy string `yaml:"default-policy,omitempty" json:"-"`
}

// SandboxRipgrepConfig controls ripgrep within sandbox.
type SandboxRipgrepConfig struct {
	Command string   `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string `yaml:"args,omitempty"    json:"args,omitempty"`
}

// WorktreeConfig controls --worktree flag behavior.
type WorktreeConfig struct {
	BaseRef            string   `yaml:"base-ref,omitempty"            json:"baseRef,omitempty"`
	SymlinkDirectories []string `yaml:"symlink-directories,omitempty" json:"symlinkDirectories,omitempty"`
	SparsePaths        []string `yaml:"sparse-paths,omitempty"        json:"sparsePaths,omitempty"`
}

// AutoModeConfig controls auto-approve mode with safety checks.
type AutoModeConfig struct {
	Environment []string `yaml:"environment,omitempty" json:"environment,omitempty"` // trusted domain descriptions
	Allow       []string `yaml:"allow,omitempty"       json:"allow,omitempty"`       // allowed operations
	SoftDeny    []string `yaml:"soft-deny,omitempty"   json:"softDeny,omitempty"`    // soft-denied operations
}

// AttributionConfig controls how Claude Code attributes its work.
// json tags intentionally omit "omitempty" because empty string is meaningful
// (it means "disable attribution").
type AttributionConfig struct {
	Commit string `yaml:"commit,omitempty" json:"commit"` // commit trailer template (empty to disable)
	PR     string `yaml:"pr,omitempty"     json:"pr"`     // PR attribution template (empty to disable)
}

// DepSpec defines a single external binary dependency.
type DepSpec struct {
	Install    string `yaml:"install,omitempty"`
	Download   string `yaml:"download,omitempty"`
	SHA256     string `yaml:"sha256,omitempty"`
	Executable string `yaml:"executable,omitempty"`
	Verify     string `yaml:"verify,omitempty"`
	Global     bool   `yaml:"global,omitempty"`
}

// Repo defines a repository for workspace init.
type Repo struct {
	URL      string `yaml:"url"                json:"url"`
	Path     string `yaml:"path"               json:"path"`
	Branch   string `yaml:"branch,omitempty"    json:"branch,omitempty"`
	Required *bool  `yaml:"required,omitempty"  json:"required,omitempty"`
	Force    bool   `yaml:"force,omitempty"     json:"force,omitempty"`
}

// Validate checks a parsed KeelConfig for unsafe or invalid values.
func (c *KeelConfig) Validate() error {
	if c == nil {
		return nil
	}

	// Bounds checks for all collection fields.
	checks := []struct {
		name string
		len  int
		max  int
	}{
		{"mcp.servers", len(c.mcpServers()), 100},
		{"hooks", len(c.Hooks), 100},
		{"instructions", len(c.Instructions), 1000},
		{"permissions.rules", len(c.permRules()), 100},
		{"commands", len(c.Commands), 500},
		{"agents", len(c.Agents), 500},
		{"skills", len(c.Skills), 500},
		{"plugins", len(c.Plugins), 500},
		{"agent-memory", len(c.AgentMemory), 500},
		{"environment", len(c.Environment), 100},
		{"rules", len(c.Rules), 500},
		{"init", len(c.Init), 100},
		{"snippets", len(c.Snippets), 500},
		{"bin", len(c.Bin), 50},
		{"loop", len(c.Loop), 100},
		{"worktree-include", len(c.WorktreeInclude), 100},
		{"deps", len(c.Deps), 50},
		{"files map", len(c.Files), 500},
	}
	for _, ch := range checks {
		if err := checkMaxLen(ch.name, ch.len, ch.max); err != nil {
			return err
		}
	}

	// Validate repos
	for name, repo := range c.Repos {
		if len(repo.URL) > 2048 {
			return fmt.Errorf("repo %q URL exceeds maximum length of 2048", name)
		}
		if repo.Path != "" {
			if err := validateRepoPath(name, repo.Path); err != nil {
				return err
			}
		}
	}

	// Validate merge strategy values in the files map.
	for name, fp := range c.Files {
		if fp.Merge != "" && !policy.IsValidStrategy(fp.Merge) {
			return fmt.Errorf("files[%q]: invalid merge strategy %q (valid: deep-merge, replace, additive)", name, fp.Merge)
		}
	}

	// Validate permissions policy merge value.
	if c.Permissions != nil && c.Permissions.Policy != nil {
		m := c.Permissions.Policy.Merge
		if m != "" && m != "additive" && m != "replace" {
			return fmt.Errorf("permissions.policy.merge: invalid value %q (valid: additive, replace)", m)
		}
	}

	// Validate deps entries.
	if err := c.validateDeps(); err != nil {
		return err
	}

	return nil
}

// checkMaxLen returns an error if length exceeds max.
func checkMaxLen(name string, length, max int) error {
	if length > max {
		return fmt.Errorf("%s list has %d entries, exceeding maximum of %d", name, length, max)
	}
	return nil
}

// validateRepoPath checks that a repo path is a safe relative path without
// traversal components. It rejects absolute paths, ".." or "." segments,
// and empty segments (consecutive slashes).
func validateRepoPath(repoName, path string) error {
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("repo %q: invalid path %q: must be a relative path without '..' or '.' segments", repoName, path)
	}
	// Allow a single trailing slash (directory convention) by trimming it
	// before splitting into segments.
	cleaned := strings.TrimSuffix(path, "/")
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == ".." || seg == "." || seg == "" {
			return fmt.Errorf("repo %q: invalid path %q: must be a relative path without '..' or '.' segments", repoName, path)
		}
	}
	return nil
}

// validateDeps checks that each dependency entry is well-formed.
func (c *KeelConfig) validateDeps() error {
	for name, dep := range c.Deps {
		// Name constraints: no path separators, not empty, not dot-prefixed.
		if name == "" {
			return fmt.Errorf("deps: empty dependency name")
		}
		if strings.HasPrefix(name, ".") {
			return fmt.Errorf("deps[%q]: name must not start with '.'", name)
		}
		if strings.ContainsAny(name, "/\\") {
			return fmt.Errorf("deps[%q]: name must not contain path separators", name)
		}

		// Install and Download are mutually exclusive but one is required.
		if dep.Install != "" && dep.Download != "" {
			return fmt.Errorf("deps[%q]: cannot set both install and download", name)
		}
		if dep.Install == "" && dep.Download == "" {
			return fmt.Errorf("deps[%q]: must set either install or download", name)
		}

		// SHA256 and Executable only make sense with Download.
		if dep.SHA256 != "" && dep.Download == "" {
			return fmt.Errorf("deps[%q]: sha256 requires download to be set", name)
		}
		if dep.Executable != "" && dep.Download == "" {
			return fmt.Errorf("deps[%q]: executable requires download to be set", name)
		}
	}
	return nil
}

// mcpServers returns the MCP server FileRef slice, or nil if MCP is unset.
func (c *KeelConfig) mcpServers() []FileRef {
	if c.MCP == nil {
		return nil
	}
	return c.MCP.Servers
}

// permRules returns the Permissions rules FileRef slice, or nil if Permissions is unset.
func (c *KeelConfig) permRules() []FileRef {
	if c.Permissions == nil {
		return nil
	}
	return c.Permissions.Rules
}

// Parse reads and unmarshals a keel.yml file.
func Parse(path string) (*KeelConfig, error) {
	data, err := pathutil.SafeReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading keel.yml at %q: %w", path, err)
	}

	return ParseBytes(data)
}

// ParseBytes unmarshals a keel.yml from raw bytes.
func ParseBytes(data []byte) (*KeelConfig, error) {
	var cfg KeelConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing keel.yml: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating keel config: %w", err)
	}

	return &cfg, nil
}
