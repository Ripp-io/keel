package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/snapshot"
)

func newSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage compiled output snapshots",
		Long:  "Capture, list, and restore compiled output snapshots for behavioral rollback.",
	}
	cmd.AddCommand(newSnapshotCreateCmd())
	cmd.AddCommand(newSnapshotListCmd())
	cmd.AddCommand(newSnapshotRestoreCmd())
	cmd.AddCommand(newSnapshotPruneCmd())
	return cmd
}

func newSnapshotCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [dir]",
		Short: "Create a snapshot of the current compiled state",
		Long: `Capture the current compiled output state for rollback.

Auto-discovers .keel.json in agent subdirectories (.claude, .cursor).
Creates snapshots for each discovered agent.

Examples:
  keel snapshot create
  keel snapshot create ./output`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			// Find .keel.json files
			manifests, paths, err := compiler.FindKeelState(dir)
			if err != nil {
				return err
			}

			if len(manifests) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No compiled output found.")
				return nil
			}

			for i, cm := range manifests {
				outputDir := dir
				agentDir := paths[i]

				if agentDir != dir {
					outputDir = filepath.Dir(agentDir)
				}

				snap, err := snapshot.Create(outputDir, agentDir)
				if err != nil {
					return fmt.Errorf("creating snapshot for %s: %w", cm.Agent, err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Snapshot created: %s (%s)\n", snap.ID, snap.Agent)
			}

			return nil
		},
	}
	return cmd
}

func newSnapshotListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [dir]",
		Short: "List all snapshots",
		Long: `List all snapshots with ID, agent, and timestamp.

Examples:
  keel snapshot list
  keel snapshot list ./output`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			snapshots, err := snapshot.List(dir)
			if err != nil {
				return err
			}

			if len(snapshots) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No snapshots found.")
				return nil
			}

			for _, snap := range snapshots {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s\n",
					snap.ID, snap.Agent, snap.CreatedAt.Format("2006-01-02 15:04:05 MST"))
			}

			return nil
		},
	}
	return cmd
}

func newSnapshotRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <id> [dir]",
		Short: "Restore a snapshot",
		Long: `Restore a previously captured snapshot, overwriting current compiled state.

Examples:
  keel snapshot restore 20260518T143022Z
  keel snapshot restore 20260518T143022Z ./output`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshotID := args[0]
			dir := "."
			if len(args) > 1 {
				dir = args[1]
			}

			// Find the snapshot
			snapshots, err := snapshot.List(dir)
			if err != nil {
				return err
			}

			var targetSnapshot *snapshot.Snapshot
			for _, snap := range snapshots {
				if snap.ID == snapshotID {
					targetSnapshot = &snap
					break
				}
			}

			if targetSnapshot == nil {
				return fmt.Errorf("snapshot %s not found", snapshotID)
			}

			// Restore
			if err := snapshot.Restore(targetSnapshot.Dir, dir); err != nil {
				return fmt.Errorf("restoring snapshot: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Restored snapshot %s\n", snapshotID)
			return nil
		},
	}
	return cmd
}

func newSnapshotPruneCmd() *cobra.Command {
	var keep int

	cmd := &cobra.Command{
		Use:   "prune [dir]",
		Short: "Remove old snapshots",
		Long: `Remove snapshots older than the Nth newest.

Examples:
  keel snapshot prune --keep 5
  keel snapshot prune --keep 3 ./output`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			removed, err := snapshot.Prune(dir, keep)
			if err != nil {
				return err
			}

			if len(removed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No snapshots to prune.")
				return nil
			}

			for _, id := range removed {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed snapshot: %s\n", id)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nPruned %d snapshots.\n", len(removed))
			return nil
		},
	}

	cmd.Flags().IntVar(&keep, "keep", 5, "Number of newest snapshots to keep")

	return cmd
}
