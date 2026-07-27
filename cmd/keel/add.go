package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	// Import agent packages for side effects (registry registration)
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/agentsmd"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/aider"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/claude"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cline"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/copilot"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cursor"
)

func newAddCmd() *cobra.Command {
	var noCompile bool
	var outputTargets map[string]string

	cmd := &cobra.Command{
		Use:   "add <layer>",
		Short: "Add a configuration layer to the project",
		Long: `Add a configuration layer to .keel/config.yml and compile.

If no config file exists, one is created with auto-detected output targets.
The layer is appended as the last entry (highest priority before .keel/local).

Use --output to set output targets (creates them if missing, merges with existing):
  keel add ./my-layer --output claude=.
  keel add ./my-layer --output claude=. --output cursor=.

Examples:
  keel add ./layers/team-standards
  keel add 'https://git.example.com/org/policy.git#main:keel'
  keel add ./my-layer --no-compile`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd.OutOrStdout(), ".", args[0], noCompile, outputTargets)
		},
	}

	cmd.Flags().BoolVar(&noCompile, "no-compile", false, "Add layer without auto-compiling")
	cmd.Flags().StringToStringVar(&outputTargets, "output", nil, "Agent output targets as agent=path (e.g. claude=.)")
	if f := cmd.Flags().Lookup("output"); f != nil {
		f.DefValue = ""
	}
	return cmd
}

func runAdd(w io.Writer, dir string, layerSource string, noCompile bool, extraOutputs map[string]string) error {
	keelDir := filepath.Join(dir, ".keel")
	configPath := filepath.Join(keelDir, "config.yml")

	// Check if config file exists
	_, err := os.Stat(configPath)
	configExists := err == nil

	if !configExists {
		// Case 1: No config exists — create it
		if err := os.MkdirAll(keelDir, 0o755); err != nil {
			return fmt.Errorf("creating .keel directory: %w", err)
		}

		// Start with auto-detected targets, overlay --output flags
		outputs := config.DetectOutputTargets(dir)
		for agent, path := range extraOutputs {
			outputs[agent] = path
		}

		// Create config with the layer
		layers := []string{layerSource}
		if err := config.CreateConfigFile(configPath, layers, outputs); err != nil {
			return fmt.Errorf("creating config file: %w", err)
		}

		// Print summary
		if len(outputs) > 0 {
			outputStr := formatOutputMap(outputs)
			fmt.Fprintf(w, "Created .keel/config.yml (output: %s)\n", outputStr)
		} else {
			fmt.Fprintln(w, "Created .keel/config.yml (no output targets detected — set with keel compile --output)")
			noCompile = true
		}
		fmt.Fprintf(w, "Added layer %q\n", layerSource)
	} else {
		// Case 2: Config exists — add layer to it
		// First check if layer already exists
		cfg, err := config.LoadConfigFile(configPath)
		if err != nil {
			return fmt.Errorf("loading config file: %w", err)
		}

		if config.LayerExistsInConfig(cfg, layerSource) {
			fmt.Fprintf(w, "Layer %q already in config, skipping\n", layerSource)
		} else {
			if err := config.AddLayerToConfigFile(configPath, layerSource); err != nil {
				return fmt.Errorf("adding layer to config: %w", err)
			}
			fmt.Fprintf(w, "Added layer %q to .keel/config.yml\n", layerSource)
		}

		// Merge --output targets into existing config
		if len(extraOutputs) > 0 {
			if err := config.MergeOutputTargets(configPath, extraOutputs); err != nil {
				return fmt.Errorf("merging output targets: %w", err)
			}
			fmt.Fprintf(w, "Added output targets: %s\n", formatOutputMap(extraOutputs))
		}
	}

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

		// Run compilation with no variable overrides and default flags
		if err := runCompile(w, *cfg, nil, compileFlags{}); err != nil {
			return fmt.Errorf("compilation failed: %w", err)
		}
	}

	return nil
}

// formatOutputMap converts an output map to a human-readable string.
// Example: {"claude": ".", "cursor": "."} → "claude=., cursor=."
func formatOutputMap(outputs map[string]string) string {
	if len(outputs) == 0 {
		return ""
	}

	var result string
	first := true
	for agent, path := range outputs {
		if !first {
			result += ", "
		}
		result += fmt.Sprintf("%s=%s", agent, path)
		first = false
	}
	return result
}
