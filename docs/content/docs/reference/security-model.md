---
title: Security Model
weight: 60
---

# Security Model

Keel processes configuration layers that can contain instructions, scripts, MCP server definitions, and external dependency specifications.
Understanding the trust boundaries is essential for safe adoption.

## Trust Hierarchy

Layers are compiled in order from lowest trust to highest trust.
Higher-trust layers can override lower-trust layers.
Force-locks allow higher-trust layers to prevent downstream overrides.

In a typical enterprise deployment:

| Layer | Trust level | Authored by |
|---|---|---|
| Company | Highest | Platform / security team |
| Organization | High | Engineering org leads |
| Team | Medium | Team leads |
| Project | Standard | Developers |
| User | Lowest | Individual contributors |

**Key principle:** Layer authors are trusted to the degree their layer position implies.
A company layer with `force: true` cannot be overridden by project layers.

## Blast Radius by Feature

### Instructions, commands, agents, skills (low risk)

These produce markdown files that are consumed by AI agents.
A malicious layer could inject harmful instructions, but the AI agent's own safety mechanisms are the trust boundary — not Keel.

### MCP server definitions (medium risk)

MCP servers run as subprocesses of the AI agent.
A malicious layer could define an MCP server that executes arbitrary commands.
Mitigations:
- MCP governance policy (`mcp.policy.allowed`, `mcp.policy.denied`) controls which servers can run
- `enableAllProjectMcpServers` must be explicitly set to bypass approval prompts

### Settings and permissions (medium risk)

Settings control agent behavior (sandbox, auto-mode, hooks).
A malicious layer could weaken security settings (e.g., disable sandbox, add permissive permissions).
Mitigations:
- Force-locks prevent downstream layers from overriding security-critical settings
- `_keel.merge: "additive"` policy prevents downstream layers from removing permissions

### External dependencies — `deps:` section (high risk)

Dependencies with `install:` execute arbitrary shell commands via `sh -c`.
Dependencies with `download:` fetch binaries from URLs.

Mitigations:
- Requires explicit `--allow-install` CLI flag (opt-in, never automatic)
- Sensitive environment variables are stripped before command execution (26 explicit names plus pattern-based matching for `*_API_KEY`, `*_SECRET`, `*_PASSWORD`, etc.)
- 5-minute timeout on install commands, 30-second timeout on verify commands
- SHA256 verification for downloads
- SSRF protection with DNS rebinding mitigation on all HTTP requests
- Download size limits (500 MB total, 50 MB per file)
- Archive extraction guards (zip-slip protection, file count limits)
- Install commands are logged before execution for audit trail

**Recommendation:** Review `deps:` sections in all layers before using `--allow-install`.
Use `keel compile` without `--allow-install` first to see which deps would be installed.

### Hooks (medium risk)

Hooks execute commands in response to agent events.
A malicious layer could define hooks that exfiltrate data or modify files.
Mitigations:
- Hooks are visible in compiled settings.json — inspect before use
- `disableAllHooks` and `allowManagedHooksOnly` settings restrict hook execution
- Force-locks can prevent downstream hook additions

## Network Security

Remote layers (Git repos, HTTPS archives) and dependency downloads use a hardened HTTP client:
- HTTPS required for all remote sources (HTTP rejected)
- SSRF protection: loopback, private, and link-local IP addresses are rejected
- DNS rebinding mitigation: resolved IPs are validated at connect time
- Redirect targets are validated for scheme and SSRF
- Git `ext::` remote helper protocol is explicitly rejected
- 120-second request timeout, 10 redirect limit

## File System Safety

- All file reads validate path containment (symlink-aware) to prevent layer boundary escapes
- All file writes validate path containment to prevent output directory escapes
- Configuration files are size-limited (10 MB) to prevent memory exhaustion
- Archive extraction enforces size limits, file count limits, and path traversal checks
- Cleanup operations validate paths before deletion to prevent manifest tampering attacks

## Known Limitations

- **TOCTOU window:** Path containment checks resolve symlinks, then the file is read/written separately. A symlink could theoretically be swapped between the check and the I/O operation. Exploitation requires the attacker to have write access to the layer directory during compilation.
- **Environment sanitization is pattern-based:** Custom credential variables not matching known names or patterns will not be stripped from dependency install environments.
- **JSON parsing depth:** Standard library JSON parsing without custom depth limits. Bounded by the 10 MB file size limit.

## Recommendations for Enterprise Adoption

1. **Control layer sources.** Use force-locks in company/org layers for security-critical settings.
2. **Review before `--allow-install`.** Audit all `deps:` sections in your layer stack.
3. **Use `--strict` in CI.** Catches configuration conflicts and missing layers.
4. **Use `keel drift` in CI.** Detects unauthorized manual changes to compiled output.
5. **Pin remote layer versions.** Use specific Git refs or SHA256-verified archives, not `main` branches.
