package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/signing"
)

func newVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [path]",
		Short: "Verify signatures on a layer or compiled output",
		Long: `Verify cryptographic signatures on a layer directory or compiled manifest.

For layers: checks the keel.sig file against the current layer content and the keystore.
For compiled output: verifies the manifest signature and shows per-layer signature status.

The signing key must be in your keystore (either as a local key or a trusted key).

Examples:
  keel verify                   # verify current directory
  keel verify ./my-layer        # verify specific layer
  keel verify /path/to/output   # verify compiled manifest in output directory`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			return runVerify(cmd, path)
		},
	}

	return cmd
}

func runVerify(cmd *cobra.Command, path string) error {
	// Validate that the path is a directory.
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("accessing path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path must be a directory: %s", path)
	}

	ks := signing.DefaultKeyStore()

	// First, try to find compiled manifests (agent output directories).
	agentDirs := getAgentDirs()
	manifests, manifestPaths, err := manifest.FindKeelState(path, agentDirs)
	if err != nil {
		return fmt.Errorf("searching for manifests: %w", err)
	}

	if len(manifests) > 0 {
		// Found manifests — verify them.
		return verifyManifests(cmd, manifests, manifestPaths, ks)
	}

	// No manifests found — try layer verification.
	return verifyLayer(cmd, path, ks)
}

// getAgentDirs returns the list of known agent directory names.
func getAgentDirs() []string {
	agents := config.AgentList()
	var dirs []string
	for _, agent := range agents {
		dirs = append(dirs, agent.Dirs...)
	}
	return dirs
}

// printVerifyResult prints the verification result in a consistent format.
func printVerifyResult(cmd *cobra.Command, result *signing.VerifyResult, successLabel, failureLabel string) {
	if result.Valid {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", successLabel)
		fmt.Fprintf(cmd.OutOrStdout(), "Key ID:       %s\n", result.KeyID)
		fmt.Fprintf(cmd.OutOrStdout(), "Key name:     %s\n", result.KeyName)
		fmt.Fprintf(cmd.OutOrStdout(), "Content hash: %s\n", result.ContentHash)
		fmt.Fprintf(cmd.OutOrStdout(), "Signed at:    %s\n", result.SignedAt)
	} else {
		fmt.Fprintf(cmd.OutOrStderr(), "%s\n", failureLabel)
		if result.KeyID != "" {
			fmt.Fprintf(cmd.OutOrStderr(), "Key ID:       %s\n", result.KeyID)
		}
		if result.KeyName != "" {
			fmt.Fprintf(cmd.OutOrStderr(), "Key name:     %s\n", result.KeyName)
		}
		if result.ContentHash != "" {
			fmt.Fprintf(cmd.OutOrStderr(), "Content hash: %s\n", result.ContentHash)
		}
		if result.SignedAt != "" {
			fmt.Fprintf(cmd.OutOrStderr(), "Signed at:    %s\n", result.SignedAt)
		}
		fmt.Fprintf(cmd.OutOrStderr(), "Error:        %s\n", result.Error)
	}
}

// verifyManifests verifies one or more compiled manifests.
func verifyManifests(cmd *cobra.Command, manifests []*manifest.CompiledManifest, paths []string, ks *signing.KeyStore) error {
	allValid := true

	for i, cm := range manifests {
		agentDir := paths[i]
		fmt.Fprintf(cmd.OutOrStdout(), "\nVerifying manifest: %s\n", agentDir)

		result, err := signing.VerifyManifest(cm, ks, nil)
		if err != nil {
			return fmt.Errorf("verifying manifest %s: %w", agentDir, err)
		}

		printVerifyResult(cmd, result, "Manifest signature valid", "Manifest signature verification failed")
		if !result.Valid {
			allValid = false
		}

		// Show per-layer signature status.
		if len(cm.LayerStack) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "\nLayer signatures:\n")
			for _, lr := range cm.LayerStack {
				if lr.LayerSignature != "" {
					verifiedMark := ""
					if lr.SignatureVerified {
						verifiedMark = " (verified)"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: signed by %s%s\n", lr.Path, lr.SignerKeyID, verifiedMark)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: not signed\n", lr.Path)
				}
			}
		}
	}

	if !allValid {
		return fmt.Errorf("manifest signature verification failed")
	}
	return nil
}

// verifyLayer verifies a layer signature.
func verifyLayer(cmd *cobra.Command, layerPath string, ks *signing.KeyStore) error {
	// Check for keel.sig (basic layer check).
	sigPath := filepath.Join(layerPath, "keel.sig")
	if _, err := os.Stat(sigPath); err != nil {
		if os.IsNotExist(err) {
			// Also check for keel.yml to give a better error message.
			keelYmlPath := filepath.Join(layerPath, "keel.yml")
			if _, err := os.Stat(keelYmlPath); os.IsNotExist(err) {
				return fmt.Errorf("not a valid layer or compiled output: no keel.yml or .keel.json found in %s", layerPath)
			}
			return fmt.Errorf("layer is not signed: keel.sig not found in %s", layerPath)
		}
		return fmt.Errorf("checking for signature file: %w", err)
	}

	// Verify the signature.
	result, err := signing.VerifyLayerSignature(layerPath, ks, nil)
	if err != nil {
		return fmt.Errorf("verifying signature: %w", err)
	}

	// Print results.
	printVerifyResult(cmd, result, "Signature valid", "Signature verification failed")
	if !result.Valid {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}
