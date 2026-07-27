# Template Engine Test Content

## Phase 1: Variables

Deploy to ${keel:environment} using ${keel:api_url}.

API endpoint: ${keel:api_url}/v1/parties

## Phase 2: Markdown Conditionals

<!-- keel-if: keel.agent == "claude" -->
Use Claude Code's multi-agent orchestration for parallel task dispatch.
<!-- keel-elif: keel.agent == "cursor" -->
Use Cursor's background agent pool for parallel task dispatch.
<!-- keel-else -->
Use single-threaded task dispatch for this agent.
<!-- keel-endif -->

## Phase 2: Bug Fix — Include Near Conditional

This section tests that content between a snippet include and a conditional is preserved.

<!-- keel:include reasoning-protocol -->

Content between include and conditional MUST be preserved.
This line must not be silently eaten by the carve-out processor.

<!-- keel-if: keel.agent == "claude" -->
Claude-specific note after the include.
<!-- keel-else -->
Other agent note after the include.
<!-- keel-endif -->

## Auto-Injected Variables

Agent: ${keel:keel.agent}
Scope: ${keel:keel.scope}

## Parallel Dispatch (snippet with conditionals)

<!-- keel:include parallel-dispatch -->

## Deployment Note

<!-- keel:include deployment-note -->
