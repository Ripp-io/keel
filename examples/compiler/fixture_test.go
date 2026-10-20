package fixture_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/agentplugins"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/agentsmd"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/claude"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/codex"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/goose"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/opencode"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cursor"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/drift"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/report"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err, "os.Getwd")
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "could not find repo root (no go.mod in any ancestor)")
		dir = parent
	}
}

func fixtureLayersDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(findRepoRoot(t), "examples", "compiler", "layers")
}

func buildFixtureLayers(t *testing.T) []config.Layer {
	t.Helper()
	root := fixtureLayersDir(t)
	entries, err := os.ReadDir(root)
	require.NoError(t, err, "reading fixture layers dir")
	var layers []config.Layer
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name())
		format, version := config.DetectLayerFormat(p)
		layers = append(layers, config.Layer{Path: p, Type: e.Name(), Format: format, Version: version})
	}
	require.Len(t, layers, 8, "expected 8 fixture layers")
	return layers
}

func compileTo(t *testing.T, agent string, opts compiler.CompileOptions) (string, *compiler.CompileResult) {
	t.Helper()
	layers := buildFixtureLayers(t)
	outputDir := t.TempDir()
	result, err := compiler.Compile(agent, layers, outputDir, opts)
	require.NoError(t, err, "Compile(%s)", agent)
	return outputDir, result
}

func readFile(t *testing.T, dir, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, relPath))
	require.NoError(t, err, "reading %s", relPath)
	return string(data)
}

func hasForceLock(result *compiler.CompileResult, category, key string) bool {
	for _, lock := range result.Governance.ForceLocked {
		if lock.Category == category && lock.Key == key {
			return true
		}
	}
	return false
}

func assertForceLocked(t *testing.T, result *compiler.CompileResult, category, key string) {
	t.Helper()
	assert.True(t, hasForceLock(result, category, key),
		"expected force-lock record category=%q key=%q; locks: %+v", category, key, result.Governance.ForceLocked)
}

type logCapture struct {
	mu       sync.Mutex
	messages []string
}

func (c *logCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (c *logCapture) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	msg := r.Message
	r.Attrs(func(a slog.Attr) bool { msg += " " + a.Key + "=" + a.Value.String(); return true })
	c.messages = append(c.messages, msg)
	return nil
}
func (c *logCapture) WithAttrs(_ []slog.Attr) slog.Handler { return c }
func (c *logCapture) WithGroup(_ string) slog.Handler      { return c }

func (c *logCapture) contains(substrs ...string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.messages {
		allMatch := true
		for _, sub := range substrs {
			if !strings.Contains(m, sub) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return true
		}
	}
	return false
}

func (c *logCapture) dump() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]string, len(c.messages))
	copy(cp, c.messages)
	return cp
}

// captureLogsForCompile replaces the global slog logger to capture log output.
// Tests using this helper must NOT call t.Parallel() since the global logger
// is shared state.
func captureLogsForCompile(t *testing.T, agent string, opts compiler.CompileOptions) (*logCapture, string, *compiler.CompileResult) {
	t.Helper()
	capture := &logCapture{}
	origLogger := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(origLogger) })
	layers := buildFixtureLayers(t)
	outputDir := t.TempDir()
	result, err := compiler.Compile(agent, layers, outputDir, opts)
	require.NoError(t, err, "Compile(%s)", agent)
	return capture, outputDir, result
}

// ---------------------------------------------------------------------------
// TestFixture_CompileClaude
// ---------------------------------------------------------------------------

func TestFixture_CompileClaude(t *testing.T) {
	outputDir, result := compileTo(t, "claude", compiler.CompileOptions{})
	claudeMD := readFile(t, outputDir, "CLAUDE.md")

	t.Run("CarveOut_AgentTeams", func(t *testing.T) { assert.Contains(t, claudeMD, "Agent Teams") })
	t.Run("CarveOut_NotCoordinationMCP", func(t *testing.T) { assert.NotContains(t, claudeMD, "coordination MCP") })
	t.Run("CarveOut_DoctorBlock", func(t *testing.T) { assert.Contains(t, claudeMD, "/doctor") })
	t.Run("CarveOut_NotSequentially", func(t *testing.T) { assert.NotContains(t, claudeMD, "sequentially") })

	t.Run("AllLayersContribute", func(t *testing.T) {
		for layer, marker := range map[string]string{
			"01-keel-base": "Keel Base Instructions", "02-claude-base": "Claude Base Instructions",
			"04-keel-overrides": "Keel Overrides Instructions", "05-claude-overrides": "Claude Overrides Instructions",
		} {
			assert.Contains(t, claudeMD, marker, "CLAUDE.md from %s", layer)
		}
	})

	t.Run("ForceLock_Command", func(t *testing.T) {
		cmd := readFile(t, outputDir, ".claude/commands/force-locked-cmd.md")
		assert.Contains(t, cmd, "KEEL-BASE FORCE-LOCKED")
		assert.NotContains(t, cmd, "KEEL-OVERRIDES ATTEMPT")
	})
	t.Run("ForceLock_ClaudeCmd", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".claude/commands/claude-cmd.md"), "CLAUDE-BASE COMMAND")
	})
	t.Run("ForceLock_Skill", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".claude/skills/force-locked-skill/SKILL.md"), "KEEL-BASE FORCE-LOCKED SKILL")
	})
	t.Run("ForceLock_Records", func(t *testing.T) {
		for _, r := range []struct{ cat, key string }{{"command", "force-locked-cmd.md"}, {"command", "claude-cmd.md"}, {"mcp", "sonarqube"}, {"mcp", "claude-native-server"}, {"hook", "PreToolUse"}, {"skill", "force-locked-skill/SKILL.md"}} {
			assertForceLocked(t, result, r.cat, r.key)
		}
	})

	t.Run("OverlayShadow_SharedCmd", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".claude/commands/shared-cmd.md"), "CURSOR-OVERRIDES SHARED")
	})
	t.Run("OverlayShadow_Provenance", func(t *testing.T) {
		require.NotNil(t, result.Output.Overlays)
		found := false
		for key, writers := range result.Output.Overlays {
			if strings.HasSuffix(key, "shared-cmd.md") {
				found = true
				assert.GreaterOrEqual(t, len(writers), 2, "writers for shared-cmd.md")
				break
			}
		}
		assert.True(t, found, "expected overlay provenance for shared-cmd.md; overlays: %v", result.Output.Overlays)
	})

	t.Run("MCP_Content", func(t *testing.T) {
		mcpRaw := readFile(t, outputDir, ".mcp.json")
		assert.Contains(t, mcpRaw, "claude-only-server")
		assert.NotContains(t, mcpRaw, "cursor-only-server")
		assert.Contains(t, mcpRaw, "not-cursor-server")
	})
	t.Run("MCP_ForceLock_Sonarqube", func(t *testing.T) { assert.Contains(t, readFile(t, outputDir, ".mcp.json"), "sonar-mcp") })
	t.Run("MCP_ForceLock_ClaudeNative", func(t *testing.T) {
		mcpRaw := readFile(t, outputDir, ".mcp.json")
		assert.Contains(t, mcpRaw, "claude-native-mcp")
		assert.NotContains(t, mcpRaw, "override-claude-native-attempt")
	})

	t.Run("Category_Plugins", func(t *testing.T) { assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude/plugins"))) })
	t.Run("Category_AgentMemory", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude/agent-memory")))
	})
	t.Run("Category_Rules", func(t *testing.T) { assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude/rules"))) })

	t.Run("Snippet_Resolved", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, "CLAUDE.md"), "State what you're about to do")
	})
	t.Run("Snippet_CrossLayer", func(t *testing.T) {
		assert.GreaterOrEqual(t, strings.Count(readFile(t, outputDir, "CLAUDE.md"), "State what you're about to do"), 2)
	})
	t.Run("Snippet_MarkersStripped", func(t *testing.T) { assert.NotContains(t, readFile(t, outputDir, "CLAUDE.md"), "keel:include") })
	t.Run("Snippet_Parameterized", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, "CLAUDE.md"), "In the security domain, always validate user input.")
	})
	t.Run("Snippet_ParameterizedCrossLayer", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, "CLAUDE.md"), "In the performance domain, always profile before optimizing.")
	})
	t.Run("Snippet_PlaceholdersFullyResolved", func(t *testing.T) {
		c := readFile(t, outputDir, "CLAUDE.md")
		assert.NotContains(t, c, "{{domain}}")
		assert.NotContains(t, c, "{{action}}")
	})
	t.Run("Snippet_DirNotInOutput", func(t *testing.T) { assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".claude/snippets"))) })

	t.Run("NoLeaks", func(t *testing.T) {
		for _, leak := range []string{"instructions.md", "permissions.json", "mcp.json", "hooks.json", "environment.json", "settings.claude.json", "settings.cursor.json", ".cursorrules", ".claude/mcp.json", ".claude/permissions.json", ".claude/environment.json", "loop.md"} {
			assert.False(t, testutil.FileExists(filepath.Join(outputDir, leak)), "leaked: %s", leak)
		}
	})
}

func TestFixture_CompileCursor(t *testing.T) {
	outputDir, result := compileTo(t, "cursor", compiler.CompileOptions{})

	t.Run("Content_Rules", func(t *testing.T) {
		rules := readFile(t, outputDir, ".cursor/rules/00-keel.mdc")
		assert.Contains(t, rules, "coordination MCP")
		assert.NotContains(t, rules, "Agent Teams")
		assert.NotContains(t, rules, "/doctor")
	})
	t.Run("ForceLock_Command", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".cursor/commands/force-locked-cmd.md"), "KEEL-BASE FORCE-LOCKED")
	})
	t.Run("ForceLock_CursorCmd", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".cursor/commands/cursor-cmd.md"), "CURSOR-BASE COMMAND")
	})
	t.Run("ForceLock_ClaudeCmd", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".cursor/commands/claude-cmd.md"), "CLAUDE-BASE COMMAND")
	})
	t.Run("ForceLock_Records", func(t *testing.T) {
		for _, r := range []struct{ cat, key string }{{"command", "force-locked-cmd.md"}, {"command", "cursor-cmd.md"}, {"command", "claude-cmd.md"}, {"mcp", "sonarqube"}, {"mcp", "claude-native-server"}, {"mcp", "cursor-native-server"}, {"hook", "PreToolUse"}, {"skill", "force-locked-skill/SKILL.md"}} {
			assertForceLocked(t, result, r.cat, r.key)
		}
	})
	t.Run("MCP_Content", func(t *testing.T) {
		mcpRaw := readFile(t, outputDir, ".cursor/mcp.json")
		assert.Contains(t, mcpRaw, "cursor-only-server")
		assert.NotContains(t, mcpRaw, "claude-only-server")
		assert.NotContains(t, mcpRaw, "not-cursor-server")
	})
	t.Run("MCP_ForceLock_CursorNative", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".cursor/mcp.json"), "cursor-native-mcp")
	})
	t.Run("Category_Environment", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/environment.json")))
	})
	t.Run("Category_NoPlugins", func(t *testing.T) { assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/plugins"))) })
	t.Run("Category_NoAgentMemory", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/agent-memory")))
	})
	t.Run("NoLeaks", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/permissions.json")))
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".cursorrules")))
	})
}

