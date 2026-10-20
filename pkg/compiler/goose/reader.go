package goose

import (
	"log/slog"
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gopkg.in/yaml.v3"
)

func init() {
	compiler.RegisterReader("goose", func() compiler.NativeReader { return &Reader{} })
}

// Reader reads Goose native configuration into a LayerIR.
type Reader struct{}

func (r *Reader) AgentName() string { return "goose" }

// Read reads a Goose project directory and normalizes it into a LayerIR.
// All files are independently optional — missing files produce nil/empty
// fields, not errors. Only actual I/O failures on existing files cause errors.
func (r *Reader) Read(layerPath string) (*compiler.LayerIR, error) {
	ir := &compiler.LayerIR{
		SourceAgent: "goose",
		LayerPath:   layerPath,
	}

	compiler.ReadInstructions(layerPath, ProjectProfile().InstructionCandidates, ir)

	if err := r.readSettings(layerPath, ir); err != nil {
		return nil, err
	}

	categories := []string{"commands", "agents", "skills"}
	if err := compiler.ReadCategories(layerPath, ".goose", categories, ir); err != nil {
		return nil, err
	}

	compiler.ReadSnippets(layerPath, ".goose", ir)
	compiler.ReadBinDir(layerPath, ir)

	r.readOtherDotDirFiles(layerPath, ir)

	compiler.ReadKeelYmlSidecar(layerPath, ir)

	return ir, nil
}

// readSettings reads settings from the layer, checking both .goose/config.yaml
// and layer-root config.yaml (and the convention name settings.goose.yaml).
// The first found file wins. It decomposes the contents into MCP, portable settings,
// and Goose-specific settings.
func (r *Reader) readSettings(layerPath string, ir *compiler.LayerIR) error {
	candidates := settingsReadCandidates(layerPath, ".goose", ProjectProfile().SettingsCandidates)
	raw, err := readYAMLConfigStripped(candidates)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	slog.Debug("read goose config.yaml", "keys", len(raw), "layer", layerPath)

	// Extract MCP from extensions and translate to IR format
	if extRaw, ok := raw["extensions"]; ok {
		if extArray, ok := extRaw.([]any); ok {
			if translated := TranslateMCPFromGoose(extArray); len(translated) > 0 {
				ir.MCP = translated
			}
		}
		delete(raw, "extensions")
	}

	// Extract portable model setting
	if modelVal, ok := raw["model"]; ok {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = modelVal
		delete(raw, "model")
	}

	// Remaining keys are Goose-specific
	if len(raw) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific["goose"] == nil {
			ir.AgentSpecific["goose"] = make(map[string]any)
		}
		for key, val := range raw {
			ir.AgentSpecific["goose"][key] = val
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings into canonical IR fields for cross-agent transpilation.
	compiler.ExtractPortableConcepts(ir, "goose")

	return nil
}

// readOtherDotDirFiles reads arbitrary files from the .goose/ directory
// that are not handled by specific readers (settings, categories, snippets).
// These files are stored in categories under the special CategoryDotDir key,
// which the writer handles as a raw overlay.
func (r *Reader) readOtherDotDirFiles(layerPath string, ir *compiler.LayerIR) {
	excludeFiles := map[string]bool{
		"config.yaml":         true,
		"settings.goose.yaml": true,
		".keel.json":          true,
	}
	knownCategories := []string{
		"commands", "agents", "skills", "snippets",
	}
	compiler.ReadOtherDotDirFiles(layerPath, ".goose", excludeFiles, knownCategories, ir)
}

// settingsReadCandidates returns a list of file paths to check for settings,
// in priority order. Follows the same pattern as compiler.SettingsReadCandidates.
func settingsReadCandidates(layerPath, dotDir string, candidates []string) []string {
	var paths []string
	for _, filename := range candidates {
		// Check in dot directory first
		paths = append(paths, filepath.Join(layerPath, dotDir, filename))
		// Then in layer root
		paths = append(paths, filepath.Join(layerPath, filename))
	}
	return paths
}

// readYAMLConfigStripped reads the first existing file from candidates as YAML,
// strips KEEL_MANAGED comment, and returns the parsed map. Returns nil if no
// candidates exist.
func readYAMLConfigStripped(candidates []string) (map[string]any, error) {
	for _, path := range candidates {
		data, err := pathutil.SafeReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		var result map[string]any
		if err := yaml.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, nil
}

// TranslateMCPFromGoose converts Goose's extensions format to IR MCP format.
// Input:  [{"name": "name", "type": "local", "command": ["cmd","arg"], "environment": {"K":"V"}}]
// Output: {"mcpServers": {"name": {"command": "cmd", "args": ["arg"], "env": {"K":"V"}}}}
func TranslateMCPFromGoose(extensions []any) map[string]any {
	if extensions == nil || len(extensions) == 0 {
		return nil
	}

	servers := make(map[string]any)
	for _, extRaw := range extensions {
		ext, ok := extRaw.(map[string]any)
		if !ok {
			continue
		}

		// Only process MCP extensions (they have type "local" or "remote" and command/url)
		extType, _ := ext["type"].(string)
		if extType != "local" && extType != "remote" {
			continue
		}

		name, ok := ext["name"].(string)
		if !ok || name == "" {
			continue
		}

		irServer := make(map[string]any)

		if extType == "remote" {
			// Remote server: keep URL, don't process command array
			if url, ok := ext["url"].(string); ok {
				irServer["url"] = url
			}
		} else {
			// Local server: split command array into command + args
			if cmdRaw, ok := ext["command"]; ok {
				if cmdArray, ok := cmdRaw.([]any); ok && len(cmdArray) > 0 {
					// First element is command
					if cmd, ok := cmdArray[0].(string); ok {
						irServer["command"] = cmd
					}
					// Rest are args
					if len(cmdArray) > 1 {
						irServer["args"] = cmdArray[1:]
					}
				}
			}
		}

		// Rename environment → env
		if envRaw, ok := ext["environment"]; ok {
			if env, ok := envRaw.(map[string]any); ok {
				irServer["env"] = env
			}
		}

		servers[name] = irServer
	}

	if len(servers) == 0 {
		return nil
	}

	return map[string]any{"mcpServers": servers}
}
