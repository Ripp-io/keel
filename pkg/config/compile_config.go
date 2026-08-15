package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
	"gopkg.in/yaml.v3"
)

// CompileConfig is the single source of truth for all compile parameters.
// Struct tags drive YAML parsing, Cobra flag registration, usage text, and defaults.
// Adding a new compile parameter = add one struct field here.
type CompileConfig struct {
	Output        map[string]string `yaml:"output"            flag:"output"            usage:"Agent output targets as agent=path (e.g. claude=/keel)"`
	Layers        []string          `yaml:"-"                 flag:"layer"             usage:"Layer paths (repeatable, overrides CONFIG_LAYERS)"`
	LayerEntries  []LayerEntry      `yaml:"layers,omitempty"`
	Distributable bool              `yaml:"distributable"     flag:"distributable"     usage:"Produce a distributable layer"`
	Name          string            `yaml:"name"              flag:"name"              usage:"Project name (overrides layer-derived name in manifest)"`
	UserConfig      string            `yaml:"user-config"       flag:"user-config"       usage:"User config directory (default: /mnt/user-config)"`
	UserPriority    string            `yaml:"user-priority"     flag:"user-priority"     usage:"User config priority: high or low (default: high)"`
	Offline         bool              `yaml:"offline"           flag:"offline"           usage:"Use cached remote layers without network access"`
	MaxRemoteLayers int               `yaml:"max-remote-layers" flag:"max-remote-layers" usage:"Maximum remote layer count (default: 50, -1 for unlimited)"`
	Scope           string            `yaml:"scope"             flag:"scope"             usage:"Compilation scope: project or user (default: project)"`
	Package       string            `yaml:"package"           flag:"package"           usage:"Archive format for compiled output: tar.gz or zip"`
	Locked        bool              `yaml:"locked"            flag:"locked"            usage:"Verify layer content matches lockfile before compiling"`
	AllowInstall  bool              `yaml:"allow-install"     flag:"allow-install"     usage:"Execute dep install/download commands during compilation"`
	Var           map[string]string `yaml:"variables"         flag:"var"               short:"v" usage:"Set a template variable (repeatable, e.g. -v env=staging)"`
	VarFile       []string          `yaml:"-"                 flag:"var-file"          usage:"Load variables from a KEY=VALUE file (repeatable)"`
	Sign          bool              `yaml:"sign"              flag:"sign"              usage:"Sign the compiled manifest with Ed25519 key"`
	SignKey       string            `yaml:"sign-key"          flag:"sign-key"          usage:"Key name for signing (default: default)"`
	VerifyLayers  bool              `yaml:"verify-layers"     flag:"verify-layers"     usage:"Require and verify layer signatures before compiling"`
	IncludeLocal  bool              `yaml:"include-local"     flag:"include-local"     usage:"Include .keel/local layer even at user scope (normally skipped)"`
}

// RegisterFlags uses reflection to iterate over CompileConfig fields and register
// the appropriate pflag type for each one.
// This means adding a new compile parameter requires only adding a struct field.
func RegisterFlags(cmd *cobra.Command, cfg *CompileConfig) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		flagName := field.Tag.Get("flag")
		usage := field.Tag.Get("usage")
		if flagName == "" {
			continue
		}

		ptr := v.Field(i).Addr().Interface()
		defaultVal := field.Tag.Get("default")
		short := field.Tag.Get("short")

		switch field.Type.Kind() {
		case reflect.String:
			cmd.Flags().StringVarP(ptr.(*string), flagName, short, defaultVal, usage)
		case reflect.Bool:
			cmd.Flags().BoolVarP(ptr.(*bool), flagName, short, false, usage)
		case reflect.Int:
			defaultInt := 0
			if defaultVal != "" {
				fmt.Sscanf(defaultVal, "%d", &defaultInt)
			}
			cmd.Flags().IntVarP(ptr.(*int), flagName, short, defaultInt, usage)
		case reflect.Map:
			cmd.Flags().StringToStringVarP(ptr.(*map[string]string), flagName, short, nil, usage)
		case reflect.Slice:
			cmd.Flags().StringArrayVarP(ptr.(*[]string), flagName, short, nil, usage)
		default:
			// Other reflect.Kind values are not used in CompileConfig
		}
	}
}