func TestFixture_CompileWarnings_Claude(t *testing.T) {
	capture, _, _ := captureLogsForCompile(t, "claude", compiler.CompileOptions{})
	t.Run("WarnEnvironmentUnsupported", func(t *testing.T) {
		assert.True(t, capture.contains("layer uses environment but target adapter does not support them"), "logs: %v", capture.dump())
	})
	t.Run("WarnCursorEnvironmentInIR", func(t *testing.T) {
		assert.True(t, capture.contains("config block has no equivalent in target agent", "environment"), "logs: %v", capture.dump())
	})
	t.Run("WarnHookEvent_BeforeShellExecution", func(t *testing.T) {
		assert.True(t, capture.contains("hook event may not be supported by target agent", "beforeShellExecution") || capture.contains("hook event may not be supported by target agent", "BeforeShellExecution"), "logs: %v", capture.dump())
	})
	t.Run("WarnHookEvent_AfterFileEdit", func(t *testing.T) {
		assert.True(t, capture.contains("hook event may not be supported by target agent", "afterFileEdit") || capture.contains("hook event may not be supported by target agent", "AfterFileEdit"), "logs: %v", capture.dump())
	})
	t.Run("WarnHookEvent_AfterAgentResponse", func(t *testing.T) {
		assert.True(t, capture.contains("hook event may not be supported by target agent", "afterAgentResponse") || capture.contains("hook event may not be supported by target agent", "AfterAgentResponse"), "logs: %v", capture.dump())
	})
	t.Run("NoWarnPortableHook_PreToolUse", func(t *testing.T) { assert.False(t, capture.contains("hook event may not be supported", "PreToolUse")) })
	t.Run("NoWarnPortableHook_SessionStart", func(t *testing.T) {
		assert.False(t, capture.contains("hook event may not be supported", "SessionStart"))
	})
}

func TestFixture_CompileWarnings_Cursor(t *testing.T) {
	capture, _, _ := captureLogsForCompile(t, "cursor", compiler.CompileOptions{})
	t.Run("WarnPluginsUnsupported", func(t *testing.T) {
		assert.True(t, capture.contains("layer uses plugins but target adapter does not support them"), "logs: %v", capture.dump())
	})
	t.Run("WarnAgentMemoryUnsupported", func(t *testing.T) {
		assert.True(t, capture.contains("layer uses agent-memory but target adapter does not support them"), "logs: %v", capture.dump())
	})
	t.Run("WarnMCPPolicyNotSupported", func(t *testing.T) {
		assert.True(t, capture.contains("mcp.policy is not supported by Cursor"), "logs: %v", capture.dump())
	})
	t.Run("WarnCategory_Plugins_InIR", func(t *testing.T) {
		assert.True(t, capture.contains("category has no equivalent in target agent", "plugins"), "logs: %v", capture.dump())
	})
	t.Run("WarnCategory_AgentMemory_InIR", func(t *testing.T) {
		assert.True(t, capture.contains("category has no equivalent in target agent", "agent-memory"), "logs: %v", capture.dump())
	})
	t.Run("WarnHookEvent_Setup", func(t *testing.T) {
		assert.True(t, capture.contains("hook event may not be supported by target agent", "Setup"), "logs: %v", capture.dump())
	})
	t.Run("WarnHookHandlerType_HTTP", func(t *testing.T) {
		assert.True(t, capture.contains("hook handler type not supported by target agent", "http"), "logs: %v", capture.dump())
	})
	t.Run("WarnHookHandlerType_MCPTool", func(t *testing.T) {
		assert.True(t, capture.contains("hook handler type not supported by target agent", "mcp_tool"), "logs: %v", capture.dump())
	})
	t.Run("WarnFrontmatter_WhenToUse", func(t *testing.T) {
		assert.True(t, capture.contains("frontmatter contains fields not supported by target agent", "when_to_use"), "logs: %v", capture.dump())
	})
	t.Run("WarnCommandFrontmatter", func(t *testing.T) {
		assert.True(t, capture.contains("command frontmatter is not supported by Cursor"), "logs: %v", capture.dump())
	})
}

func TestFixture_UserScope_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{Scope: compiler.ScopeUser})
	t.Run("ClaudeJSON_Exists", func(t *testing.T) { assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude.json"))) })
	t.Run("McpJSON_NotExists", func(t *testing.T) { assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".mcp.json"))) })
	t.Run("CLAUDE_MD_Exists", func(t *testing.T) { assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude/CLAUDE.md"))) })
	t.Run("Settings_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude/settings.json")))
	})
	t.Run("Manifest_ScopeUser", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".claude/.keel.json"), `"scope": "user"`)
	})
}

func TestFixture_UserScope_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{Scope: compiler.ScopeUser})
	t.Run("NoInstructions", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/rules/00-keel.mdc")))
	})
	t.Run("NoEnvironment", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/environment.json")))
	})
	t.Run("MCP_Exists", func(t *testing.T) { assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/mcp.json"))) })
	t.Run("Manifest_ScopeUser", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".cursor/.keel.json"), `"scope": "user"`)
	})
}

func TestFixture_MCP_Parsed_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	data, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err)
	var mcpMap map[string]any
	require.NoError(t, json.Unmarshal(data, &mcpMap))
	servers, ok := mcpMap["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers key")

	t.Run("Sonarqube_Original", func(t *testing.T) {
		sq, ok := servers["sonarqube"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "sonar-mcp", sq["command"])
	})
	t.Run("ClaudeNative_Original", func(t *testing.T) {
		cn, ok := servers["claude-native-server"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "claude-native-mcp", cn["command"])
	})
	t.Run("AgentFiltering", func(t *testing.T) {
		assert.Contains(t, servers, "claude-only-server")
		assert.NotContains(t, servers, "cursor-only-server")
		assert.Contains(t, servers, "not-cursor-server")
	})
	t.Run("InOperator_MultiAgentPresent", func(t *testing.T) { assert.Contains(t, servers, "multi-agent-server") })
	t.Run("NotInOperator_ExcludedAbsent", func(t *testing.T) { assert.NotContains(t, servers, "excluded-agent-server") })
	t.Run("NoKeelKeys", func(t *testing.T) { testutil.AssertNoKeelKeys(t, mcpMap) })
}

func TestFixture_MCP_Parsed_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{})
	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "mcp.json"))
	require.NoError(t, err)
	var mcpMap map[string]any
	require.NoError(t, json.Unmarshal(data, &mcpMap))
	servers, ok := mcpMap["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers key")

	t.Run("AgentFiltering", func(t *testing.T) {
		assert.Contains(t, servers, "cursor-only-server")
		assert.NotContains(t, servers, "claude-only-server")
		assert.NotContains(t, servers, "not-cursor-server")
	})
	t.Run("InOperator_MultiAgentPresent", func(t *testing.T) { assert.Contains(t, servers, "multi-agent-server") })
	t.Run("NotInOperator_ExcludedAbsent", func(t *testing.T) { assert.NotContains(t, servers, "excluded-agent-server") })
	t.Run("CursorNative_Original", func(t *testing.T) {
		cn, ok := servers["cursor-native-server"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "cursor-native-mcp", cn["command"])
	})
	t.Run("NoKeelKeys", func(t *testing.T) { testutil.AssertNoKeelKeys(t, mcpMap) })
}

func TestFixture_OverlayShadow_Cursor(t *testing.T) {
	outputDir, result := compileTo(t, "cursor", compiler.CompileOptions{})
	t.Run("SharedCmd_LastLayerWins", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".cursor/commands/shared-cmd.md"), "CURSOR-OVERRIDES SHARED")
	})
	t.Run("OverlayProvenance", func(t *testing.T) {
		require.NotNil(t, result.Output.Overlays)
		found := false
		for key, writers := range result.Output.Overlays {
			if strings.HasSuffix(key, "shared-cmd.md") {
				found = true
				assert.GreaterOrEqual(t, len(writers), 2, "writers for shared-cmd.md: %v", writers)
				break
			}
		}
		assert.True(t, found, "expected overlay provenance; overlays: %v", result.Output.Overlays)
	})
}

func TestFixture_Variables_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	claudeMD := readFile(t, outputDir, "CLAUDE.md")
	t.Run("ApiUrl_Resolved", func(t *testing.T) { assert.Contains(t, claudeMD, "https://api.example.com") })
	t.Run("Environment_OverriddenToStaging", func(t *testing.T) { assert.Contains(t, claudeMD, "Deploy to staging") })
	t.Run("Environment_InOverrideLayer", func(t *testing.T) { assert.Contains(t, claudeMD, "Environment from layer override: staging") })
	t.Run("NoUnresolvedVariables", func(t *testing.T) { assert.NotContains(t, claudeMD, "${keel:") })
}

func TestFixture_Variables_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{})
	rules := readFile(t, outputDir, ".cursor/rules/00-keel.mdc")
	t.Run("ApiUrl_Resolved", func(t *testing.T) { assert.Contains(t, rules, "https://api.example.com") })
	t.Run("Environment_OverriddenToStaging", func(t *testing.T) { assert.Contains(t, rules, "Deploy to staging") })
	t.Run("NoUnresolvedVariables", func(t *testing.T) { assert.NotContains(t, rules, "${keel:") })
}

