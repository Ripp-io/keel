---
title: Compatibility Matrix
weight: 10
---

# Cross-Agent Compatibility Matrix

Exhaustive reference for every config file, setting, hook event, frontmatter field, and behavioral difference between supported agents.
This is the single source of truth for all cross-agent portability claims — the compiler's portability tables are derived from this document and verified by automated tests.

Last verified: May 2026.

## Sources

| Source | URL | Agent | Purpose |
|--------|-----|-------|---------|
| Settings docs | https://code.claude.com/docs/en/settings | Claude | Settings.json keys, scopes, merge behavior |
| Skills docs | https://code.claude.com/docs/en/skills | Claude | SKILL.md frontmatter, companion files, variables |
| Hooks docs | https://code.claude.com/docs/en/hooks | Claude | Hook events, handler types, scoping |
| Sub-agents docs | https://code.claude.com/docs/en/sub-agents | Claude | Agent frontmatter, tool restrictions |
| MCP docs | https://code.claude.com/docs/en/mcp | Claude | MCP server config format and scopes |
| Plugins docs | https://code.claude.com/docs/en/plugins | Claude | Plugin manifest, directory structure |
| Commands docs | https://code.claude.com/docs/en/commands | Claude | Slash command format |
| Memory docs | https://code.claude.com/docs/en/memory | Claude | Auto-memory, agent memory |
| JSON schema | https://json.schemastore.org/claude-code-settings.json | Claude | Authoritative settings schema |
| Agent Skills spec | https://agentskills.io/specification | Both | SKILL.md base spec |
| CLI config docs | https://cursor.com/docs/cli/reference/configuration | Cursor | cli-config.json keys, scopes |
| CLI permissions docs | https://cursor.com/docs/cli/reference/permissions | Cursor | Permission format, tool names |
| Rules docs | https://cursor.com/docs/context/rules | Cursor | .mdc frontmatter, activation modes |
| Hooks docs | https://cursor.com/docs/hooks | Cursor | Hook events, handler types |
| Third-party hooks | https://cursor.com/docs/reference/third-party-hooks | Cursor | Claude-Cursor hook mapping |
| Skills docs | https://cursor.com/docs/context/skills | Cursor | SKILL.md frontmatter |
| Subagents docs | https://cursor.com/docs/subagents | Cursor | Agent frontmatter |
| MCP docs | https://cursor.com/docs/mcp | Cursor | mcp.json format, transports |
| Sandbox docs | https://cursor.com/docs/reference/sandbox | Cursor | sandbox.json format |
| Plugins docs | https://cursor.com/docs/plugins | Cursor | Plugin structure |
| Ignore files docs | https://cursor.com/docs/context/ignore-files | Cursor | .cursorignore |
| Enterprise docs | https://cursor.com/docs/enterprise | Cursor | Managed policies |

---

## Section 1: File Inventory

Every config file both agents read, at project and user scope.

| Config | Claude Project | Claude User | Cursor Project | Cursor User | Format Same? | Keel Handles | Notes |
|--------|---------------|------------|---------------|------------|-------------|-------------|-------|
| Instructions | `./CLAUDE.md` or `./.claude/CLAUDE.md` | `~/.claude/CLAUDE.md` | `.cursor/rules/00-keel.mdc` | No filesystem path | No | Yes | Claude concatenates markdown; Cursor wraps in MDC with frontmatter |
| Instructions (legacy) | — | — | `.cursorrules` | — | — | Reader only | Deprecated, ignored by Cursor Agent mode |
| Instructions (cross-tool) | ✅ (fallback) | — | ✅ `AGENTS.md` | — | Yes | Yes | AAIF standard; Keel reads/writes via `agentsmd` adapter |
| Local instructions | `./CLAUDE.local.md` | — | — | — | — | No | Claude-only, gitignored |
| Settings | `.claude/settings.json` | `~/.claude/settings.json` | `.cursor/cli-config.json` (or `cli.json`) | `~/.cursor/cli-config.json` | No | Yes | Different filenames, different key sets |
| Settings local | `.claude/settings.local.json` | `~/.claude/settings.local.json` | — | — | — | No | Claude-only, by design (per-developer) |
| MCP | `./.mcp.json` | Embedded in `~/.claude.json` | `.cursor/mcp.json` | `~/.cursor/mcp.json` | No | Yes | Claude user MCP is embedded in `~/.claude.json`; Cursor uses standalone file at both scopes |
| Commands | `.claude/commands/*.md` | `~/.claude/commands/*.md` | `.cursor/commands/*.md` | `~/.cursor/commands/*.md` | No | Yes | Claude supports frontmatter; Cursor does not |
| Agents | `.claude/agents/*.md` | `~/.claude/agents/*.md` | `.cursor/agents/*.md` | `~/.cursor/agents/*.md` | No | Yes | Different frontmatter fields; Cursor also reads `.claude/agents/` as fallback |
| Skills | `.claude/skills/*/SKILL.md` | `~/.claude/skills/*/SKILL.md` | `.cursor/skills/*/SKILL.md` | `~/.cursor/skills/*/SKILL.md` | Partial | Yes | Claude extends Agent Skills spec with 9 additional fields |
| Plugins | `.claude/plugins/` | `~/.claude/plugins/` | — | `~/.cursor/plugins/local/` | No | Claude only | Different manifest format; Cursor plugins are marketplace artifacts |
| Agent memory | `.claude/agent-memory/` | `~/.claude/agent-memory/` | — | — | — | Yes | Claude-only category |
| Rules | `.claude/rules/*.md` | `~/.claude/rules/*.md` | `.cursor/rules/*.mdc` | No filesystem path | No | Yes | Different frontmatter; Cursor user rules are UI-only |
| Hooks | In `.claude/settings.json` | In `~/.claude/settings.json` | `.cursor/hooks.json` | `~/.cursor/hooks.json` | No | Yes | Claude embeds in settings; Cursor uses separate file |
| Environment | — | — | `.cursor/environment.json` | — | — | Yes | Cursor-only |
| Sandbox | In `.claude/settings.json` | In `~/.claude/settings.json` | `.cursor/sandbox.json` | `~/.cursor/sandbox.json` | No | No | Claude uses settings keys; Cursor uses separate file |
| Loop prompt | `.claude/loop.md` | `~/.claude/loop.md` | — | — | — | No | Claude-only |
| Worktree include | `./.worktreeinclude` | — | — | — | — | No | Claude-only |
| Scheduled tasks | `.claude/scheduled_tasks.json` | — | — | — | — | No | Claude-only, runtime state |
| User global config | — | `~/.claude.json` | — | — | — | No | Claude-only, runtime state (OAuth, MCP, IDE flags) |
| Managed settings | System paths | — | — | — | — | No | Claude-only (system/container) |
| Managed MCP | System paths | — | — | — | — | No | Claude-only (system/container) |
| Permissions (IDE) | — | — | — | `~/.cursor/permissions.json` | — | No | Cursor-only, IDE-only |
| Ignore files | — | — | `.cursorignore`, `.cursorindexingignore` | — | — | No | Cursor-only, IDE indexing |
| Enterprise hooks | — | — | System paths | — | — | No | Cursor-only (system/container) |
| Enterprise policies | — | — | MDM/GPO | — | — | No | Cursor-only (MDM/GPO) |

