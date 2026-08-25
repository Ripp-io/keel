package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFold_HooksDir_IsAutoVar(t *testing.T) {
	s := newTestState()

	// keel.hooks.dir is an auto-var set at construction time,
	// pointing to .keel/hooks/ under the output directory.
	val, exists := s.variables[varKeelHooksDir]
	assert.True(t, exists, "keel.hooks.dir should be set as an auto-var")
	assert.Contains(t, val, filepath.Join(".keel", "hooks"))
}

func TestFold_HookScripts_Accumulated(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		LayerPath: "/path/to/layer",
		Hooks: map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash(*)",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "${keel:keel.hooks.dir}/guard-bash.sh",
						},
					},
				},
			},
		},
		HookScripts: []BinFile{
			{RelPath: "guard-bash.sh", Content: []byte("#!/bin/bash\necho guard"), Mode: 0755},
		},
	}

	require.NoError(t, s.Fold(ir, "layer1"))

	assert.Len(t, s.hookScripts, 1)
	assert.Equal(t, "guard-bash.sh", s.hookScripts[0].RelPath)
}

func TestFold_HookScripts_LastLayerWins(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		LayerPath: "/path/to/layer1",
		Hooks:     map[string]any{"PreToolUse": []any{}},
		HookScripts: []BinFile{
			{RelPath: "guard.sh", Content: []byte("v1"), Mode: 0755},
		},
	}

	ir2 := &LayerIR{
		LayerPath: "/path/to/layer2",
		Hooks:     map[string]any{"PostToolUse": []any{}},
		HookScripts: []BinFile{
			{RelPath: "guard.sh", Content: []byte("v2"), Mode: 0755},
		},
	}

	require.NoError(t, s.Fold(ir1, "layer1"))
	require.NoError(t, s.Fold(ir2, "layer2"))

	// Last layer wins for same RelPath
	assert.Len(t, s.hookScripts, 1)
	assert.Equal(t, []byte("v2"), s.hookScripts[0].Content)
}

func TestWriteHookScripts_DeployedCorrectly(t *testing.T) {
	outputDir := t.TempDir()

	scripts := []BinFile{
		{RelPath: "guard-bash.sh", Content: []byte("#!/bin/bash\necho guard"), Mode: 0755},
		{RelPath: "audit.sh", Content: []byte("#!/bin/bash\necho audit"), Mode: 0755},
	}

	records, err := WriteHookScripts(scripts, outputDir)
	require.NoError(t, err)
	assert.Len(t, records, 2)

	// Verify files exist at .keel/hooks/
	for _, s := range scripts {
		path := filepath.Join(outputDir, ".keel", "hooks", s.RelPath)
		info, err := os.Stat(path)
		require.NoError(t, err, "hook script %s should exist", s.RelPath)
		assert.True(t, info.Mode()&0111 != 0, "hook script should be executable")
	}

	// Verify record paths
	assert.Equal(t, filepath.Join(".keel", "hooks", "guard-bash.sh"), records[0].Output)
	assert.Equal(t, filepath.Join(".keel", "hooks", "audit.sh"), records[1].Output)
}