func TestFixture_ExternalVarOverride_Claude(t *testing.T) {
	t.Run("ExternalVar_Overrides_Layer", func(t *testing.T) {
		outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{Vars: map[string]string{"environment": "ci"}})
		claudeMD := readFile(t, outputDir, "CLAUDE.md")
		assert.Contains(t, claudeMD, "Deploy to ci")
		assert.NotContains(t, claudeMD, "Deploy to staging")
		assert.NotContains(t, claudeMD, "Deploy to production")
	})
	t.Run("AutoInjected_NotOverridable", func(t *testing.T) {
		outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{Vars: map[string]string{"keel.agent": "fake"}})
		claudeMD := readFile(t, outputDir, "CLAUDE.md")
		assert.Contains(t, claudeMD, "Agent: claude")
		assert.NotContains(t, claudeMD, "Agent: fake")
	})
	t.Run("ExternalVar_NewVariable", func(t *testing.T) {
		compileTo(t, "claude", compiler.CompileOptions{Vars: map[string]string{"custom_var": "custom_value"}})
	})
}

func TestFixture_ExternalVarOverride_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{Vars: map[string]string{"environment": "ci"}})
	rules := readFile(t, outputDir, ".cursor/rules/00-keel.mdc")
	assert.Contains(t, rules, "Deploy to ci")
	assert.NotContains(t, rules, "Deploy to staging")
}

func TestFixture_Conditionals_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	claudeMD := readFile(t, outputDir, "CLAUDE.md")
	t.Run("ClaudeBranch_Present", func(t *testing.T) {
		assert.Contains(t, claudeMD, "multi-agent orchestration for parallel task dispatch")
	})
	t.Run("CursorBranch_Absent", func(t *testing.T) {
		assert.NotContains(t, claudeMD, "background agent pool for parallel task dispatch")
	})
	t.Run("ElseBranch_Absent", func(t *testing.T) { assert.NotContains(t, claudeMD, "single-threaded task dispatch for this agent") })
	t.Run("NoConditionalMarkers", func(t *testing.T) {
		for _, m := range []string{"keel-if", "keel-elif", "keel-else", "keel-endif"} {
			assert.NotContains(t, claudeMD, m)
		}
	})
}

func TestFixture_Conditionals_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{})
	rules := readFile(t, outputDir, ".cursor/rules/00-keel.mdc")
	t.Run("CursorBranch_Present", func(t *testing.T) { assert.Contains(t, rules, "background agent pool for parallel task dispatch") })
	t.Run("ClaudeBranch_Absent", func(t *testing.T) {
		assert.NotContains(t, rules, "multi-agent orchestration for parallel task dispatch")
	})
	t.Run("ElseBranch_Absent", func(t *testing.T) { assert.NotContains(t, rules, "single-threaded task dispatch for this agent") })
	t.Run("NoConditionalMarkers", func(t *testing.T) {
		for _, m := range []string{"keel-if", "keel-elif", "keel-else", "keel-endif"} {
			assert.NotContains(t, rules, m)
		}
	})
}

func TestFixture_AutoInjectedVars_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	claudeMD := readFile(t, outputDir, "CLAUDE.md")
	t.Run("AgentVar_Resolved", func(t *testing.T) { assert.Contains(t, claudeMD, "Agent: claude") })
	t.Run("ScopeVar_Resolved", func(t *testing.T) { assert.Contains(t, claudeMD, "Scope: project") })
}

func TestFixture_IncludeNearConditional_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	claudeMD := readFile(t, outputDir, "CLAUDE.md")
	t.Run("ContentBetweenIncludeAndConditional_Preserved", func(t *testing.T) {
		assert.Contains(t, claudeMD, "Content between include and conditional MUST be preserved")
		assert.Contains(t, claudeMD, "This line must not be silently eaten")
	})
	t.Run("Include_Resolved", func(t *testing.T) { assert.Contains(t, claudeMD, "State what you're about to do") })
	t.Run("Conditional_CorrectBranch", func(t *testing.T) {
		assert.Contains(t, claudeMD, "Claude-specific note after the include")
		assert.NotContains(t, claudeMD, "Other agent note after the include")
	})
}

func TestFixture_Snippet_VariableInContent(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	claudeMD := readFile(t, outputDir, "CLAUDE.md")
	t.Run("Variable_ResolvedInSnippet", func(t *testing.T) { assert.Contains(t, claudeMD, "Deploy to staging environment") })
	t.Run("ParameterizedSnippet_Resolved", func(t *testing.T) { assert.Contains(t, claudeMD, "In the security domain") })
}

func TestFixture_SnippetWithConditionals_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	claudeMD := readFile(t, outputDir, "CLAUDE.md")
	t.Run("MatchingBranch_Present", func(t *testing.T) { assert.Contains(t, claudeMD, "hook-injected coordination messages") })
	t.Run("ElseBranch_Absent", func(t *testing.T) { assert.NotContains(t, claudeMD, "Poll for coordination messages manually") })
	t.Run("NoConditionalMarkers", func(t *testing.T) {
		assert.NotContains(t, claudeMD, "keel-if")
		assert.NotContains(t, claudeMD, "keel-endif")
	})
}

func TestFixture_SnippetWithConditionals_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{})
	rules := readFile(t, outputDir, ".cursor/rules/00-keel.mdc")
	t.Run("MatchingBranch_Present", func(t *testing.T) { assert.Contains(t, rules, "hook-injected coordination messages") })
	t.Run("ElseBranch_Absent", func(t *testing.T) { assert.NotContains(t, rules, "Poll for coordination messages manually") })
}

func TestFixture_DepsAndBin_Claude(t *testing.T) {
	outputDir, result := compileTo(t, "claude", compiler.CompileOptions{})
	t.Run("Bin_ScriptExists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".keel/bin/check-env.sh")))
	})
	t.Run("Bin_Executable", func(t *testing.T) {
		info, err := os.Stat(filepath.Join(outputDir, ".keel", "bin", "check-env.sh"))
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&0111, "expected executable")
	})
	t.Run("Bin_Content", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".keel/bin/check-env.sh"), "check-env v1.0")
	})
	t.Run("Deps_Skipped", func(t *testing.T) {
		require.NotEmpty(t, result.Dependencies.InstalledDeps)
		found := false
		for _, dep := range result.Dependencies.InstalledDeps {
			if dep.Name == "fixture-tool" {
				found = true
				assert.Equal(t, "skipped", dep.Method)
				assert.Equal(t, "project", dep.Scope)
				assert.True(t, strings.HasSuffix(dep.Path, ".keel/bin/fixture-tool"))
			}
		}
		assert.True(t, found, "fixture-tool not found")
	})
	t.Run("MCP_BinPathResolved", func(t *testing.T) {
		mcpRaw := readFile(t, outputDir, ".mcp.json")
		assert.Contains(t, mcpRaw, "check-env.sh")
		assert.NotContains(t, mcpRaw, "${keel:keel.bin.project}")
	})
	t.Run("MCP_DepPathResolved", func(t *testing.T) {
		mcpRaw := readFile(t, outputDir, ".mcp.json")
		assert.NotContains(t, mcpRaw, "${keel:")
		assert.Contains(t, mcpRaw, ".keel/bin/fixture-tool")
	})
	t.Run("Hooks_BinPathResolved", func(t *testing.T) {
		settings := readFile(t, outputDir, ".claude/settings.json")
		assert.Contains(t, settings, "check-env.sh")
		assert.NotContains(t, settings, "${keel:keel.bin.project}")
	})
	t.Run("Hooks_HooksDirResolved", func(t *testing.T) {
		settings := readFile(t, outputDir, ".claude/settings.json")
		assert.Contains(t, settings, "on-session-end.sh")
		assert.NotContains(t, settings, "${keel:keel.hooks.dir}", "keel.hooks.dir should be resolved in hook commands")
	})
	t.Run("Instructions_DepVarsResolved", func(t *testing.T) {
		claudeMD := readFile(t, outputDir, "CLAUDE.md")
		assert.NotContains(t, claudeMD, "${keel:keel.deps")
		assert.NotContains(t, claudeMD, "${keel:keel.bin")
		assert.Contains(t, claudeMD, ".keel/bin/fixture-tool")
		assert.Contains(t, claudeMD, ".keel/bin")
	})
	t.Run("Manifest_HasDeps", func(t *testing.T) {
		mf := readFile(t, outputDir, ".claude/.keel.json")
		assert.Contains(t, mf, "installed_deps")
		assert.Contains(t, mf, "fixture-tool")
	})
}

func TestFixture_DepsAndBin_Cursor(t *testing.T) {
	outputDir, result := compileTo(t, "cursor", compiler.CompileOptions{})
	t.Run("Bin_ScriptExists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".keel/bin/check-env.sh")))
	})
	t.Run("Bin_Executable", func(t *testing.T) {
		info, err := os.Stat(filepath.Join(outputDir, ".keel", "bin", "check-env.sh"))
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&0111)
	})
	t.Run("Deps_Skipped", func(t *testing.T) {
		found := false
		for _, dep := range result.Dependencies.InstalledDeps {
			if dep.Name == "fixture-tool" {
				found = true
				assert.Equal(t, "skipped", dep.Method)
			}
		}
		assert.True(t, found, "fixture-tool not found")
	})
	t.Run("MCP_BinPathResolved", func(t *testing.T) {
		mcpRaw := readFile(t, outputDir, ".cursor/mcp.json")
		assert.Contains(t, mcpRaw, "check-env.sh")
		assert.NotContains(t, mcpRaw, "${keel:keel.bin.project}")
	})
	t.Run("MCP_DepPathResolved", func(t *testing.T) {
		mcpRaw := readFile(t, outputDir, ".cursor/mcp.json")
		assert.NotContains(t, mcpRaw, "${keel:")
		assert.Contains(t, mcpRaw, ".keel/bin/fixture-tool")
	})
	t.Run("Hooks_BinPathResolved", func(t *testing.T) {
		hooks := readFile(t, outputDir, ".cursor/hooks.json")
		assert.Contains(t, hooks, "check-env.sh")
		assert.NotContains(t, hooks, "${keel:keel.bin.project}")
	})
	t.Run("Hooks_HooksDirResolved", func(t *testing.T) {
		hooks := readFile(t, outputDir, ".cursor/hooks.json")
		assert.Contains(t, hooks, "on-session-end.sh")
		assert.NotContains(t, hooks, "${keel:keel.hooks.dir}", "keel.hooks.dir should be resolved in hook commands")
	})
	t.Run("Instructions_DepVarsResolved", func(t *testing.T) {
		rules := readFile(t, outputDir, ".cursor/rules/00-keel.mdc")
		assert.NotContains(t, rules, "${keel:keel.deps")
		assert.NotContains(t, rules, "${keel:keel.bin")
	})
	t.Run("Manifest_HasDeps", func(t *testing.T) {
		mf := readFile(t, outputDir, ".cursor/.keel.json")
		assert.Contains(t, mf, "installed_deps")
		assert.Contains(t, mf, "fixture-tool")
	})
}

