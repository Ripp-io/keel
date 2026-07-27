# Agent-Specific Configuration

<!-- keel-if: keel.agent == "claude" -->
Use Agent Teams for parallel execution. Spawn subagents via the Agent tool.
<!-- keel-elif: keel.agent == "cursor" -->
Use the coordination MCP server for parallel execution. Call mcp__coordinator__dispatch.
<!-- keel-else -->
Use sequential execution. No parallelism infrastructure available.
<!-- keel-endif -->

<!-- keel-if: keel.agent == "claude" -->
Claude-only tip: use /doctor for diagnostics across all repos.
<!-- keel-endif -->