### User-Level MCP: The Critical Difference

Project-level MCP is a standalone file for both agents (`.mcp.json` for Claude, `.cursor/mcp.json` for Cursor).
User-level MCP diverges: Cursor uses a standalone `~/.cursor/mcp.json`, while Claude embeds MCP config inside `~/.claude.json` alongside OAuth tokens and session state.
The individual server entry format (`command`, `args`, `env`) is identical between user and project for both agents.
Only the container file differs for Claude at user scope.

### Scope Precedence

**Claude:** managed > CLI `--settings` > local > project > user.
Project overrides user.
Scalars: higher-precedence scope wins.
Arrays: concatenated and deduplicated.
Objects: deep-merged.
Permissions: deny > ask > allow, first match wins.

**Cursor:** Enterprise > Team (cloud) > Project > User.
Project permissions merge with global.
Project-level `cli.json` is restricted to permissions only; all other keys are global-only.

### Scope-Restricted Settings (Claude)

- **User/managed-only:** `autoMemoryDirectory`
- **Rejected from project:** `skipDangerousModePermissionPrompt`
- **Not from project:** `autoMode`
- **Managed-only:** `allowedMcpServers`, `deniedMcpServers`, `allowManagedMcpServersOnly`, `allowManagedHooksOnly`, `forceLoginMethod`, `forceLoginOrgUUID`, `allowManagedPermissionRulesOnly`

---

## Section 2: Hook Events

The "IR Event (PascalCase)" column matches the compiler's internal representation.
See [Hooks Reference]({{< relref "hooks" >}}) for usage details and examples.

