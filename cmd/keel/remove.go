package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func newRemoveCmd() *cobra.Command {
	var noCompile bool

	cmd := &cobra.Command{
		Use:   "remove <layer>",
		Short: "Remove a configuration layer from the project",
		Long: `Remove a configuration layer from .keel/config.yml and recompile.

The layer source must match exactly as it appears in the config file.
Use 'keel explain' or inspect .keel/config.yml to see current layers.

Examples:
  keel remove ./layers/team-standards
  keel remove 'https://git.example.com/org/policy.git#main:keel'
  keel remove ./my-layer --no-compile`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(cmd.OutOrStdout(), ".", args[0], noCompile)
		},
	}

	cmd.Flags().BoolVar(&noCompile, "no-compile", false, "Remove layer without recompiling")
	return cmd
}

func runRemove(w io.Writer, dir string, layerSource string, noCompile bool) error {
	configPath := filepath.Join(dir, ".keel", "config.yml")

	// Load existing config
	cfg, err := config.LoadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("loading config file: %w (does .keel/config.yml exist?)", err)
	}

	// Check if layer exists
	if !config.LayerExistsInConfig(cfg, layerSource) {
		fmt.Fprintf(w, "Layer %q not found in config, skipping\n", layerSource)
		return nil
	}

	// Remove the layer
	if err := config.RemoveLayerFromConfigFile(configPath, layerSource); err != nil {
		return fmt.Errorf("removing layer from config: %w", err)
	}

	fmt.Fprintf(w, "Removed layer %q from .keel/config.yml\n", layerSource)

	// Auto-compile unless --no-compile was set
	if !noCompile {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Compiling...")

		// Reload config to get the updated state
		cfg, err := config.LoadConfigFile(configPath)
		if err != nil {
			return fmt.Errorf("loading config for compilation: %w", err)
		}

		cfg.ApplyDefaults()

		if err := runCompile(w, *cfg, nil, compileFlags{}); err != nil {
			return fmt.Errorf("compilation failed: %w", err)
		}
	}

	return nil
}
