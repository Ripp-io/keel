package main

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/globaldeps"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func newCleanCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clean [dir]",
		Short: "Remove previously compiled output",
		Long: `Remove all files produced by a previous 'keel compile', preserving
user-created files that are not tracked in the compilation manifest.

Reads .keel.json to determine which files were compiled, then removes them.

Auto-discovers .keel.json in agent subdirectories (.claude, .cursor).
If multiple agents are found, cleans all of them.

Use this when you want to start fresh without recompiling.

Examples:
  keel clean
  keel clean ./output
  keel clean .claude`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			// Try to find .keel.json files
			manifests, paths, err := compiler.FindKeelState(dir)
			if err != nil {
				return err
			}

			if len(manifests) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No compiled output to clean.")
				return nil
			}

			totalRemoved := 0
			for i, cm := range manifests {
				// Derive outputRoot from the path relative to dir
				agentDir := paths[i]
				outputDir := dir

				// If the .keel.json was found in an agent subdir, the output root
				// is the subdir relative to the parent.
				outputRoot := ""
				if agentDir != dir {
					outputDir = filepath.Dir(agentDir)
					outputRoot = filepath.Base(agentDir)
				} else {
					// .keel.json in dir directly -- look up agent to find outputRoot
					if cm.Agent != "" {
						meta := config.LookupAgent(cm.Agent)
						if meta != nil && len(meta.Dirs) > 0 {
							outputRoot = meta.Dirs[0]
						}
					}
				}

				pruned, cleanErr := manifest.CleanAllOutput(outputDir, outputRoot, force)
				if cleanErr != nil {
					return cleanErr
				}
				if pruned == nil || len(pruned.Removed) == 0 {
					continue
				}
				if len(manifests) > 1 {
					fmt.Fprintf(cmd.OutOrStdout(), "%s:\n", cm.Agent)
				}
				for _, f := range pruned.Removed {
					fmt.Fprintf(cmd.OutOrStdout(), "  removed %s\n", f)
				}
				totalRemoved += len(pruned.Removed)
			}

			if totalRemoved == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No compiled output to clean.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nCleaned %d files.\n", totalRemoved)

			// Deregister this project from the global manifest.
			manifestPath := globaldeps.ManifestPath()
			mf, loadErr := globaldeps.LoadManifest(manifestPath)
			if loadErr == nil && len(mf.Deps) > 0 {
				absDir, _ := filepath.Abs(dir)
				changed := false

				for depName, dep := range mf.Deps {
					// Check if this project is a consumer of this dep.
					isConsumer := false
					for _, c := range dep.Consumers {
						if c == absDir {
							isConsumer = true
							break
						}
					}
					if !isConsumer {
						continue
					}

					changed = true
					orphaned := mf.DeregisterConsumer(depName, absDir)
					if orphaned {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s: no other consumers (run 'keel global clean' to remove)\n", depName)
					} else {
						remaining := len(mf.Deps[depName].Consumers)
						fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d other project(s) still use this dep, keeping global install\n", depName, remaining)
					}
				}

				if changed {
					if saveErr := globaldeps.SaveManifest(mf, manifestPath); saveErr != nil {
						slog.Warn("failed to update global manifest", "error", saveErr)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Also remove user-editable files (e.g., CLAUDE.md) that are normally preserved")

	return cmd
}
