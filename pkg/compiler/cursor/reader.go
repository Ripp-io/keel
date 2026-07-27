package cursor

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"unicode"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

func init() {
	compiler.RegisterReader("cursor", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Cursor native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "cursor" }

// Read reads a Cursor project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "cursor",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}
	if err := r.readMCP(layerPath, ir); err != nil {
		return nil, err
	}
	if err := r.readHooks(layerPath, ir); err != nil {
		return nil, err
	}
	if err := r.readEnvironment(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills", "rules"}
	if err := compiler.ReadCategories(layerPath, ".cursor", categories, ir); err != nil {
		return nil, err
	}

	// Sandbox (separate file in Cursor)
	r.readSandbox(layerPath, ir)

	// Load snippets from the native layer.
	compiler.ReadSnippets(layerPath, ".cursor", ir)

	// Load bin files from native layer.
	compiler.ReadBinDir(layerPath, ir)

	// Read arbitrary "other" files from the dot-directory.
	r.readOtherDotDirFiles(layerPath, ir)

	// Read keel.yml sidecar for file policies and metadata.
	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking multiple locations
// and candidate names. Decomposes into permissions, portable settings, and
// Cursor-specific settings.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	// Derive candidate paths from the profile's SettingsCandidates to keep
	// reader and profile in sync.
	candidates := compiler.SettingsReadCandidates(layerPath, ".cursor", ProjectProfile().SettingsCandidates)
	raw, err := compiler.ReadJSONConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read cli-config.json", "keys", len(raw), "layer", layerPath)

	// Extract permissions.allow
	if allowRaw, ok := raw["permissions.allow"]; ok {
		r.extractPermissions(ir, "allow", allowRaw)
		delete(raw, "permissions.allow")
	}

	// Extract permissions.deny
	if denyRaw, ok := raw["permissions.deny"]; ok {
		r.extractPermissions(ir, "deny", denyRaw)
		delete(raw, "permissions.deny")
	}

	// Extract portable model setting
	if model, ok := raw["model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = model
		delete(raw, "model")
	}

	// Discard version (Cursor artifact, not portable)
	delete(raw, "version")

	// All remaining keys are Cursor-specific
	for key, val := range raw {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["cursor"] == nil {
			ir.AgentSpecific["cursor"] = make(map[string]any)
		}
		ir.AgentSpecific["cursor"][key] = val
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "cursor")

	return nil
}

// extractPermissions converts a permission list from Cursor format to
// canonical form and stores it in ir.Permissions under the given key.
func (r *Reader) extractPermissions(ir *compiler.LayerIR, key string, raw any) {
	slice, ok := raw.([]any)
	if !ok {
		return
	}

	stripped := make([]string, 0, len(slice))
	for _, item := range slice {
		s, ok := item.(string)
		if !ok {
			continue
		}
		stripped = append(stripped, stripCursorPermissionSuffix(s))
	}

	// Convert []string to []any for consistent cross-agent merge semantics
	anySlice := make([]any, len(stripped))
	for i, s := range stripped {
		anySlice[i] = s
	}

	if ir.Permissions == nil {
		ir.Permissions = make(map[string]any)
	}
	ir.Permissions[key] = anySlice
}

// stripCursorPermissionSuffix converts Cursor permission format to canonical form.
// "Read(*)" -> "Read", but "Bash(git:*)" stays unchanged (it's parameterized, not a bare suffix).
func stripCursorPermissionSuffix(perm string) string {
	if strings.HasSuffix(perm, "(*)") {
		return strings.TrimSuffix(perm, "(*)")
	}
	return perm
}

// readMCP reads mcp.json from the layer root, falling back to .cursor/mcp.json
// for real Cursor project directories.
func (r *Reader) readMCP(layerPath string, ir *compiler.LayerIR) error {
	candidates := []string{
		filepath.Join(layerPath, "mcp.json"),
		filepath.Join(layerPath, ".cursor", "mcp.json"),
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

// readHooks reads hooks.json from the layer root, falling back to .cursor/hooks.json
// for real Cursor project directories. Keys are transformed from camelCase to
// PascalCase (the IR canonical form). _keel policies are stripped and converted
// to force entries on the hook event arrays.
func (r *Reader) readHooks(layerPath string, ir *compiler.LayerIR) error {
	path := filepath.Join(layerPath, "hooks.json")
	data, err := pathutil.SafeReadFile(path)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		path = filepath.Join(layerPath, ".cursor", "hooks.json")
		data, err = pathutil.SafeReadFile(path)
	}
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	slog.Debug("read hooks.json", "keys", len(raw), "layer", layerPath)

	// Extract _keel policies before stripping. The hooks _keel format uses
	// Cursor-native camelCase keys: {"_keel": {"preToolUse": {"force": true}}}.
	keelPolicies := compiler.ExtractTopLevelKeelPolicies(raw)

	// Strip _keel from the raw map.
	cleaned, _, err := merge.StripKeelPolicies(raw)
	if err != nil {
		slog.Warn("stripping _keel policies failed, using raw data", "error", err)
	} else {
		raw = cleaned
	}

	// Transform keys from Cursor-native to IR canonical (PascalCase).
	// Check the explicit rename table first, then fall back to camelToPascal.
	transformed := make(map[string]any, len(raw))
	for key, val := range raw {
		if renamed, ok := ToClaudeHookEvents[key]; ok {
			transformed[renamed] = val
		} else {
			transformed[camelToPascal(key)] = val
		}
	}

	// Inject force: true into hook entries for events that had _keel force policies.
	// This makes the accumulator's hasForceEntry check pick them up.
	for eventName, p := range keelPolicies {
		if p.Force == nil || !*p.Force {
			continue
		}
		// Transform the event name to canonical form
		canonicalName := eventName
		if renamed, ok := ToClaudeHookEvents[eventName]; ok {
			canonicalName = renamed
		} else {
			canonicalName = camelToPascal(eventName)
		}
		if entries, ok := transformed[canonicalName]; ok {
			if arr, ok := entries.([]any); ok && len(arr) > 0 {
				if first, ok := arr[0].(map[string]any); ok {
					first["force"] = true
				}
			}
		}
	}

	ir.Hooks = transformed
	return nil
}

// camelToPascal converts camelCase to PascalCase.
// preToolUse -> PreToolUse
func camelToPascal(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// readEnvironment reads .cursor/environment.json if it exists into ConfigBlocks.
func (r *Reader) readEnvironment(layerPath string, ir *compiler.LayerIR) error {
	path := filepath.Join(layerPath, ".cursor", "environment.json")
	data, err := pathutil.SafeReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	slog.Debug("read environment.json", "keys", len(raw), "layer", layerPath)
	if ir.ConfigBlocks == nil {
		ir.ConfigBlocks = make(map[string]map[string]any)
	}
	ir.ConfigBlocks["environment"] = raw
	return nil
}

// readSandbox reads sandbox.json from the Cursor native layer and normalizes
// it into the portable ir.Sandbox field.
func (r *Reader) readSandbox(layerPath string, ir *compiler.LayerIR) {
	candidates := []string{
		filepath.Join(layerPath, ".cursor", "sandbox.json"),
		filepath.Join(layerPath, "sandbox.json"),
	}
	for _, path := range candidates {
		data, err := pathutil.SafeReadFile(path)
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			slog.Debug("failed to parse sandbox.json", "path", path, "error", err)
			continue
		}
		ir.Sandbox = cursorSandboxToCanonical(raw)
		slog.Debug("read sandbox.json", "path", path, "layer", layerPath)
		return
	}
}

// cursorSandboxToCanonical maps Cursor sandbox.json format to a canonical
// *schema.SandboxConfig. Cursor-only fields (type, additionalReadwritePaths,
// etc.) are stored in their dedicated struct fields. Network policy is
// translated to the portable SandboxNetConfig.
func cursorSandboxToCanonical(raw map[string]any) *schema.SandboxConfig {
	sb := &schema.SandboxConfig{}

	if t, ok := raw["type"].(string); ok {
		sb.Type = t
	}
	if paths, ok := raw["additionalReadwritePaths"].([]any); ok {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				sb.AdditionalReadwritePaths = append(sb.AdditionalReadwritePaths, s)
			}
		}
	}
	if paths, ok := raw["additionalReadonlyPaths"].([]any); ok {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				sb.AdditionalReadonlyPaths = append(sb.AdditionalReadonlyPaths, s)
			}
		}
	}
	if v, ok := raw["disableTmpWrite"].(bool); ok {
		sb.DisableTmpWrite = &v
	}
	if v, ok := raw["enableSharedBuildCache"].(bool); ok {
		sb.EnableSharedBuildCache = &v
	}
	if np, ok := raw["networkPolicy"].(map[string]any); ok {
		sb.Network = &schema.SandboxNetConfig{}
		if d, ok := np["default"].(string); ok {
			sb.Network.DefaultPolicy = d
		}
		if allow, ok := np["allow"].([]any); ok {
			for _, a := range allow {
				if s, ok := a.(string); ok {
					sb.Network.AllowedDomains = append(sb.Network.AllowedDomains, s)
				}
			}
		}
		if deny, ok := np["deny"].([]any); ok {
			for _, d := range deny {
				if s, ok := d.(string); ok {
					sb.Network.DeniedDomains = append(sb.Network.DeniedDomains, s)
				}
			}
		}
	}

	return sb
}

// readOtherDotDirFiles reads arbitrary files from the .cursor/ directory
// that are not handled by specific readers.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"cli-config.json":      true,
		"cli.json":             true,
		"settings.cursor.json": true,
		"mcp.json":             true,
		"hooks.json":           true,
		"environment.json":     true,
		"sandbox.json":         true,
		".keel.json":           true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "rules", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".cursor", excludeFiles, knownCategories, ir)
}
