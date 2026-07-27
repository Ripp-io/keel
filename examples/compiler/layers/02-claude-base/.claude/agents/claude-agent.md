---
name: claude-agent
description: Claude-native agent definition
tools: ["Read", "Write", "Edit"]
disallowedTools: ["Bash"]
model: opus
permissionMode: acceptEdits
maxTurns: 20
skills: ["deploy"]
background: false
effort: xhigh
isolation: worktree
color: green
initialPrompt: Begin analysis
---
Claude-native agent from claude-base.