| IR Event (PascalCase) | Claude | Cursor | Portable | Notes |
|-----------------------|--------|--------|----------|-------|
| SessionStart | ✅ | ✅ sessionStart | ✅ | Case transform only |
| Setup | ✅ | ❌ | ❌ | Claude-only |
| UserPromptSubmit | ✅ | ✅ beforeSubmitPrompt | ✅ | Name differs, not a case transform |
| UserPromptExpansion | ✅ | ❌ | ❌ | Claude-only |
| PreToolUse | ✅ | ✅ preToolUse | ✅ | Case transform only |
| PermissionRequest | ✅ | ❌ | ❌ | Claude-only |
| PermissionDenied | ✅ | ❌ | ❌ | Claude-only |
| PostToolUse | ✅ | ✅ postToolUse | ✅ | Case transform only |
| PostToolUseFailure | ✅ | ✅ postToolUseFailure | ✅ | Case transform only |
| PostToolBatch | ✅ | ❌ | ❌ | Claude-only |
| Notification | ✅ | ❌ | ❌ | Claude-only |
| SubagentStart | ✅ | ✅ subagentStart | ✅ | Case transform only |
| SubagentStop | ✅ | ✅ subagentStop | ✅ | Case transform only |
| TaskCreated | ✅ | ❌ | ❌ | Claude-only |
| TaskCompleted | ✅ | ❌ | ❌ | Claude-only |
| Stop | ✅ | ✅ stop | ✅ | Case transform only |
| StopFailure | ✅ | ❌ | ❌ | Claude-only |
| TeammateIdle | ✅ | ❌ | ❌ | Claude-only |
| InstructionsLoaded | ✅ | ❌ | ❌ | Claude-only |
| ConfigChange | ✅ | ❌ | ❌ | Claude-only |
| CwdChanged | ✅ | ❌ | ❌ | Claude-only |
| FileChanged | ✅ | ❌ | ❌ | Claude-only |
| WorktreeCreate | ✅ | ❌ | ❌ | Claude-only |
| WorktreeRemove | ✅ | ❌ | ❌ | Claude-only |
| PreCompact | ✅ | ✅ preCompact | ✅ | Case transform only |
| PostCompact | ✅ | ❌ | ❌ | Claude-only |
| Elicitation | ✅ | ❌ | ❌ | Claude-only |
| ElicitationResult | ✅ | ❌ | ❌ | Claude-only |
| SessionEnd | ✅ | ✅ sessionEnd | ✅ | Case transform only |
| beforeShellExecution | ❌ | ✅ beforeShellExecution | ❌ | Cursor-only |
| afterShellExecution | ❌ | ✅ afterShellExecution | ❌ | Cursor-only |
| beforeMCPExecution | ❌ | ✅ beforeMCPExecution | ❌ | Cursor-only |
| afterMCPExecution | ❌ | ✅ afterMCPExecution | ❌ | Cursor-only |
| beforeReadFile | ❌ | ✅ beforeReadFile | ❌ | Cursor-only |
| afterFileEdit | ❌ | ✅ afterFileEdit | ❌ | Cursor-only |
| afterAgentResponse | ❌ | ✅ afterAgentResponse | ❌ | Cursor-only |
| afterAgentThought | ❌ | ✅ afterAgentThought | ❌ | Cursor-only |
| beforeTabFileRead | ❌ | ✅ beforeTabFileRead | ❌ | Cursor-only |
| afterTabFileEdit | ❌ | ✅ afterTabFileEdit | ❌ | Cursor-only |

---

## Section 3: Hook Handler Types

| Type | Claude | Cursor | Portable |
|------|--------|--------|----------|
| command | ✅ | ✅ | ✅ |
| prompt | ✅ | ✅ | ✅ |
| http | ✅ | ❌ | ❌ |
| mcp_tool | ✅ | ❌ | ❌ |
| agent | ✅ | ❌ | ❌ |

---

## Section 4: Frontmatter Fields by Category

Portable means the field is part of the compiler's intermediate representation and handled during cross-compilation.
Fields marked portable but only supported by one agent are carried in the IR and emit lossy warnings when cross-compiling to the agent that lacks native support.
See [Frontmatter Reference]({{< relref "frontmatter" >}}) for usage details.

### agent

| Category | Field | Claude | Cursor | Portable | Notes |
|----------|-------|--------|--------|----------|-------|
| agent | name | ✅ | ✅ | ✅ | |
| agent | description | ✅ | ✅ | ✅ | |
| agent | tools | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | disallowedTools | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | model | ✅ | ✅ | ✅ | |
| agent | permissionMode | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | maxTurns | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | skills | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | mcpServers | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | hooks | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | memory | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | background | ✅ | ❌ | ✅ | Cursor uses `is_background` instead |
| agent | effort | ✅ | ❌ | ✅ | In IR; `xhigh` is Claude-only value |
| agent | isolation | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | color | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | initialPrompt | ✅ | ❌ | ✅ | In IR; dropped with warning for Cursor |
| agent | readonly | ❌ | ✅ | ❌ | Cursor-only; not in IR |
| agent | is_background | ❌ | ✅ | ❌ | Cursor-only; Claude uses `background` |

### skill

| Category | Field | Claude | Cursor | Portable | Notes |
|----------|-------|--------|--------|----------|-------|
| skill | name | ✅ | ✅ | ✅ | Agent Skills spec |
| skill | description | ✅ | ✅ | ✅ | Agent Skills spec |
| skill | paths | ✅ | ✅ | ✅ | Agent Skills spec |
| skill | license | ✅ | ✅ | ✅ | Agent Skills spec |
| skill | compatibility | ✅ | ✅ | ✅ | Agent Skills spec |
| skill | metadata | ✅ | ✅ | ✅ | Agent Skills spec |
| skill | disable-model-invocation | ✅ | ✅ | ✅ | Agent Skills spec |
| skill | allowed-tools | ✅ | ✅ | ✅ | Agent Skills spec |
| skill | when_to_use | ✅ | ❌ | ❌ | Claude-only extension |
| skill | argument-hint | ✅ | ❌ | ❌ | Claude-only extension |
| skill | arguments | ✅ | ❌ | ❌ | Claude-only extension |
| skill | user-invocable | ✅ | ❌ | ❌ | Claude-only extension |
| skill | model | ✅ | ❌ | ❌ | Claude-only extension |
| skill | effort | ✅ | ❌ | ❌ | Claude-only extension |
| skill | context | ✅ | ❌ | ❌ | Claude-only extension |
| skill | agent | ✅ | ❌ | ❌ | Claude-only extension (skill frontmatter field) |
| skill | hooks | ✅ | ❌ | ❌ | Claude-only extension |
| skill | shell | ✅ | ❌ | ❌ | Claude-only extension |

