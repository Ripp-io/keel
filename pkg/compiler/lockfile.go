package compiler

import (
	"fmt"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// VerifyKeelState reads .keel.json from agentDir and verifies all layer
// hashes match the current content. Returns nil if all hashes match, or an
// error describing mismatches.
func VerifyKeelState(agentDir string, layers []config.Layer) error {
	cm, err := manifest.LoadKeelState(agentDir)
	if err != nil {
		return err
	}
	if cm == nil {
		return fmt.Errorf("no compiled state found at %s", filepath.Join(agentDir, ".keel.json"))
	}

	if len(cm.LayerStack) != len(layers) {
		return fmt.Errorf("layer count mismatch: state has %d layers, got %d",
			len(cm.LayerStack), len(layers))
	}

	var mismatches []string
	for i, recorded := range cm.LayerStack {
		if recorded.Path != layers[i].Path {
			return fmt.Errorf("layer order mismatch at index %d: state has %q, got %q",
				i, recorded.Path, layers[i].Path)
		}

		if recorded.Hash == "" {
			continue // no hash recorded, skip verification
		}

		currentHash, err := manifest.HashLayerDir(layers[i].Path)
		if err != nil {
			return fmt.Errorf("hashing layer %q for verification: %w", layers[i].Path, err)
		}

		if currentHash != recorded.Hash {
			mismatches = append(mismatches, fmt.Sprintf(
				"layer %q hash mismatch: expected %s got %s",
				recorded.Path, recorded.Hash, currentHash))
		}
	}

	if len(mismatches) > 0 {
		msg := fmt.Sprintf("%d layer(s) changed since state was recorded:", len(mismatches))
		for _, m := range mismatches {
			msg += "\n  " + m
		}
		return fmt.Errorf("%s", msg)
	}

	return nil
}
