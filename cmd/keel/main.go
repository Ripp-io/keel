// Keel — Governed configuration and delivery infrastructure for AI coding agents.
//
// CLI for compiling multi-layer configuration into tool-native output.
package main

import (
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/cli"
)

var version = "dev"

const shortRevLen = 7

func init() {
	if version != "dev" {
		return // -ldflags override
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var revision string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if len(revision) >= shortRevLen {
		version = revision[:shortRevLen]
		if modified {
			version += "-dirty"
		}
	}
}

func main() {
	var dbg int

	rootCmd := &cobra.Command{
		Use:     "keel",
		Short:   "Governed configuration for AI coding agents",
		Long:    "Keel compiles multi-layer configuration into tool-native output for AI coding tools.",
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cli.InitLogging(dbg)
		},
	}

	rootCmd.PersistentFlags().CountVarP(&dbg, "debug", "d", "Increase verbosity (--debug info, -dd debug)")
	rootCmd.AddCommand(newAddCmd())
	rootCmd.AddCommand(newCompileCmd())
	rootCmd.AddCommand(newExplainCmd())
	rootCmd.AddCommand(newReportCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newCacheCmd())
	rootCmd.AddCommand(newImportCmd())
	rootCmd.AddCommand(newPackageCmd())
	rootCmd.AddCommand(newRemoveCmd())
	rootCmd.AddCommand(newCleanCmd())
	rootCmd.AddCommand(newDriftCmd())
	rootCmd.AddCommand(newSnapshotCmd())
	rootCmd.AddCommand(newGlobalCmd())
	rootCmd.AddCommand(newKeyCmd())
	rootCmd.AddCommand(newSignCmd())
	rootCmd.AddCommand(newVerifyCmd())
	rootCmd.SilenceUsage = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
