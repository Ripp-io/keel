# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Keel, please report it by creating a **confidential issue** on GitLab:

1. Go to [gitlab.com/ripp.io/oss/agentic/keel/-/issues/new](https://gitlab.com/ripp.io/oss/agentic/keel/-/issues/new)
2. Check **"This issue is confidential"**
3. Use the title prefix `[SECURITY]`
4. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Affected version or commit
   - Potential impact

Please do **not** open a public issue for security vulnerabilities.

## Scope

Keel compiles agent configuration that may include:
- Permission rules (allow/deny lists)
- MCP server definitions with credential references
- Hook scripts executed by the agent
- Remote layer sources fetched over HTTPS / SSH

Security-relevant areas include path traversal, secrets leakage into compiled output, remote layer integrity, and hook injection.

## Response

Confirmed vulnerabilities will be patched and noted in the relevant release.