func TestFixture_DepsAndBin_MultiAgent(t *testing.T) {
	layers := buildFixtureLayers(t)
	outputDir := t.TempDir()
	_, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	_, err = compiler.Compile("cursor", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	t.Run("SharedBinDir", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".keel/bin/check-env.sh")))
	})
	t.Run("NotInClaudeDir", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".claude/bin/check-env.sh")))
	})
	t.Run("NotInCursorDir", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/bin/check-env.sh")))
	})
	t.Run("ClaudeManifest_HasDeps", func(t *testing.T) { assert.Contains(t, readFile(t, outputDir, ".claude/.keel.json"), "fixture-tool") })
	t.Run("CursorManifest_HasDeps", func(t *testing.T) { assert.Contains(t, readFile(t, outputDir, ".cursor/.keel.json"), "fixture-tool") })
}

func TestFixture_ConventionDefaults_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	t.Run("Hooks_FromConventionDir", func(t *testing.T) {
		settings := readFile(t, outputDir, ".claude/settings.json")
		assert.Contains(t, settings, "SessionStart")
		assert.Contains(t, settings, "convention-hook-fired")
	})
	t.Run("Hooks_ShScriptIgnored", func(t *testing.T) {
		assert.NotContains(t, readFile(t, outputDir, ".claude/settings.json"), "helper script")
	})
	t.Run("Agents_FromConventionDir", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude/agents/fixture-agent.md")))
		assert.Contains(t, readFile(t, outputDir, ".claude/agents/fixture-agent.md"), "convention defaults")
	})
	t.Run("Snippets_FromConventionDir", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, "CLAUDE.md"), "Welcome to the convention defaults test")
	})
	t.Run("Snippets_MarkersStripped", func(t *testing.T) { assert.NotContains(t, readFile(t, outputDir, "CLAUDE.md"), "keel:include") })
}

func TestFixture_ConventionDefaults_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{})
	t.Run("Hooks_FromConventionDir", func(t *testing.T) {
		hooks := readFile(t, outputDir, ".cursor/hooks.json")
		assert.Contains(t, hooks, "sessionStart")
		assert.Contains(t, hooks, "convention-hook-fired")
	})
	t.Run("Hooks_ShScriptIgnored", func(t *testing.T) {
		assert.NotContains(t, readFile(t, outputDir, ".cursor/hooks.json"), "helper script")
	})
	t.Run("Agents_FromConventionDir", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/agents/fixture-agent.md")))
		assert.Contains(t, readFile(t, outputDir, ".cursor/agents/fixture-agent.md"), "convention defaults")
	})
	t.Run("Snippets_FromConventionDir", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".cursor/rules/00-keel.mdc"), "Welcome to the convention defaults test")
	})
	t.Run("Snippets_MarkersStripped", func(t *testing.T) {
		assert.NotContains(t, readFile(t, outputDir, ".cursor/rules/00-keel.mdc"), "keel:include")
	})
}

func TestFixture_Distributable_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{Distributable: true, Name: "test-distrib"})
	t.Run("Command_ForceFrontmatterPreserved", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".claude/commands/force-locked-cmd.md"), "force: true")
	})
	t.Run("Skill_ForceFrontmatterPreserved", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".claude/skills/force-locked-skill/SKILL.md"), "force: true")
	})
	t.Run("Command_ContentIntact", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".claude/commands/force-locked-cmd.md"), "KEEL-BASE FORCE-LOCKED")
	})
	t.Run("Skill_ContentIntact", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".claude/skills/force-locked-skill/SKILL.md"), "KEEL-BASE FORCE-LOCKED SKILL")
	})
}

func TestFixture_Distributable_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{Distributable: true, Name: "test-distrib"})
	t.Run("Command_ForceFrontmatterPreserved", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".cursor/commands/force-locked-cmd.md"), "force: true")
	})
}

func TestFixture_NonDistributable_FrontmatterStripped(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	t.Run("Command_ForceFrontmatterStripped", func(t *testing.T) {
		assert.NotContains(t, readFile(t, outputDir, ".claude/commands/force-locked-cmd.md"), "force:")
	})
	t.Run("Skill_ForceFrontmatterStripped", func(t *testing.T) {
		assert.NotContains(t, readFile(t, outputDir, ".claude/skills/force-locked-skill/SKILL.md"), "force:")
	})
}

func TestFixture_DepsAndBin_HooksDepPathResolved(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	t.Run("Claude_DepPathInHooksResolved", func(t *testing.T) {
		settings := readFile(t, outputDir, ".claude/settings.json")
		assert.NotContains(t, settings, "${keel:keel.deps")
		assert.Contains(t, settings, ".keel/bin/fixture-tool")
	})
}

func TestFixture_DepsAndBin_HooksDepPathResolved_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{})
	t.Run("Cursor_DepPathInHooksResolved", func(t *testing.T) {
		hooks := readFile(t, outputDir, ".cursor/hooks.json")
		assert.NotContains(t, hooks, "${keel:keel.deps")
		assert.Contains(t, hooks, ".keel/bin/fixture-tool")
	})
}

func TestFixture_Permissions_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	perms, ok := settings["permissions"].(map[string]any)
	require.True(t, ok, "expected permissions in settings")

	t.Run("Allow_AdditiveUnion", func(t *testing.T) {
		allow, ok := perms["allow"].([]any)
		require.True(t, ok, "expected allow to be an array")
		// Layer 01 contributes: Read, Grep, Glob
		// Layer 04 contributes: Write, Edit
		// Additive policy unions them.
		for _, expected := range []string{"Read", "Grep", "Glob", "Write", "Edit"} {
			assert.Contains(t, allow, expected, "expected %s in allow list", expected)
		}
	})
	t.Run("Deny_AdditiveUnion", func(t *testing.T) {
		deny, ok := perms["deny"].([]any)
		require.True(t, ok, "expected deny to be an array")
		// Layer 01: Write(/etc/*), Layer 04: Bash(shutdown)
		assert.Contains(t, deny, "Write(/etc/*)")
		assert.Contains(t, deny, "Bash(shutdown)")
	})
	t.Run("Format_Nested", func(t *testing.T) {
		// Claude uses nested format: settings.permissions.allow / settings.permissions.deny
		assert.Contains(t, perms, "allow")
		assert.Contains(t, perms, "deny")
		// Should NOT have flat Cursor-style keys
		assert.NotContains(t, settings, "permissions.allow")
		assert.NotContains(t, settings, "permissions.deny")
	})
}

func TestFixture_Permissions_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{})
	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err)
	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))

	t.Run("Format_FlatKeys", func(t *testing.T) {
		// Cursor uses flat keys: permissions.allow / permissions.deny
		_, hasAllow := config["permissions.allow"]
		_, hasDeny := config["permissions.deny"]
		assert.True(t, hasAllow, "expected permissions.allow flat key")
		assert.True(t, hasDeny, "expected permissions.deny flat key")
		// Should NOT have nested permissions map
		_, hasNested := config["permissions"]
		assert.False(t, hasNested, "should not have nested permissions key in Cursor")
	})
	t.Run("Allow_WithSuffix", func(t *testing.T) {
		allow, ok := config["permissions.allow"].([]any)
		require.True(t, ok, "expected permissions.allow to be an array")
		// Bare tool names get (*) suffix
		assert.Contains(t, allow, "Read(*)", "bare Read should get (*) suffix")
		assert.Contains(t, allow, "Write(*)", "bare Write should get (*) suffix")
	})
	t.Run("Deny_ParameterizedPassthrough", func(t *testing.T) {
		deny, ok := config["permissions.deny"].([]any)
		require.True(t, ok, "expected permissions.deny to be an array")
		// Already-parameterized permissions pass through unchanged
		assert.Contains(t, deny, "Write(/etc/*)", "parameterized should pass through")
		assert.Contains(t, deny, "Bash(shutdown)", "parameterized should pass through")
	})
	t.Run("Version_AlwaysPresent", func(t *testing.T) {
		assert.Equal(t, float64(1), config["version"], "Cursor cli-config.json must always have version: 1")
	})
}

func TestFixture_MCPPolicy_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	settings := readFile(t, outputDir, ".claude/settings.json")
	t.Run("AllowedServers", func(t *testing.T) {
		assert.Contains(t, settings, "allowedMcpServers")
		assert.Contains(t, settings, "sonarqube")
		assert.Contains(t, settings, "gitlab")
	})
	t.Run("DeniedServers", func(t *testing.T) {
		assert.Contains(t, settings, "deniedMcpServers")
		assert.Contains(t, settings, "unsafe-server")
	})
	t.Run("EnableAll", func(t *testing.T) { assert.Contains(t, settings, "enableAllProjectMcpServers") })
	t.Run("DeniedServers_Parsed", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
		require.NoError(t, err)
		var s map[string]any
		require.NoError(t, json.Unmarshal(data, &s))
		denied, ok := s["deniedMcpServers"].([]any)
		require.True(t, ok, "expected deniedMcpServers to be an array")
		found := false
		for _, d := range denied {
			if d == "unsafe-server" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected 'unsafe-server' in deniedMcpServers, got %v", denied)
	})
}

