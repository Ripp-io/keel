---
name: claude-skill
description: Claude-native skill with all frontmatter fields
license: Apache-2.0
allowed-tools: ["Read", "Bash"]
metadata:
  author: keel-test
compatibility: "Claude Code 2.x"
disable-model-invocation: false
paths: ["**/*.py"]
when_to_use: Use when Python code needs review
argument-hint: "[module-name]"
arguments: ["module"]
user-invocable: true
model: sonnet
effort: high
context: fork
agent: code-reviewer
hooks:
  PreToolUse:
    - type: command
      command: echo skill-hook
shell: bash
---
Claude skill with ${CLAUDE_SKILL_DIR} reference.
Analyze the module specified by $ARGUMENTS.
