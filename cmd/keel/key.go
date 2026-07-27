package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/signing"
)

func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage cryptographic signing keys",
		Long: `Manage Ed25519 signing keys for Keel layer verification.

Keys are stored in ~/.keel/keys/ with two categories:
  - Local keys: Your own keypairs (both .key and .pub files)
  - Trusted keys: Public keys you trust (in trusted/ subdirectory)

Use the subcommands to generate, list, export, trust, and revoke keys.`,
	}

	cmd.AddCommand(newKeyGenerateCmd())
	cmd.AddCommand(newKeyListCmd())
	cmd.AddCommand(newKeyExportCmd())
	cmd.AddCommand(newKeyTrustCmd())
	cmd.AddCommand(newKeyRevokeCmd())

	return cmd
}

func newKeyGenerateCmd() *cobra.Command {
	var name string
	var force bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new Ed25519 keypair",
		Long: `Generate a new Ed25519 signing keypair.

The keypair is saved to ~/.keel/keys/<name>.key (private) and <name>.pub (public).
Private keys have mode 0600, public keys have mode 0644.

Examples:
  keel key generate                    # generate "default" keypair
  keel key generate --name mykey       # generate "mykey" keypair
  keel key generate --name mykey --force  # overwrite existing keypair`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyGenerate(cmd.OutOrStdout(), name, force)
		},
	}

	cmd.Flags().StringVar(&name, "name", "default", "Name for the keypair")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing keypair")

	return cmd
}

func newKeyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all stored keys",
		Long:  "List all keys in the keystore with their type and status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyList(cmd.OutOrStdout())
		},
	}
}

func newKeyExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <name>",
		Short: "Export a public key",
		Long: `Export a public key in PEM format to stdout.

This is useful for sharing your public key with others who want to
verify signatures from your private key.

Examples:
  keel key export default > default.pub
  keel key export mykey | pbcopy`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyExport(cmd.OutOrStdout(), args[0])
		},
	}
}

func newKeyTrustCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trust <pubkey-file>",
		Short: "Add a public key to the trusted keyring",
		Long: `Add a public key to the trusted keyring.

Trusted keys are copied to ~/.keel/keys/trusted/ and used to verify
signed layers. The file must be a valid Ed25519 public key in PEM format.

Examples:
  keel key trust ./org-signing-key.pub
  keel key trust /tmp/downloaded-key.pub`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyTrust(cmd.OutOrStdout(), args[0])
		},
	}
}

func newKeyRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <name-or-id>",
		Short: "Remove a key from the trusted keyring",
		Long: `Remove a key from the trusted keyring.

You can specify either the key name or the full key ID (keel:sha256:...).
Only keys in the trusted/ subdirectory can be revoked.

Examples:
  keel key revoke org-key
  keel key revoke keel:sha256:abc123...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyRevoke(cmd.OutOrStdout(), args[0])
		},
	}
}

func runKeyGenerate(w io.Writer, name string, force bool) error {
	ks := signing.DefaultKeyStore()

	// Check if key already exists (unless --force).
	if !force {
		keys, err := ks.ListKeys()
		if err == nil {
			for _, k := range keys {
				if k.Name == name && k.HasPrivate {
					return fmt.Errorf("keypair %q already exists (use --force to overwrite)", name)
				}
			}
		}
	}

	// Generate keypair.
	pub, priv, err := signing.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("generating keypair: %w", err)
	}

	// Save to disk.
	if err := ks.SaveKeyPair(name, pub, priv); err != nil {
		return fmt.Errorf("saving keypair: %w", err)
	}

	keyID := signing.KeyID(pub)
	fmt.Fprintf(w, "Generated keypair %q\n", name)
	fmt.Fprintf(w, "Key ID: %s\n", keyID)
	fmt.Fprintf(w, "Saved to: %s/%s.{key,pub}\n", ks.BaseDir, name)

	return nil
}

func runKeyList(w io.Writer) error {
	ks := signing.DefaultKeyStore()

	keys, err := ks.ListKeys()
	if err != nil {
		return fmt.Errorf("listing keys: %w", err)
	}

	if len(keys) == 0 {
		fmt.Fprintln(w, "No keys found.")
		return nil
	}

	// Print table header.
	fmt.Fprintf(w, "%-20s %-72s %-12s %s\n", "NAME", "KEY ID", "TYPE", "HAS PRIVATE")
	fmt.Fprintln(w)

	// Print keys.
	for _, key := range keys {
		keyType := "local"
		if key.Trusted {
			keyType = "trusted"
		}

		hasPrivate := "no"
		if key.HasPrivate {
			hasPrivate = "yes"
		}

		fmt.Fprintf(w, "%-20s %-72s %-12s %s\n", key.Name, key.KeyID, keyType, hasPrivate)
	}

	return nil
}

func runKeyExport(w io.Writer, name string) error {
	ks := signing.DefaultKeyStore()

	pub, err := ks.LoadPublicKey(name)
	if err != nil {
		return fmt.Errorf("loading public key: %w", err)
	}

	pem, err := signing.MarshalPublicKey(pub)
	if err != nil {
		return fmt.Errorf("marshaling public key: %w", err)
	}

	fmt.Fprint(w, string(pem))

	return nil
}

func runKeyTrust(w io.Writer, pubKeyPath string) error {
	ks := signing.DefaultKeyStore()

	if err := ks.TrustKey(pubKeyPath); err != nil {
		return fmt.Errorf("trusting key: %w", err)
	}

	// Read the key to get its ID for the success message.
	data, err := os.ReadFile(pubKeyPath)
	if err == nil {
		if pub, err := signing.UnmarshalPublicKey(data); err == nil {
			keyID := signing.KeyID(pub)
			fmt.Fprintf(w, "Trusted key %s\n", keyID)
			return nil
		}
	}

	fmt.Fprintln(w, "Key trusted successfully")
	return nil
}

func runKeyRevoke(w io.Writer, nameOrID string) error {
	ks := signing.DefaultKeyStore()

	if err := ks.RevokeKey(nameOrID); err != nil {
		return fmt.Errorf("revoking key: %w", err)
	}

	fmt.Fprintf(w, "Revoked key %q from trusted keyring\n", nameOrID)

	return nil
}
