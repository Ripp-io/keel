package sarif

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReport(t *testing.T) {
	report := NewReport()

	assert.Equal(t, SchemaURL, report.Schema)
	assert.Equal(t, Version, report.Version)
	assert.Empty(t, report.Runs)
}

func TestNewRun(t *testing.T) {
	run := NewRun("keel", "1.0.0")

	assert.Equal(t, "keel", run.Tool.Driver.Name)
	assert.Equal(t, "1.0.0", run.Tool.Driver.SemanticVersion)
	assert.Empty(t, run.Tool.Driver.Rules)
	assert.Empty(t, run.Results)
}

func TestAddRule(t *testing.T) {
	run := NewRun("keel", "1.0.0")
	run.AddRule("test/rule", "Short description", "Full description", "warning")

	require.Len(t, run.Tool.Driver.Rules, 1)
	rule := run.Tool.Driver.Rules[0]

	assert.Equal(t, "test/rule", rule.ID)
	assert.Equal(t, "Short description", rule.ShortDescription.Text)
	assert.Equal(t, "Full description", rule.FullDescription.Text)
	require.NotNil(t, rule.DefaultConfiguration)
	assert.Equal(t, "warning", rule.DefaultConfiguration.Level)
}

func TestAddResult(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		message  string
		level    string
		filePath string
		wantLocs bool
	}{
		{
			name:     "result with file location",
			ruleID:   "test/rule",
			message:  "Test message",
			level:    "warning",
			filePath: "path/to/file.txt",
			wantLocs: true,
		},
		{
			name:     "result without file location",
			ruleID:   "test/rule2",
			message:  "Another message",
			level:    "error",
			filePath: "",
			wantLocs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := NewRun("keel", "1.0.0")
			run.AddResult(tt.ruleID, tt.message, tt.level, tt.filePath)

			require.Len(t, run.Results, 1)
			result := run.Results[0]

			assert.Equal(t, tt.ruleID, result.RuleID)
			assert.Equal(t, tt.message, result.Message.Text)
			assert.Equal(t, tt.level, result.Level)

			if tt.wantLocs {
				require.Len(t, result.Locations, 1)
				assert.Equal(t, tt.filePath, result.Locations[0].PhysicalLocation.ArtifactLocation.URI)
			} else {
				assert.Empty(t, result.Locations)
			}
		})
	}
}

func TestToJSON(t *testing.T) {
	report := NewReport()
	run := NewRun("keel", "1.0.0")
	run.AddRule("keel/drift/modified", "File modified", "File content has changed", "warning")
	run.AddResult("keel/drift/modified", "File was modified", "warning", "CLAUDE.md")
	report.Runs = append(report.Runs, *run)

	data, err := report.ToJSON()
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Check structure
	assert.Equal(t, SchemaURL, parsed["$schema"])
	assert.Equal(t, Version, parsed["version"])

	runs, ok := parsed["runs"].([]interface{})
	require.True(t, ok)
	require.Len(t, runs, 1)

	run0 := runs[0].(map[string]interface{})
	tool := run0["tool"].(map[string]interface{})
	driver := tool["driver"].(map[string]interface{})
	assert.Equal(t, "keel", driver["name"])
	assert.Equal(t, "1.0.0", driver["semanticVersion"])

	rules := driver["rules"].([]interface{})
	require.Len(t, rules, 1)

	results := run0["results"].([]interface{})
	require.Len(t, results, 1)
}

func TestSARIFCompliance(t *testing.T) {
	// This test verifies the SARIF output structure matches the 2.1.0 schema expectations
	report := NewReport()
	run := NewRun("keel", "1.0.0")

	// Add standard Keel drift rules
	run.AddRule("keel/drift/modified", "File content changed", "The file has been modified from its compiled state", "warning")
	run.AddRule("keel/drift/missing", "File missing", "The file is missing from disk but present in the manifest", "warning")
	run.AddRule("keel/drift/policy-violation", "Policy violation", "A force-locked file has been modified", "error")

	// Add sample results
	run.AddResult("keel/drift/modified", "File CLAUDE.md was modified", "warning", "CLAUDE.md")
	run.AddResult("keel/drift/missing", "File commands/deploy.md is missing", "warning", "commands/deploy.md")
	run.AddResult("keel/drift/policy-violation", "Force-locked file CLAUDE.md was modified", "error", "CLAUDE.md")

	report.Runs = append(report.Runs, *run)

	data, err := report.ToJSON()
	require.NoError(t, err)

	// Parse and verify required fields
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Verify required top-level fields
	assert.Equal(t, SchemaURL, parsed["$schema"])
	assert.Equal(t, Version, parsed["version"])
	assert.Contains(t, parsed, "runs")

	// Verify run structure
	runs := parsed["runs"].([]interface{})
	require.Len(t, runs, 1)
	run0 := runs[0].(map[string]interface{})

	// Verify tool.driver structure
	tool := run0["tool"].(map[string]interface{})
	driver := tool["driver"].(map[string]interface{})
	assert.Equal(t, "keel", driver["name"])
	assert.Contains(t, driver, "rules")

	// Verify rules
	rules := driver["rules"].([]interface{})
	require.Len(t, rules, 3)

	// Verify defaultConfiguration is an object with a level key (SARIF 2.1.0 spec)
	for i, r := range rules {
		rule := r.(map[string]interface{})
		if defaultConfig, ok := rule["defaultConfiguration"]; ok {
			config := defaultConfig.(map[string]interface{})
			assert.Contains(t, config, "level", "rule %d defaultConfiguration must have a level key", i)
			level := config["level"].(string)
			assert.NotEmpty(t, level, "rule %d level must not be empty", i)
		}
	}

	// Verify results
	results := run0["results"].([]interface{})
	require.Len(t, results, 3)

	// Check result structure
	result0 := results[0].(map[string]interface{})
	assert.Contains(t, result0, "ruleId")
	assert.Contains(t, result0, "message")
	assert.Contains(t, result0, "level")
	assert.Contains(t, result0, "locations")

	// Verify location structure
	locations := result0["locations"].([]interface{})
	require.Len(t, locations, 1)
	loc0 := locations[0].(map[string]interface{})
	physicalLocation := loc0["physicalLocation"].(map[string]interface{})
	artifactLocation := physicalLocation["artifactLocation"].(map[string]interface{})
	assert.Equal(t, "CLAUDE.md", artifactLocation["uri"])
}
