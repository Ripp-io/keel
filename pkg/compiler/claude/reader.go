package claude

import (
	"log/slog"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
)

func init() {
	compiler.RegisterReader("claude", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Claude Code native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "claude" }

// Read reads a Claude project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "claude",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}
	if err := r.readMCP(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills", "plugins", "agent-memory", "rules"}
	if err := compiler.ReadCategories(layerPath, ".claude", categories, ir); err != nil {
		return nil, err
	}

	// Load snippets from the native layer (snippets/ at root or .claude/snippets/).
	compiler.ReadSnippets(layerPath, ".claude", ir)

	// Load bin files from native layer.
	compiler.ReadBinDir(layerPath, ir)

	// Read arbitrary "other" files from the dot-directory that are not
	// settings, MCP, or categories — these are passed through unchanged.
	r.readOtherDotDirFiles(layerPath, ir)

	// Loop prompt
	r.readLoop(layerPath, ir)

	// Worktree include
	r.readWorktreeInclude(layerPath, ir)

	// Read keel.yml sidecar for file policies and metadata.
	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .claude/settings.json
// and layer-root settings.json (and the convention name settings.claude.json).
// The first found file wins. It decomposes the contents into permissions,
// hooks, portable settings, and Claude-specific settings.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	// Derive candidate paths from the profile's SettingsCandidates to keep
	// reader and profile in sync.
	candidates := compiler.SettingsReadCandidates(layerPath, ".claude", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read settings.json", "keys", len(raw), "layer", layerPath)

	// Extract permissions
	if perms, ok := raw["permissions"]; ok {
		if permMap, ok := perms.(map[string]any); ok {
			ir.Permissions = permMap
		}
		delete(raw, "permissions")
	}

	// Extract hooks (already PascalCase in Claude format)
	if hooks, ok := raw["hooks"]; ok {
		if hookMap, ok := hooks.(map[string]any); ok {
			ir.Hooks = hookMap
		}
		delete(raw, "hooks")
	}

	// Extract portable keys: model goes to ir.Settings
	// Everything else goes to AgentSpecific["claude"]
	if modelVal, ok := raw["model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = modelVal
		delete(raw, "model")
	}

	// Remaining keys are Claude-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["claude"] == nil {
			ir.AgentSpecific["claude"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["claude"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "claude")

	return nil
}

// readMCP reads MCP config, checking .mcp.json, mcp.json at layer root,
// and .claude/.mcp.json for Claude layers.
func (r *Reader) readMCP(layerPath string, ir *compiler.LayerIR) error {
	candidates := []string{
		filepath.Join(layerPath, ".mcp.json"),
		filepath.Join(layerPath, "mcp.json"),
	}
	raw, err := compiler.ReadJSONConfig(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read mcp.json", "keys", len(raw), "layer", layerPath)
	ir.MCP = raw
	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .claude/ directory
// that are not handled by specific readers (settings, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"settings.json":        true,
		"settings.claude.json": true,
		"settings.local.json":  true,
		".keel.json":           true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "plugins", "agent-memory", "rules", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".claude", excludeFiles, knownCategories, ir)
}

// readLoop reads .claude/loop.md from a native Claude layer into ContentBlocks.
func (r *Reader) readLoop(layerPath string, ir *compiler.LayerIR) {
	candidates := []string{
		filepath.Join(layerPath, ".claude", "loop.md"),
		filepath.Join(layerPath, "loop.md"),
	}
	for _, path := range candidates {
		data, err := pathutil.SafeReadFile(path)
		if err != nil {
			continue
		}
		if ir.ContentBlocks == nil {
			ir.ContentBlocks = make(map[string][]compiler.InstructionBlock)
		}
		ir.ContentBlocks["loop"] = []compiler.InstructionBlock{
			{Filename: "loop.md", Content: string(data)},
		}
		return
	}
}

// readWorktreeInclude reads .worktreeinclude from a native Claude layer into ContentBlocks.
func (r *Reader) readWorktreeInclude(layerPath string, ir *compiler.LayerIR) {
	path := filepath.Join(layerPath, ".worktreeinclude")
	data, err := pathutil.SafeReadFile(path)
	if err != nil {
		return
	}
	if ir.ContentBlocks == nil {
		ir.ContentBlocks = make(map[string][]compiler.InstructionBlock)
	}
	ir.ContentBlocks["worktree-include"] = []compiler.InstructionBlock{
		{Filename: ".worktreeinclude", Content: string(data)},
	}
}