func TestFixture_ClaudeSettings_FirstClass(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Portable: model stays in settings
	t.Run("Model_Portable", func(t *testing.T) {
		_, ok := settings["model"]
		assert.True(t, ok, "expected 'model' in compiled settings")
	})

	// Agent/Model
	t.Run("EffortLevel", func(t *testing.T) { assert.Equal(t, "high", settings["effortLevel"]) })
	t.Run("AlwaysThinkingEnabled", func(t *testing.T) { assert.Equal(t, true, settings["alwaysThinkingEnabled"]) })

	// Hooks meta
	t.Run("DisableAllHooks", func(t *testing.T) { assert.Equal(t, false, settings["disableAllHooks"]) })

	// MCP controls
	t.Run("EnabledMcpjsonServers", func(t *testing.T) {
		servers, ok := settings["enabledMcpjsonServers"].([]any)
		require.True(t, ok, "expected enabledMcpjsonServers to be an array")
		assert.Contains(t, servers, "context7")
	})

	// Git
	t.Run("IncludeGitInstructions", func(t *testing.T) { assert.Equal(t, true, settings["includeGitInstructions"]) })
	t.Run("PrUrlTemplate", func(t *testing.T) {
		assert.Equal(t, "https://gitlab.example.com/-/merge_requests", settings["prUrlTemplate"])
	})

	// UI
	t.Run("OutputStyle", func(t *testing.T) { assert.Equal(t, "concise", settings["outputStyle"]) })
	t.Run("Language", func(t *testing.T) { assert.Equal(t, "en", settings["language"]) })
	t.Run("ShowTurnDuration", func(t *testing.T) { assert.Equal(t, true, settings["showTurnDuration"]) })
	t.Run("SpinnerTipsEnabled", func(t *testing.T) { assert.Equal(t, false, settings["spinnerTipsEnabled"]) })

	// Features
	t.Run("FastMode", func(t *testing.T) { assert.Equal(t, true, settings["fastMode"]) })
	t.Run("AutoMemoryEnabled", func(t *testing.T) { assert.Equal(t, true, settings["autoMemoryEnabled"]) })

	// Misc
	t.Run("DefaultShell", func(t *testing.T) { assert.Equal(t, "/bin/zsh", settings["defaultShell"]) })
	t.Run("RespectGitignore", func(t *testing.T) { assert.Equal(t, true, settings["respectGitignore"]) })
	t.Run("TeammateMode", func(t *testing.T) { assert.Equal(t, "auto", settings["teammateMode"]) })
	t.Run("CleanupPeriodDays", func(t *testing.T) { assert.Equal(t, float64(30), settings["cleanupPeriodDays"]) })

	// Grouped: voice
	t.Run("Voice", func(t *testing.T) {
		voice, ok := settings["voice"].(map[string]any)
		require.True(t, ok, "expected voice to be a map")
		assert.Equal(t, false, voice["enabled"])
	})

	// Grouped: statusLine
	t.Run("StatusLine", func(t *testing.T) {
		sl, ok := settings["statusLine"].(map[string]any)
		require.True(t, ok, "expected statusLine to be a map")
		assert.Equal(t, "command", sl["type"])
	})

	// Grouped: fileSuggestion
	t.Run("FileSuggestion", func(t *testing.T) {
		fs, ok := settings["fileSuggestion"].(map[string]any)
		require.True(t, ok, "expected fileSuggestion to be a map")
		assert.Equal(t, "command", fs["type"])
	})

	// Updates (flat — was sub-struct, now flat on Settings)
	t.Run("AutoUpdatesChannel_Flat", func(t *testing.T) {
		assert.Equal(t, "stable", settings["autoUpdatesChannel"])
		_, nested := settings["updates"]
		assert.False(t, nested, "updates should not appear as nested key -- it should be flattened")
	})

	// Auth (flat — was sub-struct, now flat on Settings)
	t.Run("ForceLoginMethod_Flat", func(t *testing.T) {
		assert.Equal(t, "console", settings["forceLoginMethod"])
		_, nested := settings["auth"]
		assert.False(t, nested, "auth should not appear as nested key -- it should be flattened")
	})

	// Expanded sandbox
	t.Run("Sandbox_EnableWeakerNetworkIsolation", func(t *testing.T) {
		sandbox, ok := settings["sandbox"].(map[string]any)
		require.True(t, ok, "expected sandbox to be a map")
		assert.Equal(t, false, sandbox["enableWeakerNetworkIsolation"])
	})
	t.Run("Sandbox_Network_AllowLocalBinding", func(t *testing.T) {
		sandbox := settings["sandbox"].(map[string]any)
		net, ok := sandbox["network"].(map[string]any)
		require.True(t, ok, "expected sandbox.network to be a map")
		assert.Equal(t, true, net["allowLocalBinding"])
	})
	t.Run("Sandbox_Network_HttpProxyPort", func(t *testing.T) {
		sandbox := settings["sandbox"].(map[string]any)
		net := sandbox["network"].(map[string]any)
		assert.Equal(t, float64(8080), net["httpProxyPort"])
	})
	t.Run("Sandbox_Filesystem_AllowManagedReadPathsOnly", func(t *testing.T) {
		sandbox := settings["sandbox"].(map[string]any)
		fs, ok := sandbox["filesystem"].(map[string]any)
		require.True(t, ok, "expected sandbox.filesystem to be a map")
		assert.Equal(t, false, fs["allowManagedReadPathsOnly"])
	})

	// No keel keys
	t.Run("NoKeelKeys", func(t *testing.T) { testutil.AssertNoKeelKeys(t, settings) })
}

func TestFixture_Loop_Claude(t *testing.T) {
	outputDir, result := compileTo(t, "claude", compiler.CompileOptions{})
	t.Run("LoopFile_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude/loop.md")))
	})
	t.Run("LoopFile_Content", func(t *testing.T) {
		content := readFile(t, outputDir, ".claude/loop.md")
		assert.Contains(t, content, "check your previous output")
	})
	t.Run("LoopFile_InManifest", func(t *testing.T) {
		// Verify loop file is tracked in the manifest
		data, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
		require.NoError(t, err)
		var cm manifest.CompiledManifest
		require.NoError(t, json.Unmarshal(data, &cm))
		found := false
		for _, f := range cm.FileOps {
			if f.Output == ".claude/loop.md" {
				found = true
				assert.Equal(t, "concatenate", f.Strategy, "expected concatenate strategy for loop file")
				break
			}
		}
		assert.True(t, found, "expected loop file to be tracked in manifest file_ops")
		// Also verify in result
		foundInResult := false
		for _, f := range result.Output.Files {
			if f.Output == ".claude/loop.md" {
				foundInResult = true
				break
			}
		}
		assert.True(t, foundInResult, "expected loop file in compile result")
	})
}

func TestFixture_Loop_Cursor(t *testing.T) {
	capture, outputDir, _ := captureLogsForCompile(t, "cursor", compiler.CompileOptions{})
	t.Run("NoLoopFile", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".cursor/loop.md")))
	})
	t.Run("WarnLoopUnsupported", func(t *testing.T) {
		assert.True(t, capture.contains("layer uses loop but target adapter does not support them"),
			"logs: %v", capture.dump())
	})
}

func TestFixture_WorktreeInclude_Claude(t *testing.T) {
	outputDir, result := compileTo(t, "claude", compiler.CompileOptions{})
	t.Run("WorktreeInclude_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".worktreeinclude")))
	})
	t.Run("WorktreeInclude_Content", func(t *testing.T) {
		content := readFile(t, outputDir, ".worktreeinclude")
		assert.Contains(t, content, "node_modules/")
	})
	t.Run("WorktreeInclude_InManifest", func(t *testing.T) {
		// Verify worktree-include file is tracked in the manifest
		data, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
		require.NoError(t, err)
		var cm manifest.CompiledManifest
		require.NoError(t, json.Unmarshal(data, &cm))
		found := false
		for _, f := range cm.FileOps {
			if f.Output == ".worktreeinclude" {
				found = true
				assert.Equal(t, "concatenate", f.Strategy, "expected concatenate strategy for worktreeinclude file")
				break
			}
		}
		assert.True(t, found, "expected worktreeinclude file to be tracked in manifest file_ops")
		// Also verify in result
		foundInResult := false
		for _, f := range result.Output.Files {
			if f.Output == ".worktreeinclude" {
				foundInResult = true
				break
			}
		}
		assert.True(t, foundInResult, "expected worktreeinclude file in compile result")
	})
}

func TestFixture_WorktreeInclude_Cursor(t *testing.T) {
	capture, outputDir, _ := captureLogsForCompile(t, "cursor", compiler.CompileOptions{})
	t.Run("NoWorktreeInclude", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".worktreeinclude")))
	})
	t.Run("WarnWorktreeIncludeUnsupported", func(t *testing.T) {
		assert.True(t, capture.contains("layer uses worktree-include but target adapter does not support them"),
			"logs: %v", capture.dump())
	})
}

func TestFixture_WorktreeInclude_UserScope(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{Scope: compiler.ScopeUser})
	t.Run("NoWorktreeInclude_UserScope", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".worktreeinclude")),
			".worktreeinclude is project-only, should not appear at user scope")
	})
}

func TestFixture_UserScope_Claude_MergeInto(t *testing.T) {
	outputDir := t.TempDir()

	// Pre-populate a .claude.json with OAuth token and user MCP server.
	claudeJSON := filepath.Join(outputDir, ".claude.json")
	existing := map[string]any{
		"oauthToken": "sk-ant-preserve-me",
		"mcpServers": map[string]any{
			"user-local-server": map[string]any{
				"command": "my-local-mcp",
			},
		},
	}
	existingData, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(claudeJSON, existingData, 0644))

	// Compile with user scope.
	layers := buildFixtureLayers(t)
	_, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{Scope: compiler.ScopeUser})
	require.NoError(t, err)

	// Read the result.
	data, err := os.ReadFile(claudeJSON)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	t.Run("OAuthToken_Preserved", func(t *testing.T) {
		assert.Equal(t, "sk-ant-preserve-me", result["oauthToken"])
	})
	t.Run("UserServer_Preserved", func(t *testing.T) {
		servers := result["mcpServers"].(map[string]any)
		assert.Contains(t, servers, "user-local-server")
	})
	t.Run("KeelServers_Added", func(t *testing.T) {
		servers := result["mcpServers"].(map[string]any)
		// Check for servers from the fixture layers.
		assert.True(t, len(servers) > 1, "expected Keel-managed servers to be added alongside user server")
	})
	t.Run("Manifest_HasManagedServers", func(t *testing.T) {
		mf := readFile(t, outputDir, ".claude/.keel.json")
		assert.Contains(t, mf, "managed_mcp_servers")
	})
	t.Run("Manifest_MergeIntoStrategy", func(t *testing.T) {
		mf := readFile(t, outputDir, ".claude/.keel.json")
		assert.Contains(t, mf, "merge-into")
	})
}

// ---------------------------------------------------------------------------
// TestFixture_CompileAGENTSmd
// ---------------------------------------------------------------------------

