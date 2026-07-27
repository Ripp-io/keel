---
name: code-reviewer
description: Reviews code for quality and security
tools: ["Read", "Grep", "Glob"]
disallowedTools: ["Bash"]
model: sonnet
permissionMode: default
maxTurns: 10
skills: ["security-review"]
mcpServers: ["sonarqube"]
hooks:
  PreToolUse:
    - type: command
      command: echo reviewing
memory: project
background: false
effort: high
isolation: worktree
color: blue
initialPrompt: Start by reading the diff
---
Code review agent from keel-base layer.