### command

| Category | Field | Claude | Cursor | Portable | Notes |
|----------|-------|--------|--------|----------|-------|
| command | description | ✅ | ❌ | ❌ | Claude-only; Cursor has NO command frontmatter |
| command | allowed-tools | ✅ | ❌ | ❌ | Claude-only |
| command | model | ✅ | ❌ | ❌ | Claude-only |
| command | argument-hint | ✅ | ❌ | ❌ | Claude-only |
| command | disable-model-invocation | ✅ | ❌ | ❌ | Claude-only |
| command | user-invocable | ✅ | ❌ | ❌ | Claude-only |

### rule

| Category | Field | Claude | Cursor | Portable | Notes |
|----------|-------|--------|--------|----------|-------|
| rule | paths | ✅ | ❌ | ❌ | Claude uses `paths`; Cursor uses `globs` for similar purpose |
| rule | alwaysApply | ❌ | ✅ | ❌ | Cursor-only; Claude rules without `paths` load at launch (equivalent behavior) |
| rule | globs | ❌ | ✅ | ❌ | Cursor-only; Claude uses `paths` for similar purpose |
| rule | description | ❌ | ✅ | ❌ | Cursor-only; enables agent-requested activation mode |

---

## Section 5: Settings Keys

### Agent/Model settings

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `model` | ✅ | ✅ | ✅ | Translated by Keel DSL |
| `agent` | ✅ | ❌ | ❌ | Claude-only; subagent type for main thread |
| `availableModels` | ✅ | ❌ | ❌ | Claude-only |
| `modelOverrides` | ✅ | ❌ | ❌ | Claude-only |
| `effortLevel` | ✅ | ❌ | ❌ | Claude-only |
| `alwaysThinkingEnabled` | ✅ | ❌ | ❌ | Claude-only |

### Permissions

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `permissions.allow` | ✅ | ✅ | ✅ | Format differs: Claude uses bare names; Cursor uses `(*)` suffix |
| `permissions.ask` | ✅ | ❌ | ❌ | Claude-only |
| `permissions.deny` | ✅ | ✅ | ✅ | Format differs: same suffix convention as allow |
| `permissions.additionalDirectories` | ✅ | ❌ | ❌ | Claude-only |
| `permissions.defaultMode` | ✅ | ❌ | ❌ | Claude-only |
| `permissions.disableBypassPermissionsMode` | ✅ | ❌ | ❌ | Claude-only |
| `permissions.skipDangerousModePermissionPrompt` | ✅ | ❌ | ❌ | Claude-only; rejected from project scope |

### Sandbox (Claude-only)

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `sandbox.enabled` | ✅ | ❌ | ❌ | Cursor uses separate sandbox.json |
| `sandbox.failIfUnavailable` | ✅ | ❌ | ❌ | |
| `sandbox.autoAllowBashIfSandboxed` | ✅ | ❌ | ❌ | |
| `sandbox.excludedCommands` | ✅ | ❌ | ❌ | |
| `sandbox.allowUnsandboxedCommands` | ✅ | ❌ | ❌ | |
| `sandbox.filesystem.allowWrite` | ✅ | ❌ | ❌ | |
| `sandbox.filesystem.denyWrite` | ✅ | ❌ | ❌ | |
| `sandbox.filesystem.allowRead` | ✅ | ❌ | ❌ | |
| `sandbox.filesystem.denyRead` | ✅ | ❌ | ❌ | |
| `sandbox.filesystem.allowManagedReadPathsOnly` | ✅ | ❌ | ❌ | Managed-only |
| `sandbox.network.allowedDomains` | ✅ | ❌ | ❌ | |
| `sandbox.network.deniedDomains` | ✅ | ❌ | ❌ | |
| `sandbox.network.allowUnixSockets` | ✅ | ❌ | ❌ | |
| `sandbox.network.allowAllUnixSockets` | ✅ | ❌ | ❌ | |
| `sandbox.network.allowLocalBinding` | ✅ | ❌ | ❌ | |
| `sandbox.network.allowMachLookup` | ✅ | ❌ | ❌ | |
| `sandbox.network.allowManagedDomainsOnly` | ✅ | ❌ | ❌ | Managed-only |
| `sandbox.network.httpProxyPort` | ✅ | ❌ | ❌ | |
| `sandbox.network.socksProxyPort` | ✅ | ❌ | ❌ | |
| `sandbox.enableWeakerNestedSandbox` | ✅ | ❌ | ❌ | |
| `sandbox.enableWeakerNetworkIsolation` | ✅ | ❌ | ❌ | |
| `sandbox.bwrapPath` | ✅ | ❌ | ❌ | |
| `sandbox.socatPath` | ✅ | ❌ | ❌ | |