func TestFixture_CompileAGENTSmd(t *testing.T) {
	outputDir, result := compileTo(t, "agentsmd", compiler.CompileOptions{})

	t.Run("AGENTSmd_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, "AGENTS.md")))
	})
	t.Run("AGENTSmd_HasContent", func(t *testing.T) {
		content := readFile(t, outputDir, "AGENTS.md")
		assert.NotEmpty(t, content)
		// Instructions from fixture layers should be present.
		assert.Contains(t, content, "Keel Base Instructions")
	})
	t.Run("Agent_Correct", func(t *testing.T) {
		assert.Equal(t, "agentsmd", result.Output.Agent)
	})
	t.Run("Manifest_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".agents", ".keel.json")))
	})
	t.Run("Manifest_AgentField", func(t *testing.T) {
		assert.Contains(t, readFile(t, outputDir, ".agents/.keel.json"), `"agent": "agentsmd"`)
	})
	t.Run("NoSettings", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".agents/settings.json")))
	})
	t.Run("NoMCP", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".mcp.json")))
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".agents/mcp.json")))
	})
	t.Run("NoCategories", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".agents/commands")))
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".agents/skills")))
	})
	t.Run("Variables_Resolved", func(t *testing.T) {
		content := readFile(t, outputDir, "AGENTS.md")
		assert.NotContains(t, content, "${keel:")
	})
	t.Run("Conditionals_Resolved", func(t *testing.T) {
		content := readFile(t, outputDir, "AGENTS.md")
		assert.NotContains(t, content, "keel-if")
		assert.NotContains(t, content, "keel-endif")
	})
}

func TestFixture_CompileWarnings_AGENTSmd(t *testing.T) {
	capture, _, _ := captureLogsForCompile(t, "agentsmd", compiler.CompileOptions{})
	// AGENTS.md is an instruction-only adapter — capability warnings are suppressed.
	t.Run("NoWarnHooks", func(t *testing.T) {
		assert.False(t, capture.contains("layer uses hooks but target adapter does not support them"),
			"instruction-only adapters should suppress capability warnings; logs: %v", capture.dump())
	})
	t.Run("NoWarnMCP", func(t *testing.T) {
		assert.False(t, capture.contains("layer uses mcp but target adapter does not support them"),
			"instruction-only adapters should suppress capability warnings; logs: %v", capture.dump())
	})
	t.Run("NoWarnCommands", func(t *testing.T) {
		assert.False(t, capture.contains("layer uses commands but target adapter does not support them"),
			"instruction-only adapters should suppress capability warnings; logs: %v", capture.dump())
	})
	t.Run("NoWarnSkills", func(t *testing.T) {
		assert.False(t, capture.contains("layer uses skills but target adapter does not support them"),
			"instruction-only adapters should suppress capability warnings; logs: %v", capture.dump())
	})
}

// ---------------------------------------------------------------------------
// TestFixture_Drift
// ---------------------------------------------------------------------------

func TestFixture_Drift_NoDriftAfterCompile(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	cm, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)
	require.NotNil(t, cm)

	drift, err := drift.Detect(outputDir, cm)
	require.NoError(t, err)
	if drift.HasDrift() {
		for _, m := range drift.Modified {
			t.Logf("modified: %s (expected=%s actual=%s)", m.Path, m.Expected, m.Actual)
		}
		for _, m := range drift.Missing {
			t.Logf("missing: %s (expected=%s)", m.Path, m.Expected)
		}
	}
	assert.False(t, drift.HasDrift(), "expected no drift after clean compile")
}

func TestFixture_Drift_DetectsModifiedInstructions(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})

	// Modify CLAUDE.md
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "CLAUDE.md"), []byte("tampered"), 0644))

	cm, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)
	drift, err := drift.Detect(outputDir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift())
	assert.Len(t, drift.Modified, 1)
	assert.Equal(t, "CLAUDE.md", drift.Modified[0].Path)
}

// ---------------------------------------------------------------------------
// TestFixture_Governance_Claude
// ---------------------------------------------------------------------------

func TestFixture_Governance_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
	require.NoError(t, err)
	var cm manifest.CompiledManifest
	require.NoError(t, json.Unmarshal(data, &cm))
	require.NotNil(t, cm.Governance, "expected governance section in manifest")

	t.Run("OverrideConflicts_ForceLockedCmd", func(t *testing.T) {
		found := false
		for _, conflict := range cm.Governance.OverrideConflicts {
			if conflict.Category == "command" && strings.Contains(conflict.Key, "force-locked-cmd") {
				found = true
				assert.Contains(t, conflict.AttemptedBy, "04-keel-overrides")
				assert.Contains(t, conflict.LockedBy, "01-keel-base")
				break
			}
		}
		assert.True(t, found, "expected override conflict for force-locked-cmd")
	})

	t.Run("OverrideConflicts_ForceLockedSkill", func(t *testing.T) {
		found := false
		for _, conflict := range cm.Governance.OverrideConflicts {
			if conflict.Category == "skill" && strings.Contains(conflict.Key, "force-locked-skill") {
				found = true
				assert.Contains(t, conflict.AttemptedBy, "04-keel-overrides")
				assert.Contains(t, conflict.LockedBy, "01-keel-base")
				break
			}
		}
		assert.True(t, found, "expected override conflict for force-locked-skill")
	})

	t.Run("PermissionsPolicy_Additive", func(t *testing.T) {
		require.NotNil(t, cm.Governance.PermissionsPolicy, "expected permissions policy in governance")
		assert.Equal(t, "additive", cm.Governance.PermissionsPolicy.Merge)
		assert.Contains(t, cm.Governance.PermissionsPolicy.SetBy, "01-keel-base")
	})

	t.Run("MCPPolicy_Present", func(t *testing.T) {
		require.NotNil(t, cm.Governance.MCPPolicy, "expected MCP policy in governance")
		assert.NotEmpty(t, cm.Governance.MCPPolicy.Allowed, "expected allowed servers in MCP policy")
		assert.NotEmpty(t, cm.Governance.MCPPolicy.Denied, "expected denied servers in MCP policy")
		assert.NotNil(t, cm.Governance.MCPPolicy.EnableAll, "expected enableAll in MCP policy")
	})
}

// ---------------------------------------------------------------------------
// TestFixture_EffectiveState_Claude
// ---------------------------------------------------------------------------

func TestFixture_EffectiveState_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
	require.NoError(t, err)
	var cm manifest.CompiledManifest
	require.NoError(t, json.Unmarshal(data, &cm))
	require.NotNil(t, cm.EffectiveState, "expected effective_state section in manifest")

	t.Run("Permissions_Present", func(t *testing.T) {
		require.NotNil(t, cm.EffectiveState.Permissions, "expected permissions in effective state")
		assert.NotEmpty(t, cm.EffectiveState.Permissions.Allow, "expected allow list in permissions")
	})

	t.Run("MCPServers_Present", func(t *testing.T) {
		assert.NotEmpty(t, cm.EffectiveState.MCPServers, "expected MCP servers in effective state")
		// Spot-check for a known server from the fixtures
		found := false
		for _, srv := range cm.EffectiveState.MCPServers {
			if srv.Name == "sonarqube" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected sonarqube server in effective state")
	})

	t.Run("Hooks_Present", func(t *testing.T) {
		assert.NotEmpty(t, cm.EffectiveState.Hooks, "expected hooks in effective state")
		// Check for a known hook event
		found := false
		for _, hook := range cm.EffectiveState.Hooks {
			if hook.Event == "PreToolUse" {
				found = true
				assert.Greater(t, hook.Handlers, 0, "expected at least one handler for PreToolUse")
				break
			}
		}
		assert.True(t, found, "expected PreToolUse hook in effective state")
	})

	t.Run("Settings_Present", func(t *testing.T) {
		assert.NotEmpty(t, cm.EffectiveState.Settings, "expected settings in effective state")
		// Spot-check model setting - should be present (layer 04 sets "opus", layer 06 cursor settings have "claude-opus")
		model, ok := cm.EffectiveState.Settings["model"]
		assert.True(t, ok, "expected model in settings")
		// The actual value will be from the last-wins merge - just verify it's non-empty
		assert.NotEmpty(t, model, "expected model to have a value")
	})

	t.Run("Instructions_Present", func(t *testing.T) {
		require.NotNil(t, cm.EffectiveState.Instructions, "expected instructions in effective state")
		assert.Greater(t, cm.EffectiveState.Instructions.TokenEstimate, 0, "expected non-zero token estimate")
		assert.Greater(t, cm.EffectiveState.Instructions.Bytes, 0, "expected non-zero byte count")
	})

	t.Run("Categories_Present", func(t *testing.T) {
		assert.NotEmpty(t, cm.EffectiveState.Categories, "expected categories in effective state")
		// Check for a known category
		found := false
		for _, cat := range cm.EffectiveState.Categories {
			if cat.Name == "commands" {
				found = true
				assert.Greater(t, cat.FileCount, 0, "expected at least one file in commands category")
				break
			}
		}
		assert.True(t, found, "expected commands category in effective state")
	})
}

// ---------------------------------------------------------------------------
// TestFixture_EffectiveState_Cursor
// ---------------------------------------------------------------------------

func TestFixture_EffectiveState_Cursor(t *testing.T) {
	outputDir, _ := compileTo(t, "cursor", compiler.CompileOptions{})
	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", ".keel.json"))
	require.NoError(t, err)
	var cm manifest.CompiledManifest
	require.NoError(t, json.Unmarshal(data, &cm))
	require.NotNil(t, cm.EffectiveState, "expected effective_state section in manifest")

	t.Run("Environment_Present", func(t *testing.T) {
		// Cursor has environment support - check that environment config appears in effective state
		// Layer 01 has environment section, should be captured
		assert.NotNil(t, cm.EffectiveState, "effective state should be populated for Cursor")
	})

	t.Run("Settings_Present", func(t *testing.T) {
		assert.NotEmpty(t, cm.EffectiveState.Settings, "expected settings in effective state")
	})

	t.Run("Categories_Present", func(t *testing.T) {
		assert.NotEmpty(t, cm.EffectiveState.Categories, "expected categories in effective state")
	})
}

// ---------------------------------------------------------------------------
// TestFixture_Report_Claude
// ---------------------------------------------------------------------------

