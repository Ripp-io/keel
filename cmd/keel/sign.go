package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/signing"
)

func newSignCmd() *cobra.Command {
	var keyName string

	cmd := &cobra.Command{
		Use:   "sign <layer-path>",
		Short: "Sign a layer with an Ed25519 key",
		Long: `Sign a layer directory with an Ed25519 private key from the keystore.

The signature is written to keel.sig alongside keel.yml in the layer directory.
The signature covers all files in the layer (except .git/ and keel.sig itself).

Examples:
  keel sign ./my-layer                    # sign with "default" key
  keel sign ./my-layer --key mykey        # sign with "mykey"
  keel sign /path/to/layer --key org-key  # sign with "org-key"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSign(cmd, args[0], keyName)
		},
	}

	cmd.Flags().StringVar(&keyName, "key", "default", "Name of the signing key to use")

	return cmd
}

func runSign(cmd *cobra.Command, layerPath, keyName string) error {
	// Validate that the path is a directory.
	info, err := os.Stat(layerPath)
	if err != nil {
		return fmt.Errorf("accessing layer path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("layer path must be a directory: %s", layerPath)
	}

	// Check that keel.yml exists (basic layer validation).
	keelYmlPath := filepath.Join(layerPath, "keel.yml")
	if _, err := os.Stat(keelYmlPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("not a valid layer: keel.yml not found in %s", layerPath)
		}
		return fmt.Errorf("checking for keel.yml: %w", err)
	}

	// Get keystore and sign the layer.
	ks := signing.DefaultKeyStore()
	sig, err := signing.SignLayer(layerPath, keyName, ks)
	if err != nil {
		return fmt.Errorf("signing layer: %w", err)
	}

	// Write the signature to disk.
	if err := signing.WriteLayerSignature(layerPath, sig); err != nil {
		return fmt.Errorf("writing signature: %w", err)
	}

	// Print success message.
	absPath, _ := filepath.Abs(layerPath)
	sigPath := filepath.Join(absPath, "keel.sig")

	fmt.Fprintf(cmd.OutOrStdout(), "Signed layer with key %q\n", keyName)
	fmt.Fprintf(cmd.OutOrStdout(), "Key ID:       %s\n", sig.KeyID)
	fmt.Fprintf(cmd.OutOrStdout(), "Content hash: %s\n", sig.ContentHash)
	fmt.Fprintf(cmd.OutOrStdout(), "Signature:    %s\n", sigPath)

	return nil
}
