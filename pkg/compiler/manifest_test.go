package compiler

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// --- FindKeelState tests ---

func TestFindKeelState_NoKeelStateAnywhere(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	manifests, paths, err := FindKeelState(dir)
	require.NoError(t, err)
	assert.Nil(t, manifests)
	assert.Nil(t, paths)
}

func TestFindKeelState_DirectlyInDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cm := manifest.CompiledManifest{Agent: "claude"}
	data, err := json.MarshalIndent(&cm, "", "  ")
	require.NoError(t, err)
	testutil.WriteFile(t, dir, ".keel.json", data)

	manifests, paths, err := FindKeelState(dir)
	require.NoError(t, err)
	require.Len(t, manifests, 1)
	assert.Equal(t, "claude", manifests[0].Agent)
	require.Len(t, paths, 1)
	assert.Equal(t, dir, paths[0])
}

func TestFindKeelState_InAgentSubdir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Find an agent dir from the registry to use as the subdir.
	agentList := config.AgentList()
	require.NotEmpty(t, agentList, "need at least one registered agent")
	agentDir := agentList[0].Dirs[0]

	cm := manifest.CompiledManifest{Agent: string(agentList[0].Format)}
	data, err := json.MarshalIndent(&cm, "", "  ")
	require.NoError(t, err)
	testutil.WriteFile(t, dir, filepath.Join(agentDir, ".keel.json"), data)

	manifests, paths, err := FindKeelState(dir)
	require.NoError(t, err)
	require.Len(t, manifests, 1)
	assert.Equal(t, string(agentList[0].Format), manifests[0].Agent)
	require.Len(t, paths, 1)
	assert.Equal(t, filepath.Join(dir, agentDir), paths[0])
}

func TestFindKeelState_MultipleAgentSubdirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	agentList := config.AgentList()
	if len(agentList) < 2 {
		t.Skip("need at least two registered agents to test multiple subdirs")
	}

	// Write .keel.json in each agent's first directory.
	for _, agent := range agentList {
		if len(agent.Dirs) == 0 {
			continue
		}
		cm := manifest.CompiledManifest{Agent: string(agent.Format)}
		data, err := json.MarshalIndent(&cm, "", "  ")
		require.NoError(t, err)
		testutil.WriteFile(t, dir, filepath.Join(agent.Dirs[0], ".keel.json"), data)
	}

	manifests, paths, err := FindKeelState(dir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(manifests), 2, "expected at least 2 manifests")
	assert.Equal(t, len(manifests), len(paths), "manifests and paths should have same length")

	// Verify each manifest has a non-empty Agent.
	for _, m := range manifests {
		assert.NotEmpty(t, m.Agent)
	}
}

func TestFindKeelState_CorruptKeelJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write corrupt JSON directly in dir.
	testutil.WriteFile(t, dir, ".keel.json", []byte("{corrupt json!!!"))

	manifests, paths, err := FindKeelState(dir)
	assert.Error(t, err, "corrupt .keel.json should return an error")
	assert.Nil(t, manifests)
	assert.Nil(t, paths)
}

func TestFindKeelState_CorruptKeelJSON_InAgentSubdir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	agentList := config.AgentList()
	require.NotEmpty(t, agentList)
	agentDir := agentList[0].Dirs[0]

	testutil.WriteFile(t, dir, filepath.Join(agentDir, ".keel.json"), []byte("not valid json"))

	manifests, paths, err := FindKeelState(dir)
	assert.Error(t, err, "corrupt .keel.json in agent subdir should return an error")
	assert.Nil(t, manifests)
	assert.Nil(t, paths)
}

func TestFindKeelState_NonexistentDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "does-not-exist")

	// manifest.LoadKeelState handles os.IsNotExist, so FindKeelState returns nil, nil, nil.
	manifests, paths, err := FindKeelState(dir)
	require.NoError(t, err)
	assert.Nil(t, manifests)
	assert.Nil(t, paths)
}