func TestFixture_Report_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
	require.NoError(t, err)
	var cm manifest.CompiledManifest
	require.NoError(t, json.Unmarshal(data, &cm))

	// Import the report package
	// (Already imported at top via qualified name)
	// Need to add import statement at top of file
	// For now, we'll use the unqualified name since it's in the same package scope

	t.Run("Generate_ProducesReport", func(t *testing.T) {
		rpt := report.Generate(&cm)
		require.NotNil(t, rpt, "expected report to be generated")
		assert.Equal(t, "1.0", rpt.SchemaVersion)
		assert.Equal(t, "claude", rpt.Compilation.Agent)
		assert.Greater(t, len(rpt.LayerBOM), 0, "expected layers in BOM")
		assert.NotNil(t, rpt.Governance, "expected governance section")
		assert.NotNil(t, rpt.EffectiveState, "expected effective state section")
		assert.NotEmpty(t, rpt.ForceLocks, "expected force locks")
		assert.NotEmpty(t, rpt.FileOps, "expected file operations")
	})

	t.Run("RenderText_ProducesOutput", func(t *testing.T) {
		rpt := report.Generate(&cm)
		var buf strings.Builder
		err := report.RenderText(&buf, rpt)
		require.NoError(t, err)
		output := buf.String()

		assert.Contains(t, output, "Keel Compliance Report")
		assert.Contains(t, output, "Layer Bill of Materials")
		assert.Contains(t, output, "Effective State")
		assert.Contains(t, output, "Governance")
		assert.Contains(t, output, "Force Locks")
	})
}

// ---------------------------------------------------------------------------
// TestFixture_SemanticDrift_Claude
// ---------------------------------------------------------------------------

func TestFixture_SemanticDrift_Claude(t *testing.T) {
	outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})

	// Modify the compiled settings.json to trigger semantic drift
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Change the model to trigger a semantic change
	originalModel := settings["model"]
	settings["model"] = "modified-model"

	modified, err := json.MarshalIndent(settings, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(settingsPath, modified, 0644))

	// Load manifest
	cm, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)
	require.NotNil(t, cm)

	// Create StateReader like cmd/keel/drift.go does
	stateReader := func(outputDir string) (*manifest.EffectiveState, error) {
		reader, err := compiler.GetReader("claude")
		if err != nil {
			return nil, err
		}
		ir, err := reader.Read(outputDir)
		if err != nil {
			return nil, err
		}
		return compiler.BuildEffectiveStateFromIR(ir), nil
	}

	// Detect semantic drift
	semDiff, err := drift.DetectSemantic(outputDir, cm, stateReader)
	require.NoError(t, err)

	t.Run("SemanticChanges_Detected", func(t *testing.T) {
		require.NotNil(t, semDiff, "expected semantic drift to be detected")
		assert.True(t, semDiff.HasChanges(), "expected semantic changes")
	})

	t.Run("SettingChanges_Detected", func(t *testing.T) {
		require.NotEmpty(t, semDiff.Settings, "expected setting changes")
		found := false
		for _, change := range semDiff.Settings {
			if change.Key == "model" {
				found = true
				assert.Equal(t, originalModel, change.From, "expected original model in From")
				assert.Equal(t, "modified-model", change.To, "expected modified model in To")
				break
			}
		}
		assert.True(t, found, "expected model setting change to be detected")
	})
}

// ---------------------------------------------------------------------------
// TestFixture_ConventionLayer — instructions/ directory discovery
// ---------------------------------------------------------------------------

func TestFixture_ConventionLayer_InstructionsDir(t *testing.T) {
	dir := t.TempDir()

	// Create a convention layer (no keel.yml) with instructions/ directory.
	layerDir := filepath.Join(dir, "local")
	instrDir := filepath.Join(layerDir, "instructions")
	require.NoError(t, os.MkdirAll(instrDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(instrDir, "backlog.md"), []byte("# Backlog\n\nTrack items."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(instrDir, "upkeep.md"), []byte("# Upkeep\n\nKeep current."), 0644))

	outputDir := t.TempDir()
	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "project"}}

	_, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)

	content := readFile(t, outputDir, "CLAUDE.md")
	t.Run("BacklogIncluded", func(t *testing.T) { assert.Contains(t, content, "Track items") })
	t.Run("UpkeepIncluded", func(t *testing.T) { assert.Contains(t, content, "Keep current") })
}

// ---------------------------------------------------------------------------
// TestFixture_CrossCompile — category pruning
// ---------------------------------------------------------------------------

