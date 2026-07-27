package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gopkg.in/yaml.v3"
)

// writeFixture creates <dir>/<agentDir>/.keel.json with the given YAML content
// (parsed into a CompiledManifest and written as JSON).
// If the YAML specifies an agent field, the corresponding agent dir is used.
// Falls back to .claude if no agent is specified.
func writeFixture(t *testing.T, dir, yamlContent string) {
	t.Helper()

	var cm manifest.CompiledManifest
	require.NoError(t, yaml.Unmarshal([]byte(yamlContent), &cm), "parsing fixture YAML")

	// Determine agent dir
	agentDir := ".claude" // default
	if cm.Agent != "" {
		if meta := config.LookupAgent(cm.Agent); meta != nil && len(meta.Dirs) > 0 {
			agentDir = meta.Dirs[0]
		}
	}

	targetDir := filepath.Join(dir, agentDir)
	require.NoError(t, os.MkdirAll(targetDir, 0755), "creating agent dir")

	data, err := json.MarshalIndent(&cm, "", "  ")
	require.NoError(t, err, "marshaling fixture JSON")
	data = append(data, '\n')

	require.NoError(t, os.WriteFile(filepath.Join(targetDir, ".keel.json"), data, 0o644), "writing .keel.json fixture")
}
