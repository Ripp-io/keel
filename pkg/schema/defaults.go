package schema

import (
	_ "embed"
	"log/slog"
	"reflect"
)

//go:embed keel.default.yml
var DefaultConfigYAML []byte

// ParseDefaults returns the built-in default KeelConfig.
func ParseDefaults() (*KeelConfig, error) {
	return ParseBytes(DefaultConfigYAML)
}

// MergeWithDefaults applies user config on top of defaults using shallow merge.
// For each slice/map field: if the user config has a non-nil value, it replaces
// the default entirely. If the user config has a nil/empty value, the default
// is preserved.
func MergeWithDefaults(userConfig *KeelConfig) *KeelConfig {
	if userConfig == nil {
		defaults, err := ParseDefaults()
		if err != nil {
			slog.Error("failed to parse embedded defaults", "error", err)
			return &KeelConfig{}
		}
		return defaults
	}

	defaults, err := ParseDefaults()
	if err != nil {
		// Defaults are embedded and tested — this should never happen in production.
		slog.Error("failed to parse embedded defaults, returning user config as-is", "error", err)
		return userConfig
	}

	result := &KeelConfig{}

	// Scalars: user overrides default if non-empty
	result.Schema = firstNonEmpty(userConfig.Schema, defaults.Schema)
	result.Name = firstNonEmpty(userConfig.Name, defaults.Name)
	result.Version = firstNonEmpty(userConfig.Version, defaults.Version)
	result.Description = firstNonEmpty(userConfig.Description, defaults.Description)

	// Slices: user replaces default if non-nil
	result.Instructions = firstNonNilFileRefSlice(userConfig.Instructions, defaults.Instructions)
	if userConfig.Permissions != nil {
		result.Permissions = userConfig.Permissions
	} else {
		result.Permissions = defaults.Permissions
	}
	result.Commands = firstNonNilFileRefSlice(userConfig.Commands, defaults.Commands)
	result.Agents = firstNonNilFileRefSlice(userConfig.Agents, defaults.Agents)
	result.Skills = firstNonNilFileRefSlice(userConfig.Skills, defaults.Skills)
	result.Init = firstNonNilFileRefSlice(userConfig.Init, defaults.Init)

	result.Settings = mergeSettings(userConfig.Settings, defaults.Settings)

	if userConfig.MCP != nil {
		result.MCP = userConfig.MCP
	} else {
		result.MCP = defaults.MCP
	}
	result.Hooks = firstNonNilFileRefSlice(userConfig.Hooks, defaults.Hooks)
	result.Plugins = firstNonNilFileRefSlice(userConfig.Plugins, defaults.Plugins)
	result.AgentMemory = firstNonNilFileRefSlice(userConfig.AgentMemory, defaults.AgentMemory)
	result.Environment = firstNonNilFileRefSlice(userConfig.Environment, defaults.Environment)
	result.Rules = firstNonNilFileRefSlice(userConfig.Rules, defaults.Rules)

	// Repos: user replaces default if non-nil
	if userConfig.Repos != nil {
		result.Repos = userConfig.Repos
	} else {
		result.Repos = defaults.Repos
	}

	// Snippets: user replaces default if non-nil
	result.Snippets = firstNonNilFileRefSlice(userConfig.Snippets, defaults.Snippets)

	// Variables: user replaces default if non-nil
	if userConfig.Variables != nil {
		result.Variables = userConfig.Variables
	} else {
		result.Variables = defaults.Variables
	}

	// Bin: user replaces default if non-nil
	result.Bin = firstNonNilFileRefSlice(userConfig.Bin, defaults.Bin)

	// Loop: user replaces default if non-nil
	result.Loop = firstNonNilFileRefSlice(userConfig.Loop, defaults.Loop)

	// WorktreeInclude: user replaces default if non-nil
	result.WorktreeInclude = firstNonNilFileRefSlice(userConfig.WorktreeInclude, defaults.WorktreeInclude)

	// Deps: user replaces default if non-nil
	if userConfig.Deps != nil {
		result.Deps = userConfig.Deps
	} else {
		result.Deps = defaults.Deps
	}

	// Pass through fields that have no defaults
	result.Files = userConfig.Files
	result.ValidateOpts = userConfig.ValidateOpts

	return result
}

// mergeSettings deep-merges user settings on top of defaults using reflection.
// For each field: if the user set it (non-zero value), use the user's value.
// Otherwise, preserve the default. reflect.Value.IsZero() correctly handles
// all Settings field types: nil pointers, empty strings, zero ints, nil slices/maps.
func mergeSettings(user, defaults *Settings) *Settings {
	if user == nil && defaults == nil {
		return nil
	}
	if user == nil {
		return defaults
	}
	if defaults == nil {
		return user
	}
	result := &Settings{}
	uv := reflect.ValueOf(user).Elem()
	dv := reflect.ValueOf(defaults).Elem()
	rv := reflect.ValueOf(result).Elem()
	for i := 0; i < uv.NumField(); i++ {
		uf := uv.Field(i)
		if !uf.IsZero() {
			rv.Field(i).Set(uf)
		} else {
			rv.Field(i).Set(dv.Field(i))
		}
	}
	return result
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func firstNonNilFileRefSlice(a, b []FileRef) []FileRef {
	if a != nil {
		return a
	}
	return b
}
