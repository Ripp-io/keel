---
name: add-claude-setting
description: Add a new Claude Code settings key as a first-class DSL field. After the json.Marshal refactor, this requires updating exactly ONE place — the Settings struct with yaml and json tags. Reflection-based merge and json.Marshal extraction handle the rest automatically.
---

# Add a Claude Code Settings Key

One-step process. The `Settings` struct in `schema.go` is the single source of truth.

## Step 1: Add the field to the Settings struct

**File:** `pkg/schema/schema.go`

Add a field to the `Settings` struct with both `yaml` and `json` tags:

```go
// --- Category comment ---
MyNewSetting string `yaml:"my-new-setting,omitempty" json:"myNewSetting,omitempty"`
```

Tag conventions:
- `yaml` tag: kebab-case (DSL authoring format)
- `json` tag: camelCase matching Claude's settings.json key name exactly
- Both tags must have `omitempty` to avoid writing zero values — except `AttributionConfig` fields where empty string means "disable"

Type selection:
- `string` for text values
- `*bool` for optional booleans (nil = not set, distinguishes from false)
- `*int` for optional integers
- `*float64` for optional floats
- `[]string` for string lists
- `map[string]string` for typed maps
- `map[string]any` for polymorphic/complex maps (e.g., plugin configs)
- `[]any` for polymorphic arrays (e.g., marketplace entries)
- `*SubStructType` for nested objects (create a new struct if needed)

## What happens automatically

- **`extractClaudeSettings`** (reader_dsl.go): Uses `json.Marshal(s)` on the Settings struct. Your new field is included automatically via its json tag.
- **`mergeSettings`** (defaults.go): Uses `reflect.Value.IsZero()` to merge fields. Your new field is included automatically.
- **`TestSettings_AllFieldsHaveJSONTags`** (schema_test.go): Catches missing json tags at test time.

## If adding a nested sub-struct

1. Define the struct in `schema.go` with both yaml and json tags on every field
2. Add a pointer field on `Settings`: `MyConfig *MyConfigType \`yaml:"my-config,omitempty" json:"myConfig,omitempty"\``
3. Add the struct to the `TestSettings_AllFieldsHaveJSONTags` test's type list

## Verification

```bash
go test ./pkg/schema/ -run TestSettings_AllFieldsHaveJSONTags  # catches missing json tags
go test ./...                                                   # full suite
```

## Example: adding a hypothetical "theme" setting

```go
// In Settings struct:
Theme string `yaml:"theme,omitempty" json:"theme,omitempty"`
```

That's it. Write `theme: dark` in keel.yml settings, it appears as `"theme": "dark"` in `.claude/settings.json`.

## Reference

- Settings struct: `pkg/schema/schema.go`
- Extraction: `pkg/compiler/reader_dsl.go` → `extractClaudeSettings`
- Merge: `pkg/schema/defaults.go` → `mergeSettings`
- Completeness test: `pkg/schema/schema_test.go` → `TestSettings_AllFieldsHaveJSONTags`
- Claude JSON schema: https://json.schemastore.org/claude-code-settings.json