// LoadConfigFile reads and parses a YAML config file into a CompileConfig.
// Unknown YAML keys produce warnings (via slog.Warn) but do not cause errors,
// supporting forward compatibility.
func LoadConfigFile(path string) (*CompileConfig, error) {
	data, err := pathutil.SafeReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	// First pass: decode into a map to detect unknown keys.
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	// Build the set of known YAML keys from struct tags.
	known := knownYAMLKeys()
	for key := range raw {
		if _, ok := known[key]; !ok {
			slog.Warn("unknown key in config file", "key", key, "file", path)
		}
	}

	// Second pass: decode into the struct.
	cfg := &CompileConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	return cfg, nil
}

// knownYAMLKeys returns the set of yaml tag values from CompileConfig.
func knownYAMLKeys() map[string]struct{} {
	t := reflect.TypeOf(CompileConfig{})
	keys := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		// Strip options like ",omitempty" — only the key name matters.
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		keys[tag] = struct{}{}
	}
	return keys
}

// ApplyDefaults fills in env var fallbacks for fields that are still at their
// zero value after config file and CLI flag processing.
func (cfg *CompileConfig) ApplyDefaults() {
	if len(cfg.Layers) == 0 {
		if envLayers := os.Getenv("CONFIG_LAYERS"); envLayers != "" {
			cfg.Layers = remote.SplitLayerSegments(envLayers)
		}
	}

	if len(cfg.Output) == 0 {
		if detected := DetectOutputTargets("."); len(detected) > 0 {
			cfg.Output = detected
		}
	}

	if cfg.UserConfig == "" {
		cfg.UserConfig = envOrDefault("USER_CONFIG", "/mnt/user-config")
	}

	if cfg.UserPriority == "" {
		cfg.UserPriority = envOrDefault("USER_PRIORITY", "high")
	}
}

// DetectOutputTargets scans dir for agent marker directories from the agents
// registry and returns output mappings (e.g. {"claude": "."}).
func DetectOutputTargets(dir string) map[string]string {
	targets := make(map[string]string)
	for _, agent := range agents {
		for _, d := range agent.Dirs {
			path := filepath.Join(dir, d)
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				targets[string(agent.Format)] = dir
				break
			}
		}
	}
	return targets
}

// envOrDefault returns the value of the named environment variable, or the
// fallback if the variable is unset or empty.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// NormalizeLayers merges CLI layer flags, config file layer entries, and
// the .keel/local override into a single ordered []LayerEntry slice.
// CLI flags take precedence — if --layer was set, config file LayerEntries
// are ignored. The .keel/local layer is appended last when present, but
// skipped at user scope (use IncludeLocal to override).
func (cfg *CompileConfig) NormalizeLayers() []LayerEntry {
	var entries []LayerEntry

	if len(cfg.Layers) > 0 {
		entries = make([]LayerEntry, len(cfg.Layers))
		for i, path := range cfg.Layers {
			entries[i] = LayerEntry{Source: path}
		}
	} else {
		entries = append(entries, cfg.LayerEntries...)
	}

	// Skip .keel/local at user scope — local overrides are project-specific
	// and don't belong in user-level output. Use --include-local to override.
	skipLocal := cfg.Scope == "user" && !cfg.IncludeLocal
	if !skipLocal {
		localLayer := filepath.Join(".keel", "local")
		if info, err := os.Stat(localLayer); err == nil && info.IsDir() {
			entries = append(entries, LayerEntry{Source: localLayer})
		}
	}

	return entries
}