### Hooks settings keys

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `hooks` | ✅ | ✅ | ✅ | Format differs: Claude embeds in settings (PascalCase); Cursor uses separate hooks.json (camelCase) |
| `disableAllHooks` | ✅ | ❌ | ❌ | Claude-only |
| `allowManagedHooksOnly` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `allowedHttpHookUrls` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `httpHookAllowedEnvVars` | ✅ | ❌ | ❌ | Claude-only, managed-only |

### MCP Server Controls

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `allowedMcpServers` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `deniedMcpServers` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `enableAllProjectMcpServers` | ✅ | ❌ | ❌ | Claude-only |
| `enabledMcpjsonServers` | ✅ | ❌ | ❌ | Claude-only |
| `disabledMcpjsonServers` | ✅ | ❌ | ❌ | Claude-only |
| `allowManagedMcpServersOnly` | ✅ | ❌ | ❌ | Claude-only, managed-only |

### Environment

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `env` | ✅ | ❌ | ❌ | Claude uses settings key; Cursor uses environment.json |
| `apiKeyHelper` | ✅ | ❌ | ❌ | Claude-only |
| `awsCredentialExport` | ✅ | ❌ | ❌ | Claude-only |
| `awsAuthRefresh` | ✅ | ❌ | ❌ | Claude-only |
| `gcpAuthRefresh` | ✅ | ❌ | ❌ | Claude-only |
| `otelHeadersHelper` | ✅ | ❌ | ❌ | Claude-only |

### Git/Attribution

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `attribution.commit` | ✅ | ❌ | ❌ | Format differs: Claude uses template string |
| `attribution.pr` | ✅ | ❌ | ❌ | Format differs: Claude uses template string |
| `attribution.attributeCommitsToAgent` | ❌ | ✅ | ❌ | Format differs: Cursor uses boolean |
| `attribution.attributePRsToAgent` | ❌ | ✅ | ❌ | Format differs: Cursor uses boolean |
| `includeGitInstructions` | ✅ | ❌ | ❌ | Claude-only |
| `prUrlTemplate` | ✅ | ❌ | ❌ | Claude-only |

### Worktree (Claude-only)

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `worktree.baseRef` | ✅ | ❌ | ❌ | |
| `worktree.symlinkDirectories` | ✅ | ❌ | ❌ | |
| `worktree.sparsePaths` | ✅ | ❌ | ❌ | |

### Auto Mode (Claude-only)

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `autoMode.environment` | ✅ | ❌ | ❌ | |
| `autoMode.allow` | ✅ | ❌ | ❌ | |
| `autoMode.soft_deny` | ✅ | ❌ | ❌ | |

### Skills/Commands

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `skillOverrides` | ✅ | ❌ | ❌ | Claude-only |

### UI/Terminal

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `editorMode` | ✅ | ❌ | ❌ | Claude-only |
| `tui` | ✅ | ❌ | ❌ | Claude-only |
| `viewMode` | ✅ | ❌ | ❌ | Claude-only |
| `outputStyle` | ✅ | ❌ | ❌ | Claude-only |
| `language` | ✅ | ❌ | ❌ | Claude-only |
| `spinnerTipsEnabled` | ✅ | ❌ | ❌ | Claude-only |
| `autoScrollEnabled` | ✅ | ❌ | ❌ | Claude-only |
| `terminalProgressBarEnabled` | ✅ | ❌ | ❌ | Claude-only |
| `showTurnDuration` | ✅ | ❌ | ❌ | Claude-only |
| `preferredNotifChannel` | ✅ | ❌ | ❌ | Claude-only |
| `syntaxHighlightingDisabled` | ✅ | ❌ | ❌ | Claude-only |
| `prefersReducedMotion` | ✅ | ❌ | ❌ | Claude-only |
| `spinnerVerbs` | ✅ | ❌ | ❌ | Claude-only |
| `spinnerTipsOverride` | ✅ | ❌ | ❌ | Claude-only |
| `skipWebFetchPreflight` | ✅ | ❌ | ❌ | Claude-only |

### Features

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `autoMemoryEnabled` | ✅ | ❌ | ❌ | Claude-only |
| `autoMemoryDirectory` | ✅ | ❌ | ❌ | Claude-only, user/managed only |
| `fastMode` | ✅ | ❌ | ❌ | Claude-only |
| `fastModePerSessionOptIn` | ✅ | ❌ | ❌ | Claude-only |
| `channelsEnabled` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `disableAutoMode` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `disableRemoteControl` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `disableSkillShellExecution` | ✅ | ❌ | ❌ | Claude-only |

### Plugins settings keys

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `enabledPlugins` | ✅ | ❌ | ❌ | Claude-only |
| `pluginConfigs` | ✅ | ❌ | ❌ | Claude-only |
| `extraKnownMarketplaces` | ✅ | ❌ | ❌ | Claude-only |
| `skippedMarketplaces` | ✅ | ❌ | ❌ | Claude-only |
| `skippedPlugins` | ✅ | ❌ | ❌ | Claude-only |
| `allowedChannelPlugins` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `blockedMarketplaces` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `strictKnownMarketplaces` | ✅ | ❌ | ❌ | Claude-only, managed-only |
| `pluginTrustMessage` | ✅ | ❌ | ❌ | Claude-only, managed-only |

