package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Agent tools validation tests
// ---------------------------------------------------------------------------

func TestAgentTools_OverThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: ToolTest\nagents:\n  - source: agents/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create agent with 12 tools (default threshold is 10).
	agentMD := `---
name: test-agent
description: A test agent.
allowed-tools:
  - Read
  - Write
  - Bash
  - WebSearch
  - WebFetch
  - Agent
  - Edit
  - Grep
  - Glob
  - NotebookEdit
  - MultiEdit
  - TodoRead
---
Agent instructions here.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "agent.md"), []byte(agentMD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	assert.True(t, hasItem(result, false, true, "tool count:", "test-agent", "12 tools", "threshold: 10"),
		"expected warning about test-agent exceeding threshold")
}

func TestAgentTools_UnderThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: ToolTest\nagents:\n  - source: agents/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create agent with 5 tools (under default threshold of 10).
	agentMD := `---
name: small-agent
description: Small agent.
allowed-tools:
  - Read
  - Write
  - Bash
  - WebSearch
  - WebFetch
---
Agent instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "agent.md"), []byte(agentMD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	assert.True(t, hasItem(result, true, false, "tool count:", "within threshold"),
		"expected pass message for agent under threshold")
}

func TestAgentTools_ExactlyAtThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: ToolTest\nagents:\n  - source: agents/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create agent with exactly 10 tools (at threshold).
	agentMD := `---
name: exact-agent
description: Exactly at threshold.
allowed-tools:
  - Read
  - Write
  - Bash
  - WebSearch
  - WebFetch
  - Agent
  - Edit
  - Grep
  - Glob
  - NotebookEdit
---
Agent instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "agent.md"), []byte(agentMD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	// Threshold is >, not >=, so 10 should pass.
	assert.True(t, hasItem(result, true, false, "tool count:", "within threshold"),
		"expected pass for agent with exactly 10 tools")
}

func TestAgentTools_CustomThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := `schema: v1
name: ToolTest
agents:
  - source: agents/
validate:
  agent-tools: 5
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create agent with 7 tools (over custom threshold of 5).
	agentMD := `---
name: custom-agent
description: Test custom threshold.
allowed-tools:
  - Read
  - Write
  - Bash
  - WebSearch
  - WebFetch
  - Agent
  - Edit
---
Agent instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "agent.md"), []byte(agentMD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	assert.True(t, hasItem(result, false, true, "tool count:", "custom-agent", "7 tools", "threshold: 5"),
		"expected warning about custom-agent exceeding custom threshold of 5")
}

func TestAgentTools_Disabled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: ToolTest\nagents:\n  - source: agents/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create agent with 15 tools (well over threshold).
	agentMD := `---
name: disabled-agent
description: Check disabled.
allowed-tools:
  - Read
  - Write
  - Bash
  - WebSearch
  - WebFetch
  - Agent
  - Edit
  - Grep
  - Glob
  - NotebookEdit
  - MultiEdit
  - TodoRead
  - TaskCreate
  - RunCode
  - UploadFile
---
Agent instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "agent.md"), []byte(agentMD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: false})

	// No tool count messages should appear.
	for _, item := range result.Items {
		assert.NotContains(t, item.Message, "tool count:",
			"expected no tool count messages with AgentToolsCheck=false")
	}
}

func TestAgentTools_DisabledViaKeelYml(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := `schema: v1
name: ToolTest
agents:
  - source: agents/
validate:
  agent-tools: 0
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create agent with 15 tools (well over default threshold).
	agentMD := `---
name: config-disabled-agent
description: Disabled via keel.yml.
allowed-tools:
  - Read
  - Write
  - Bash
  - WebSearch
  - WebFetch
  - Agent
  - Edit
  - Grep
  - Glob
  - NotebookEdit
  - MultiEdit
  - TodoRead
  - TaskCreate
  - RunCode
  - UploadFile
---
Agent instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "agent.md"), []byte(agentMD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	// No tool count messages should appear (threshold 0 = disabled).
	for _, item := range result.Items {
		assert.NotContains(t, item.Message, "tool count:",
			"expected no tool count messages when agent-tools: 0 in keel.yml")
	}
}

func TestAgentTools_NoAgents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: NoAgents\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	// No tool count messages should appear (no agents found).
	for _, item := range result.Items {
		assert.NotContains(t, item.Message, "tool count:",
			"expected no tool count messages when no agents exist")
	}
}

func TestAgentTools_MultipleAgents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: MultiAgent\nagents:\n  - source: agents/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Agent 1: under threshold (5 tools).
	agent1MD := `---
name: agent-ok
description: Under threshold.
allowed-tools:
  - Read
  - Write
  - Bash
  - WebSearch
  - WebFetch
---
Agent 1 instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "agent1.md"), []byte(agent1MD), 0o644))

	// Agent 2: over threshold (12 tools).
	agent2MD := `---
name: agent-over
description: Over threshold.
allowed-tools:
  - Read
  - Write
  - Bash
  - WebSearch
  - WebFetch
  - Agent
  - Edit
  - Grep
  - Glob
  - NotebookEdit
  - MultiEdit
  - TodoRead
---
Agent 2 instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "agent2.md"), []byte(agent2MD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	// Should warn about agent-over but not agent-ok.
	assert.True(t, hasItem(result, false, true, "tool count:", "agent-over", "12 tools"),
		"expected warning about agent-over")

	// Should NOT have a pass message (not all under threshold).
	for _, item := range result.Items {
		if item.OK {
			assert.NotContains(t, item.Message, "tool count:", "within threshold",
				"expected no pass message when one agent is over threshold")
		}
	}
}

func TestAgentTools_NativeLayer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Native Claude layer (no keel.yml).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0o644))

	agentDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create agent with 12 tools.
	agentMD := `---
name: native-agent
description: Native layer agent.
allowed-tools:
  - Read
  - Write
  - Bash
  - WebSearch
  - WebFetch
  - Agent
  - Edit
  - Grep
  - Glob
  - NotebookEdit
  - MultiEdit
  - TodoRead
---
Native agent instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "agent.md"), []byte(agentMD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	assert.True(t, hasItem(result, false, true, "tool count:", "native-agent", "12 tools"),
		"expected warning for native layer agent")
}

func TestAgentTools_NoFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: NoFM\nagents:\n  - source: agents/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create agent markdown without frontmatter.
	agentMD := "# Agent Without Frontmatter\n\nInstructions here.\n"
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "nofm.md"), []byte(agentMD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	// Should not crash, and no tool count messages.
	assert.Equal(t, 0, result.ErrorCount, "no frontmatter should not cause errors")
	for _, item := range result.Items {
		assert.NotContains(t, item.Message, "tool count:",
			"expected no tool count messages for agents without frontmatter")
	}
}

func TestAgentTools_NoAllowedTools(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: NoTools\nagents:\n  - source: agents/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	// Create agent with frontmatter but no allowed-tools field.
	agentMD := `---
name: no-tools-agent
description: Agent without allowed-tools.
---
Agent instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "notools.md"), []byte(agentMD), 0o644))

	result := Run(dir, Options{AgentToolsCheck: true})

	// Should not crash or warn (no allowed-tools field).
	for _, item := range result.Items {
		assert.NotContains(t, item.Message, "tool count:",
			"expected no tool count messages for agents without allowed-tools")
	}
}