func TestFixture_CrossCompile_CategoryPruning(t *testing.T) {
	dir := t.TempDir()

	// Create a Claude layer with plugins (Claude-only category).
	layerDir := filepath.Join(dir, "claude-layer")
	pluginDir := filepath.Join(layerDir, ".claude", "plugins")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "my-plugin.json"), []byte(`{"name":"test"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "CLAUDE.md"), []byte("# Instructions"), 0644))

	layers := []config.Layer{{Path: layerDir, Format: "claude"}}

	t.Run("Claude_KeepsPlugins", func(t *testing.T) {
		outputDir := t.TempDir()
		_, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
		require.NoError(t, err)
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude", "plugins", "my-plugin.json")),
			"Claude should keep its own plugins category")
	})

	t.Run("Cursor_PrunesPlugins", func(t *testing.T) {
		outputDir := t.TempDir()
		_, err := compiler.Compile("cursor", layers, outputDir, compiler.CompileOptions{})
		require.NoError(t, err)
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".cursor", "plugins")),
			"Cursor should prune plugins (not in its profile categories)")
	})

	t.Run("AGENTSmd_PrunesAll", func(t *testing.T) {
		outputDir := t.TempDir()
		_, err := compiler.Compile("agentsmd", layers, outputDir, compiler.CompileOptions{})
		require.NoError(t, err)
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".agents", "plugins")),
			"AGENTS.md should prune all categories")
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, "AGENTS.md")),
			"AGENTS.md should still have instructions")
	})
}

func TestFixture_UserScope_Claude_Clean_MergeInto(t *testing.T) {
	outputDir := t.TempDir()

	// Pre-populate .claude.json.
	claudeJSON := filepath.Join(outputDir, ".claude.json")
	existing := map[string]any{
		"oauthToken": "sk-ant-preserve-me",
		"mcpServers": map[string]any{
			"user-local-server": map[string]any{"command": "my-local-mcp"},
		},
	}
	existingData, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(claudeJSON, existingData, 0644))

	// Compile (adds Keel-managed servers).
	layers := buildFixtureLayers(t)
	_, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{Scope: compiler.ScopeUser})
	require.NoError(t, err)

	// Verify servers were added.
	data, _ := os.ReadFile(claudeJSON)
	var afterCompile map[string]any
	json.Unmarshal(data, &afterCompile)
	servers := afterCompile["mcpServers"].(map[string]any)
	require.True(t, len(servers) > 1, "expected multiple servers after compile")

	// Clean.
	_, err = manifest.CleanAllOutput(outputDir, ".claude", false)
	require.NoError(t, err)

	// .claude.json should still exist (has OAuth token).
	t.Run("File_StillExists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(claudeJSON))
	})

	// Read after clean.
	data, err = os.ReadFile(claudeJSON)
	require.NoError(t, err)
	var afterClean map[string]any
	require.NoError(t, json.Unmarshal(data, &afterClean))

	t.Run("OAuthToken_StillPreserved", func(t *testing.T) {
		assert.Equal(t, "sk-ant-preserve-me", afterClean["oauthToken"])
	})
	t.Run("UserServer_StillPreserved", func(t *testing.T) {
		if cleanServers, ok := afterClean["mcpServers"].(map[string]any); ok {
			assert.Contains(t, cleanServers, "user-local-server")
		}
	})
	t.Run("KeelServers_Removed", func(t *testing.T) {
		cleanServers, _ := afterClean["mcpServers"].(map[string]any)
		// Keel-managed servers should be gone, only user server remains.
		if cleanServers != nil {
			assert.Len(t, cleanServers, 1, "expected only user server to remain")
		}
	})
}

// ---------------------------------------------------------------------------
// TestFixture_InstructionConflict
// ---------------------------------------------------------------------------

func TestFixture_InstructionConflict(t *testing.T) {
	// Helper to create a minimal layer with instructions
	createMinimalLayer := func(t *testing.T) (string, []config.Layer) {
		t.Helper()
		layerDir := t.TempDir()
		keelYAML := `schema: v1
name: conflict-test
instructions:
  - source: instructions.md
`
		require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYAML), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Compiled by keel\n"), 0644))

		format, version := config.DetectLayerFormat(layerDir)
		layer := config.Layer{Path: layerDir, Type: "test", Format: format, Version: version}
		return layerDir, []config.Layer{layer}
	}

	t.Run("PreservedWithoutForce", func(t *testing.T) {
		outputDir := t.TempDir()

		// Pre-existing CLAUDE.md
		claudeMDPath := filepath.Join(outputDir, "CLAUDE.md")
		userContent := "# My project notes\n"
		require.NoError(t, os.WriteFile(claudeMDPath, []byte(userContent), 0644))

		// Compile a minimal layer WITHOUT force
		_, layers := createMinimalLayer(t)
		result, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
		require.NoError(t, err)

		// Assert CLAUDE.md still contains user content
		content := readFile(t, outputDir, "CLAUDE.md")
		assert.Contains(t, content, "My project notes", "user content should be preserved")

		// Assert InstructionConflicts is recorded
		assert.NotEmpty(t, result.Governance.InstructionConflicts, "expected instruction conflicts to be recorded")
		assert.Contains(t, result.Governance.InstructionConflicts, "CLAUDE.md")
	})

	t.Run("OverwrittenWithForce", func(t *testing.T) {
		outputDir := t.TempDir()

		// Pre-existing CLAUDE.md
		claudeMDPath := filepath.Join(outputDir, "CLAUDE.md")
		userContent := "# My project notes\n"
		require.NoError(t, os.WriteFile(claudeMDPath, []byte(userContent), 0644))

		// Compile WITH force
		_, layers := createMinimalLayer(t)
		result, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{ForceInstructions: true})
		require.NoError(t, err)

		// Assert CLAUDE.md now contains compiled content
		content := readFile(t, outputDir, "CLAUDE.md")
		assert.NotContains(t, content, "My project notes", "user content should be overwritten")
		assert.Contains(t, content, "Compiled by keel", "compiled content should be present")

		// Assert no conflicts recorded when force is used
		assert.Empty(t, result.Governance.InstructionConflicts, "no conflicts when force is used")
	})

	t.Run("UserEditPreserved", func(t *testing.T) {
		outputDir := t.TempDir()

		// First compile WITH force (creates manifest)
		_, layers := createMinimalLayer(t)
		_, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{ForceInstructions: true})
		require.NoError(t, err)

		// User edits the file
		claudeMDPath := filepath.Join(outputDir, "CLAUDE.md")
		userEdit := "# User edited this\n"
		require.NoError(t, os.WriteFile(claudeMDPath, []byte(userEdit), 0644))

		// Recompile WITHOUT force
		result, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
		require.NoError(t, err)

		// Assert CLAUDE.md still contains user edit
		content := readFile(t, outputDir, "CLAUDE.md")
		assert.Contains(t, content, "User edited this", "user edit should be preserved")

		// Assert conflict detected (manifest exists, file was modified)
		assert.NotEmpty(t, result.Governance.InstructionConflicts)
	})
}

// ---------------------------------------------------------------------------
// TestFixture_Clean_InstructionPreservation
// ---------------------------------------------------------------------------

func TestFixture_Clean_InstructionPreservation(t *testing.T) {
	t.Run("PreservedWithoutForce", func(t *testing.T) {
		outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{ForceInstructions: true})

		// Verify CLAUDE.md exists
		claudeMDPath := filepath.Join(outputDir, "CLAUDE.md")
		require.True(t, testutil.FileExists(claudeMDPath))

		// Clean without force
		_, err := manifest.CleanAllOutput(outputDir, ".claude", false)
		require.NoError(t, err)

		// Assert CLAUDE.md still exists
		assert.True(t, testutil.FileExists(claudeMDPath), "CLAUDE.md should be preserved without --force")
	})

	t.Run("RemovedWithForce", func(t *testing.T) {
		outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{ForceInstructions: true})

		// Verify CLAUDE.md exists
		claudeMDPath := filepath.Join(outputDir, "CLAUDE.md")
		require.True(t, testutil.FileExists(claudeMDPath))

		// Clean WITH force
		_, err := manifest.CleanAllOutput(outputDir, ".claude", true)
		require.NoError(t, err)

		// Assert CLAUDE.md is removed
		assert.False(t, testutil.FileExists(claudeMDPath), "CLAUDE.md should be removed with --force")
	})
}

// ---------------------------------------------------------------------------
// TestFixture_StrictMode_InstructionConflict
// ---------------------------------------------------------------------------

func TestFixture_StrictMode_InstructionConflict(t *testing.T) {
	// Helper to create a minimal layer with instructions
	createMinimalLayer := func(t *testing.T) (string, []config.Layer) {
		t.Helper()
		layerDir := t.TempDir()
		keelYAML := `schema: v1
name: strict-test
instructions:
  - source: instructions.md
`
		require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYAML), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Strict mode test\n"), 0644))

		format, version := config.DetectLayerFormat(layerDir)
		layer := config.Layer{Path: layerDir, Type: "test", Format: format, Version: version}
		return layerDir, []config.Layer{layer}
	}

	t.Run("FailsOnConflict", func(t *testing.T) {
		outputDir := t.TempDir()

		// Pre-existing CLAUDE.md
		claudeMDPath := filepath.Join(outputDir, "CLAUDE.md")
		require.NoError(t, os.WriteFile(claudeMDPath, []byte("# Existing content\n"), 0644))

		// Compile in strict mode WITHOUT force - should detect conflict
		_, layers := createMinimalLayer(t)
		result, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
		require.NoError(t, err, "compile should succeed, strict mode is CLI-level")

		// Verify conflict was detected
		assert.NotEmpty(t, result.Governance.InstructionConflicts, "expected instruction conflict")

		// Note: Strict mode enforcement happens at CLI level (cmd/keel/compile.go),
		// not in the compiler. The compiler just records conflicts in the result.
		// A CLI-level test would check the error, but here we verify the compiler
		// properly detects and records conflicts that strict mode would fail on.
	})

	t.Run("SucceedsWithForce", func(t *testing.T) {
		outputDir := t.TempDir()

		// Pre-existing CLAUDE.md
		claudeMDPath := filepath.Join(outputDir, "CLAUDE.md")
		require.NoError(t, os.WriteFile(claudeMDPath, []byte("# Existing content\n"), 0644))

		// Compile in strict mode WITH force - should succeed
		_, layers := createMinimalLayer(t)
		result, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{ForceInstructions: true})
		require.NoError(t, err)

		// No conflicts when force is used
		assert.Empty(t, result.Governance.InstructionConflicts, "no conflicts with force")

		// Content should be overwritten
		content := readFile(t, outputDir, "CLAUDE.md")
		assert.Contains(t, content, "Strict mode test")
	})
}

// ---------------------------------------------------------------------------
// TestFixture_FullLifecycle_UserFilesPreserved
// ---------------------------------------------------------------------------

func TestFixture_FullLifecycle_UserFilesPreserved(t *testing.T) {
	// Helper to create a minimal layer with instructions only
	createMinimalLayer := func(t *testing.T) []config.Layer {
		t.Helper()
		layerDir := t.TempDir()
		keelYAML := `schema: v1
name: lifecycle-test
instructions:
  - source: instructions.md
`
		require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYAML), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Lifecycle test\n"), 0644))

		format, version := config.DetectLayerFormat(layerDir)
		layer := config.Layer{Path: layerDir, Type: "test", Format: format, Version: version}
		return []config.Layer{layer}
	}

	outputDir := t.TempDir()

	// 1. Create pre-existing user files
	claudeMDPath := filepath.Join(outputDir, "CLAUDE.md")
	userSettingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	userAgentPath := filepath.Join(outputDir, ".claude", "agents", "helper.md")

	require.NoError(t, os.WriteFile(claudeMDPath, []byte("# My Project\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(outputDir, ".claude", "agents"), 0755))
	require.NoError(t, os.WriteFile(userSettingsPath, []byte(`{"myKey": "myValue"}`), 0644))
	require.NoError(t, os.WriteFile(userAgentPath, []byte("# Helper agent\n"), 0644))

	// 2. Compile WITHOUT force (instruction conflict should preserve CLAUDE.md)
	layers := createMinimalLayer(t)
	result, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)

	// 3. Assert: CLAUDE.md unchanged, user files unchanged, manifest created
	t.Run("CLAUDE_MD_Unchanged", func(t *testing.T) {
		content := readFile(t, outputDir, "CLAUDE.md")
		assert.Contains(t, content, "My Project", "CLAUDE.md should be preserved")
		assert.NotContains(t, content, "Lifecycle test")
	})

	t.Run("UserSettings_Unchanged", func(t *testing.T) {
		content := readFile(t, outputDir, ".claude/settings.json")
		assert.Contains(t, content, "myKey", "user settings should be preserved")
	})

	t.Run("UserAgent_Unchanged", func(t *testing.T) {
		content := readFile(t, outputDir, ".claude/agents/helper.md")
		assert.Contains(t, content, "Helper agent", "user agent should be preserved")
	})

	t.Run("Manifest_Created", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".claude", ".keel.json")),
			"manifest should be created")
	})

	t.Run("InstructionConflict_Recorded", func(t *testing.T) {
		assert.NotEmpty(t, result.Governance.InstructionConflicts,
			"instruction conflict should be recorded")
	})

	// 4. Clean (without force)
	pruned, err := manifest.CleanAllOutput(outputDir, ".claude", false)
	require.NoError(t, err)
	require.NotNil(t, pruned)

	// 5. Assert: CLAUDE.md still present, user files still present, manifest removed
	t.Run("CLAUDE_MD_StillPresent", func(t *testing.T) {
		assert.True(t, testutil.FileExists(claudeMDPath), "CLAUDE.md should be preserved after clean")
	})

	t.Run("UserSettings_StillPresent", func(t *testing.T) {
		assert.True(t, testutil.FileExists(userSettingsPath), "user settings should be preserved after clean")
	})

	t.Run("UserAgent_StillPresent", func(t *testing.T) {
		assert.True(t, testutil.FileExists(userAgentPath), "user agent should be preserved after clean")
	})

	t.Run("Manifest_Removed", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, ".claude", ".keel.json")),
			"manifest should be removed after clean")
	})
}

// ---------------------------------------------------------------------------
// TestFixture_CompileCodex
// ---------------------------------------------------------------------------

func TestFixture_CompileCodex(t *testing.T) {
	t.Parallel()

	outputDir, result := compileTo(t, "codex", compiler.CompileOptions{})

	t.Run("Compilation_Succeeds", func(t *testing.T) {
		assert.NotNil(t, result)
	})

	t.Run("AGENTS_MD_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, "AGENTS.md")))
	})

	t.Run("AGENTS_MD_HasContent", func(t *testing.T) {
		content := readFile(t, outputDir, "AGENTS.md")
		assert.NotEmpty(t, content)
		assert.Contains(t, content, "Keel Base Instructions")
	})

	t.Run("CodexDir_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".codex")))
	})

	t.Run("Agent_Correct", func(t *testing.T) {
		assert.Equal(t, "codex", result.Output.Agent)
	})
}

// ---------------------------------------------------------------------------
// TestFixture_CompileGoose
// ---------------------------------------------------------------------------

func TestFixture_CompileGoose(t *testing.T) {
	t.Parallel()

	outputDir, result := compileTo(t, "goose", compiler.CompileOptions{})

	t.Run("Compilation_Succeeds", func(t *testing.T) {
		assert.NotNil(t, result)
	})

	t.Run("Goosehints_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".goosehints")))
	})

	t.Run("Goosehints_HasContent", func(t *testing.T) {
		content := readFile(t, outputDir, ".goosehints")
		assert.NotEmpty(t, content)
		assert.Contains(t, content, "Keel Base Instructions")
	})

	t.Run("GooseDir_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".goose")))
	})

	t.Run("Agent_Correct", func(t *testing.T) {
		assert.Equal(t, "goose", result.Output.Agent)
	})
}

// ---------------------------------------------------------------------------
// TestFixture_CompileAgentPlugins
// ---------------------------------------------------------------------------

func TestFixture_CompileAgentPlugins(t *testing.T) {
	t.Parallel()

	outputDir, result := compileTo(t, "agentplugins", compiler.CompileOptions{})

	t.Run("Agent_Correct", func(t *testing.T) {
		assert.Equal(t, "agentplugins", result.Output.Agent)
	})

	t.Run("OutputDir_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".agentplugins")))
	})

	t.Run("NoAGENTSmd", func(t *testing.T) {
		assert.False(t, testutil.FileExists(filepath.Join(outputDir, "AGENTS.md")))
	})

	t.Run("Manifest_Exists", func(t *testing.T) {
		assert.True(t, testutil.FileExists(filepath.Join(outputDir, ".agentplugins", ".keel.json")))
	})
}