### Miscellaneous (Claude-only)

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `cleanupPeriodDays` | ✅ | ❌ | ❌ | |
| `defaultShell` | ✅ | ❌ | ❌ | |
| `respectGitignore` | ✅ | ❌ | ❌ | |
| `parentSettingsBehavior` | ✅ | ❌ | ❌ | |
| `includeCoAuthoredBy` | ✅ | ❌ | ❌ | Deprecated, use `attribution` |
| `teammateMode` | ✅ | ❌ | ❌ | |
| `companyAnnouncements` | ✅ | ❌ | ❌ | |
| `feedbackSurveyRate` | ✅ | ❌ | ❌ | |
| `claudeMdExcludes` | ✅ | ❌ | ❌ | |
| `plansDirectory` | ✅ | ❌ | ❌ | |

### File/Memory (Claude-only)

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `fileSuggestion.type` | ✅ | ❌ | ❌ | |
| `fileSuggestion.command` | ✅ | ❌ | ❌ | |
| `statusLine.type` | ✅ | ❌ | ❌ | |
| `statusLine.command` | ✅ | ❌ | ❌ | |

### Authentication (Claude-only, managed-only)

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `forceLoginMethod` | ✅ | ❌ | ❌ | |
| `forceLoginOrgUUID` | ✅ | ❌ | ❌ | |
| `allowManagedPermissionRulesOnly` | ✅ | ❌ | ❌ | |

### Updates (Claude-only)

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `autoUpdatesChannel` | ✅ | ❌ | ❌ | |
| `minimumVersion` | ✅ | ❌ | ❌ | Managed-only |

### SSH (Claude-only)

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `sshConfigs` | ✅ | ❌ | ❌ | Array of objects with `id`, `name`, `sshHost`, `sshPort`, `sshIdentityFile`, `startDirectory` |

### Voice (Claude-only)

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `voice.enabled` | ✅ | ❌ | ❌ | |
| `voice.mode` | ✅ | ❌ | ❌ | |
| `voice.autoSubmit` | ✅ | ❌ | ❌ | |

### Cursor-only settings

| Key | Claude | Cursor | Portable | Translation Notes |
|-----|--------|--------|----------|-------------------|
| `version` | ❌ | ✅ | ❌ | Schema version (currently 1) |
| `editor.vimMode` | ❌ | ✅ | ❌ | Vim keybindings |
| `hasChangedDefaultModel` | ❌ | ✅ | ❌ | Runtime flag |
| `network.useHttp1ForAgent` | ❌ | ✅ | ❌ | HTTP/1.1 preference |

---

## Section 6: Body-Level Patterns

See [Frontmatter Reference]({{< relref "frontmatter" >}}) for the user-facing guide to these patterns.

