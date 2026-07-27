package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/globaldeps"
)

func newGlobalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "global",
		Short: "Manage globally installed dependencies",
		Long: `Manage dependencies installed to ~/.keel/bin/ by layers with global: true.

Global dependencies are shared across projects. The global manifest
tracks which projects use each dependency for safe cleanup.

Use the subcommands to inspect and clean global dependencies.`,
	}

	cmd.AddCommand(newGlobalListCmd())
	cmd.AddCommand(newGlobalCleanCmd())

	return cmd
}

func newGlobalListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List globally installed dependencies",
		Long:  "List all globally installed dependencies with consumer count and install age.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGlobalList(cmd.OutOrStdout(), globaldeps.ManifestPath())
		},
	}
}

func newGlobalCleanCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove orphaned global dependencies",
		Long: `Remove global dependencies that have no remaining consumer projects.

Cleans stale consumer paths (deleted project directories) first,
then removes binaries from ~/.keel/bin/ for deps with zero consumers.

Use --yes to skip confirmation prompts.

Examples:
  keel global clean         # interactive confirmation per orphan
  keel global clean --yes   # remove all orphans without prompting`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGlobalClean(cmd.OutOrStdout(), cmd.InOrStdin(), globaldeps.ManifestPath(), yes)
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Remove orphaned deps without prompting")

	return cmd
}

// runGlobalList prints a table of all globally installed dependencies.
func runGlobalList(w io.Writer, manifestPath string) error {
	manifest, err := globaldeps.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("loading global manifest: %w", err)
	}

	if len(manifest.Deps) == 0 {
		fmt.Fprintln(w, "No global dependencies installed.")
		return nil
	}

	// Collect and sort dep names for stable output.
	names := make([]string, 0, len(manifest.Deps))
	for name := range manifest.Deps {
		names = append(names, name)
	}
	sort.Strings(names)

	binDir := filepath.Dir(manifestPath)
	fmt.Fprintf(w, "Global dependencies (%s/):\n\n", binDir)
	fmt.Fprintf(w, "  %-24s %-11s %s\n", "NAME", "CONSUMERS", "INSTALLED")

	totalConsumers := 0
	for _, name := range names {
		dep := manifest.Deps[name]
		consumers := len(dep.Consumers)
		totalConsumers += consumers

		age := "unknown"
		if dep.InstalledAt != "" {
			if t, err := time.Parse(time.RFC3339, dep.InstalledAt); err == nil {
				age = formatAge(t)
			}
		}

		fmt.Fprintf(w, "  %-24s %-11d %s\n", name, consumers, age)
	}

	fmt.Fprintf(w, "\n  %d deps, %d consumers\n", len(names), totalConsumers)

	return nil
}

// runGlobalClean removes orphaned global dependencies.
func runGlobalClean(w io.Writer, r io.Reader, manifestPath string, yes bool) error {
	manifest, err := globaldeps.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("loading global manifest: %w", err)
	}

	if len(manifest.Deps) == 0 {
		fmt.Fprintln(w, "No orphaned global dependencies.")
		return nil
	}

	// Clean dead consumers first (project dirs that no longer exist).
	manifest.CleanDeadConsumers()

	orphans := manifest.OrphanedDeps()
	sort.Strings(orphans)

	if len(orphans) == 0 {
		// Save the manifest in case CleanDeadConsumers changed it
		// (removed dead consumers but deps still have live consumers).
		if saveErr := globaldeps.SaveManifest(manifest, manifestPath); saveErr != nil {
			return fmt.Errorf("saving global manifest: %w", saveErr)
		}
		fmt.Fprintln(w, "No orphaned global dependencies.")
		return nil
	}

	binDir := filepath.Dir(manifestPath)
	reader := bufio.NewReader(r)
	removed := 0

	for _, name := range orphans {
		if !yes {
			fmt.Fprintf(w, "Remove %s from %s/? [y/N] ", name, binDir)
			answer, err := reader.ReadString('\n')
			if err != nil {
				// EOF or error -- treat as "no" for remaining items.
				break
			}
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				continue
			}
		}

		// Remove the binary.
		binPath := filepath.Join(binDir, name)
		if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(w, "  warning: could not remove %s: %v\n", binPath, err)
		}

		// Remove from manifest.
		delete(manifest.Deps, name)
		removed++
		fmt.Fprintf(w, "  removed %s\n", name)
	}

	// Save updated manifest.
	if saveErr := globaldeps.SaveManifest(manifest, manifestPath); saveErr != nil {
		return fmt.Errorf("saving global manifest: %w", saveErr)
	}

	if removed == 0 {
		fmt.Fprintln(w, "No global dependencies removed.")
	} else {
		fmt.Fprintf(w, "Removed %d global dependencies.\n", removed)
	}

	return nil
}