| Pattern | Claude | Cursor | Notes |
|---------|--------|--------|-------|
| `$ARGUMENTS` | Positional argument substitution | Literal text | Claude-only |
| `$ARGUMENTS[N]` | Indexed argument access | Literal text | Claude-only |
| `$0`, `$1`, `$2` | Positional arguments (shell-style) | Literal text | Claude-only |
| `${CLAUDE_SKILL_DIR}` | Resolves to skill directory | Literal text | Claude-only |
| `${CLAUDE_SESSION_ID}` | Resolves to session ID | Literal text | Claude-only |
| `${CLAUDE_EFFORT}` | Resolves to effort level | Literal text | Claude-only |
| `` !`command` `` | Shell execution, output embedded | Literal text | Claude-only dynamic injection |
| ` ```! ` fenced blocks | Multi-line shell execution | Literal text | Claude-only dynamic injection |
| `{{variable}}` | Literal text | Template variable (Cursor Maestro) | Cursor-only |
| `${env:NAME}` | Not supported | Env var in mcp.json | Cursor-only, mcp.json only |
| `${userHome}` | Not supported | Home dir in mcp.json | Cursor-only, mcp.json only |
| `${workspaceFolder}` | Not supported | Workspace root in mcp.json | Cursor-only, mcp.json only |
| `${workspaceFolderBasename}` | Not supported | Workspace folder name in mcp.json | Cursor-only, mcp.json only |
| `${pathSeparator}` / `${/}` | Not supported | Path separator in mcp.json | Cursor-only, mcp.json only |

---

## Section 7: Behavioral Differences

See [Transpilation]({{< relref "../guide/transpilation" >}}) for the user-facing guide to cross-compilation.

| Feature | Claude Behavior | Cursor Behavior | Compiler Handling |
|---------|----------------|-----------------|-------------------|
| Hook event names | PascalCase (`PreToolUse`) | camelCase (`preToolUse`) | Keel translates automatically via `claudeToCursorHookEvents` / `cursorToClaudeHookEvents` rename tables plus case transform |
| Hook event rename | `UserPromptSubmit` | `beforeSubmitPrompt` | Explicit rename table, not a case transform |
| Hook input shapes | Varies per event; e.g., `SubagentStart` provides `agent_id` + `agent_type` | Varies per event; e.g., `subagentStart` provides `subagent_id`, `task`, `is_parallel_worker` | Not a compiler concern; hook scripts must handle agent-specific input shapes |
| Hook file structure | Embedded in `settings.json` under `hooks` key | Separate `hooks.json` file | Keel uses `HooksInSettings` on `AgentProfile` to route output |
| Permission format | Bare tool names (e.g., `Bash(npm install)`) | Tool names with `(*)` suffix (e.g., `Bash(*)`) | Keel translates automatically during cross-compilation |
| Permission tiers | Three tiers: `allow`, `ask`, `deny` | Two tiers: `allow`, `deny` | `ask` is Claude-only; dropped with warning during Claude-to-Cursor compilation |
| MCP user scope | Embedded inside `~/.claude.json` (multi-purpose file) | Standalone `~/.cursor/mcp.json` | Compiler cannot safely write `~/.claude.json`; user-level Claude MCP is a manual step or CLI command |
| MCP variable interpolation | Not supported | `${env:NAME}`, `${userHome}`, `${workspaceFolder}` supported | Cursor variables pass through as-is; no translation needed |
| Instruction concatenation | Multiple CLAUDE.md files concatenated root-down | Instructions wrapped in `.mdc` file with frontmatter | Each agent's writer handles format-specific output via `InstructionOutput` on `AgentProfile` |
| Settings merge | Deep-merge with cascading scope precedence | Project-level restricted to permissions only; all other keys global-only | DSL settings use `json.Marshal` on the typed `Settings` struct; agent-specific keys route through `AgentSpecific` map |
| Rules frontmatter | `paths` field for file-scoped activation | `alwaysApply`, `globs`, `description` for four activation modes | No 1:1 mapping; `paths` (Claude) is roughly equivalent to `globs` (Cursor) |
| Agent frontmatter | 16 fields including tool restrictions, hooks, memory, isolation | 5 fields (name, description, model, readonly, is_background) | Claude-to-Cursor is heavily lossy; Cursor-to-Claude is mostly lossless |
| Skill frontmatter | 18 fields (8 Agent Skills spec + 10 Claude extensions) | 8 fields (Agent Skills spec only) | Claude extensions dropped during Claude-to-Cursor cross-compilation |
| Command frontmatter | 6 optional YAML frontmatter fields | No frontmatter support | All command frontmatter stripped with warning during Claude-to-Cursor cross-compilation |
| Handler type support | 5 types: command, prompt, http, mcp_tool, agent | 2 types: command, prompt | Non-portable types (http, mcp_tool, agent) dropped with warning |
| Handler fields | `type`, `if`, `timeout`, `statusMessage`, `once` | `type`, `matcher`, `timeout`, `loop_limit`, `failClosed` | `if` (Claude) loosely maps to `matcher` (Cursor); `loop_limit` and `failClosed` are Cursor-only, pass through as harmless unknown fields |

---

## Section 8: Tool Names

Claude Code and Cursor use identical tool names for the shared set.
No tool name mapping is needed for Claude-to-Cursor compilation.

**Shared tools:** `Read`, `Write`, `Edit`, `MultiEdit`, `Bash`, `Grep`, `Glob`, `WebSearch`, `WebFetch`, `Agent`, `NotebookEdit`.

**Cursor addition:** `Shell` is exposed as an alias for `Bash` in Cursor CLI.
This alias does not require mapping since `Bash` works in both agents.

---

## Section 9: Keel Coverage Matrix

Every config item across Keel's four dimensions: DSL authoring, compiler output, reader import, and transpiler cross-compilation.

| Config Item | DSL | Compile | Import | Transpile | Notes |
|-------------|-----|---------|--------|-----------|-------|
| CLAUDE.md (instructions) | ✅ `instructions[]` | ✅ concatenate | ✅ reader | ✅ lossless | |
| .cursor/rules/00-keel.mdc (instructions) | ✅ `instructions[]` | ✅ concatenate + wrap MDC | ✅ reader | ✅ lossless | |
| .claude/settings.json | ⚠️ core keys | ✅ deep-merge | ✅ reader decomposes | ✅ lossy warnings | Passthrough via `settings.claude` covers the rest |
| .cursor/cli-config.json | ⚠️ core keys | ✅ deep-merge | ✅ reader decomposes | ✅ lossy warnings | Passthrough via `settings.cursor` |
| .mcp.json (Claude) | ✅ `mcp[]` | ✅ deep-merge | ✅ reader | ✅ lossless | |
| .cursor/mcp.json | ✅ `mcp[]` | ✅ deep-merge | ✅ reader | ✅ lossless | |
| .cursor/hooks.json | ✅ `hooks[]` | ✅ deep-merge, separate file | ✅ reader (camel-to-Pascal) | ✅ format differs | camelCase-to-PascalCase |
| hooks (in Claude settings) | ✅ `hooks[]` | ✅ deep-merge | ✅ reader extracts | ✅ format differs | PascalCase-to-camelCase |
| .claude/commands/ | ✅ `commands[]` | ✅ overlay | ✅ reader | ⚠️ frontmatter lossy | Command frontmatter is Claude-only |
| .cursor/commands/ | ✅ `commands[]` | ✅ overlay | ✅ reader | ✅ lossless | Cursor commands have no frontmatter |
| .claude/agents/ | ✅ `agents[]` | ✅ overlay | ✅ reader | ✅ mostly lossless | `xhigh` effort is Claude-only value |
| .cursor/agents/ | ✅ `agents[]` | ✅ overlay | ✅ reader | ⚠️ frontmatter lossy | `readonly`, `is_background` Cursor-only |
| .claude/skills/ | ✅ `skills[]` | ✅ overlay | ✅ reader | ⚠️ frontmatter lossy | 10 Claude-only extension fields |
| .cursor/skills/ | ✅ `skills[]` | ✅ overlay | ✅ reader | ✅ mostly lossless | Base Agent Skills spec only |
| .claude/plugins/ | ✅ `plugins[]` | ✅ overlay | ✅ reader | ⚠️ category lossy | Claude-only category, dropped for Cursor |
| .claude/agent-memory/ | ✅ `agent-memory[]` | ✅ overlay | ✅ reader | ⚠️ category lossy | Claude-only category, dropped for Cursor |
| .claude/rules/ | ✅ `rules[]` | ✅ overlay | ✅ reader | ⚠️ frontmatter lossy | Frontmatter not portable (paths vs globs) |
| .cursor/rules/ | ✅ `rules[]` | ✅ overlay | ✅ reader | ⚠️ frontmatter lossy | `alwaysApply`/`globs`/`description` Cursor-only |
| .cursor/environment.json | ✅ `environment[]` | ✅ deep-merge | ✅ reader | ⚠️ Cursor-only | Claude has no environment.json |
| permissions (Claude) | ✅ `permissions[]` | ✅ | ✅ reader extracts | ✅ format differs | Bare names vs `(*)` suffix |
| permissions (Cursor) | ✅ `permissions[]` | ✅ | ✅ reader extracts | ✅ format differs | `(*)` suffix vs bare names |
| .claude/settings.local.json | — | — | — | — | By design (per-developer) |
| ~/.claude.json | — | — | — | — | By design (runtime state) |
| ~/.claude/settings.json | — | ✅ `--scope user` | — | — | User-level compilation |
| ~/.claude/CLAUDE.md | — | ✅ `--scope user` | — | — | User-level compilation |
| managed-settings.json | — | — | — | — | By design (system/container) |
| managed-mcp.json | — | — | — | — | By design (system/container) |
| .claude/loop.md | ✅ `loop[]` | ✅ concatenate | ✅ reader | ❌ Claude-only | Cursor does not support loop prompts |
| .worktreeinclude | ✅ `worktree-include[]` | ✅ concatenate | ✅ reader | ❌ Claude-only | Cursor does not support worktree includes |
| scheduled_tasks.json | — | — | — | — | By design (runtime state) |
| .cursorrules | — | — | ✅ reader (fallback) | ✅ | Legacy, deprecated |
| AGENTS.md | ✅ via `--output agentsmd=.` | ✅ `--output agentsmd=.` | ✅ reader | ✅ instructions lossless | AAIF standard; instructions only (settings/hooks/MCP not supported) |
| .cursor/sandbox.json | — | — | — | — | Not handled (IDE concern) |
| .cursor/plugins/ | — | — | — | — | Not handled (marketplace artifacts) |
| ~/.cursor/permissions.json | — | — | — | — | Not handled (IDE-only) |
| .cursorignore | — | — | — | — | Not handled (IDE indexing) |
| .cursorindexingignore | — | — | — | — | Not handled (IDE indexing) |
| Enterprise hooks (Cursor) | — | — | — | — | By design (system/container) |
| Enterprise policies (Cursor) | — | — | — | — | By design (MDM/GPO) |

---

## Section 10: Keel Governance Across Agents

Keel's governance features work identically regardless of which agent the output targets.
Governance is a Keel-layer concept that is resolved and stripped before agent-native output is written.
See [Governance]({{< relref "../guide/governance" >}}) for usage details.

### Force-Lock

Force-lock metadata (`_keel` keys in settings, `force:` field in frontmatter) propagates through the merge pipeline.
Higher-trust layers (earlier in layer order) can lock values that downstream layers cannot override.
Force-locked values are enforced during merge and stripped from the final agent-native output.
Neither Claude nor Cursor has a native concept of force-lock — this is entirely a Keel compilation concern.

### Merge Policies

Merge strategies (deep-merge, concatenate, overlay, additive) are configured per-field in the Keel DSL.
These strategies determine how multiple layers combine during compilation.
The resulting merged output is written in the target agent's native format.
Merge policies are the same regardless of target agent; only the output format changes.

### `_keel` Metadata

Settings values prefixed with `_keel` carry governance metadata through the merge pipeline.
These keys are stripped during final output generation.
If `--distributable` is used, `_keel` metadata is preserved in the output so the compiled layer can itself be used as an input layer for further compilation.

### Governance Fields in Frontmatter

The `force`, `scope`, and `merge` fields in YAML frontmatter are Keel governance fields.
They are excluded from portability checking and stripped during compilation.
They never appear in agent-native output.
