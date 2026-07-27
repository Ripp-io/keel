package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/drift"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/signing"
)

func newDriftCmd() *cobra.Command {
	var policyOnly bool
	var semantic bool
	var verify bool
	var format string

	cmd := &cobra.Command{
		Use:   "drift [dir]",
		Short: "Detect manual changes to compiled output",
		Long: `Compare current disk state against the compiled manifest to detect
files that have been manually edited, added, or removed since the last compilation.

Auto-discovers .keel.json in agent subdirectories (.claude, .cursor).
Returns non-zero exit code when drift is detected.

Use --policy to check only force-locked files (policy violations).
Use --semantic to show semantic changes when drift is detected.
Use --verify to include signature verification in drift check.

Examples:
  keel drift
  keel drift ./output
  keel drift --policy .
  keel drift --semantic
  keel drift --semantic --format json
  keel drift --verify`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			manifests, paths, err := compiler.FindKeelState(dir)
			if err != nil {
				return err
			}

			if len(manifests) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No compiled output found.")
				return nil
			}

			hasDrift := false
			totalForceLocked := 0
			policyViolations := 0

			// Store drift results for reuse in semantic JSON output
			driftResults := make([]*drift.Result, len(manifests))

			for i, cm := range manifests {
				// Derive outputDir from the manifest path.
				outputDir := dir
				if paths[i] != dir {
					outputDir = filepath.Dir(paths[i])
				}

				result, driftErr := drift.Detect(outputDir, cm)
				if driftErr != nil {
					return fmt.Errorf("checking drift for %s: %w", cm.Agent, driftErr)
				}
				driftResults[i] = result

				// Build force-lock lookup if --policy is set.
				var forceLockSet map[string]bool
				if policyOnly {
					forceLockSet = make(map[string]bool)
					for _, fl := range cm.ForceLocked {
						forceLockSet[fl.Key] = true
					}
					totalForceLocked += len(cm.ForceLocked)
				}

				// Filter drift entries if --policy is set.
				modified := result.Modified
				missing := result.Missing
				if policyOnly {
					modified = filterByForceLock(result.Modified, forceLockSet)
					missing = filterByForceLock(result.Missing, forceLockSet)
					policyViolations += len(modified) + len(missing)
				}

				if len(modified) == 0 && len(missing) == 0 {
					continue
				}
				hasDrift = true

				if len(manifests) > 1 {
					fmt.Fprintf(cmd.OutOrStdout(), "%s:\n", result.Agent)
				}
				tag := ""
				if policyOnly {
					tag = "[policy] "
				}
				for _, e := range modified {
					fmt.Fprintf(cmd.OutOrStdout(), "  %smodified  %s\n", tag, e.Path)
				}
				for _, e := range missing {
					fmt.Fprintf(cmd.OutOrStdout(), "  %smissing   %s\n", tag, e.Path)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			// Detect semantic drift if --semantic is set (independent of file drift)
			hasSemanticDrift := false
			if semantic {
				for i, cm := range manifests {
					outputDir := dir
					if paths[i] != dir {
						outputDir = filepath.Dir(paths[i])
					}

					stateReader := func(outputDir string) (*manifest.EffectiveState, error) {
						reader, err := compiler.GetReader(cm.Agent)
						if err != nil {
							return nil, nil
						}
						ir, err := reader.Read(outputDir)
						if err != nil {
							return nil, err
						}
						return compiler.BuildEffectiveStateFromIR(ir), nil
					}
					semDiff, semErr := drift.DetectSemantic(outputDir, cm, stateReader)
					if semErr != nil {
						return fmt.Errorf("detecting semantic drift for %s: %w", cm.Agent, semErr)
					}

					if semDiff != nil && semDiff.HasChanges() {
						hasSemanticDrift = true
						driftResults[i].Semantic = semDiff

						if format != "json" {
							if hasDrift {
								fmt.Fprintln(cmd.OutOrStdout())
							}
							fmt.Fprintln(cmd.OutOrStdout(), "Semantic changes:")
							renderSemanticDiff(cmd, semDiff)
						}
					}
				}
			}

			// Verify manifest signatures if --verify is set.
			hasInvalidSignature := false
			if verify {
				ks := signing.DefaultKeyStore()
				for i, cm := range manifests {
					sigDrift := &signing.SignatureDrift{}

					// Verify manifest signature.
					if cm.Signature != nil {
						sigDrift.ManifestSigned = true
						sigDrift.ManifestKeyID = cm.Signature.KeyID
						sigDrift.ManifestSignedAt = cm.Signature.SignedAt

						result, err := signing.VerifyManifest(cm, ks, nil)
						if err != nil {
							return fmt.Errorf("verifying manifest signature: %w", err)
						}
						if result.Valid {
							sigDrift.ManifestVerified = true
						} else {
							sigDrift.ManifestError = result.Error
							hasInvalidSignature = true
						}
					}

					// Per-layer signature status from manifest.
					for _, layer := range cm.LayerStack {
						layerSig := signing.LayerSigStatus{
							Path:     layer.Path,
							Signed:   layer.LayerSignature != "",
							KeyID:    layer.SignerKeyID,
							Verified: layer.SignatureVerified,
						}
						sigDrift.Layers = append(sigDrift.Layers, layerSig)
					}

					// Store in drift result for JSON output.
					driftResults[i].Signatures = sigDrift

					// Text output.
					if format != "json" {
						fmt.Fprintln(cmd.OutOrStdout(), "Signature verification:")
						if sigDrift.ManifestSigned {
							if sigDrift.ManifestVerified {
								fmt.Fprintf(cmd.OutOrStdout(), "  manifest: valid (key: %s, signed: %s)\n",
									sigDrift.ManifestKeyID, sigDrift.ManifestSignedAt)
							} else {
								fmt.Fprintf(cmd.OutOrStdout(), "  manifest: INVALID (%s)\n", sigDrift.ManifestError)
							}
						} else {
							fmt.Fprintln(cmd.OutOrStdout(), "  manifest: not signed")
						}

						if len(sigDrift.Layers) > 0 {
							fmt.Fprintln(cmd.OutOrStdout(), "  layers:")
							for _, ls := range sigDrift.Layers {
								if ls.Signed {
									fmt.Fprintf(cmd.OutOrStdout(), "    %s: signed by %s\n", ls.Path, ls.KeyID)
								} else {
									fmt.Fprintf(cmd.OutOrStdout(), "    %s: not signed\n", ls.Path)
								}
							}
						}
						fmt.Fprintln(cmd.OutOrStdout())
					}
				}
			}

			if !hasDrift && !hasSemanticDrift && !hasInvalidSignature {
				if policyOnly {
					fmt.Fprintln(cmd.OutOrStdout(), "No policy drift detected.")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No drift detected.")
				}
				return nil
			}

			// Print policy summary if --policy is set.
			if policyOnly && format != "json" {
				fmt.Fprintln(cmd.OutOrStdout(), "Policy summary:")
				fmt.Fprintf(cmd.OutOrStdout(), "  Force-locked files: %d\n", totalForceLocked)
				fmt.Fprintf(cmd.OutOrStdout(), "  Policy violations: %d\n", policyViolations)
			}

			// Output JSON if requested (after all drift fields are populated).
			if format == "json" {
				for _, result := range driftResults {
					data, jsonErr := json.MarshalIndent(result, "", "  ")
					if jsonErr != nil {
						return fmt.Errorf("marshaling drift result: %w", jsonErr)
					}
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
				}
			}

			return fmt.Errorf("drift detected")
		},
	}

	cmd.Flags().BoolVar(&policyOnly, "policy", false, "Check only force-locked files (policy violations)")
	cmd.Flags().BoolVar(&semantic, "semantic", false, "Show semantic changes when drift detected")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&verify, "verify", false, "Include signature verification in drift check")
	return cmd
}

// renderSemanticDiff renders semantic changes in human-readable text format.
func renderSemanticDiff(cmd *cobra.Command, diff *manifest.EffectiveStateDiff) {
	if diff == nil {
		return
	}

	// Permissions
	if diff.Permissions != nil {
		fmt.Fprintln(cmd.OutOrStdout(), "  permissions:")
		for _, perm := range diff.Permissions.AllowAdded {
			fmt.Fprintf(cmd.OutOrStdout(), "    + %s (added to allow)\n", perm)
		}
		for _, perm := range diff.Permissions.AllowRemoved {
			fmt.Fprintf(cmd.OutOrStdout(), "    - %s (removed from allow)\n", perm)
		}
		for _, perm := range diff.Permissions.DenyAdded {
			fmt.Fprintf(cmd.OutOrStdout(), "    + %s (added to deny)\n", perm)
		}
		for _, perm := range diff.Permissions.DenyRemoved {
			fmt.Fprintf(cmd.OutOrStdout(), "    - %s (removed from deny)\n", perm)
		}
	}

	// MCP Servers
	if diff.MCPServers != nil {
		fmt.Fprintln(cmd.OutOrStdout(), "  mcp_servers:")
		for _, srv := range diff.MCPServers.Added {
			fmt.Fprintf(cmd.OutOrStdout(), "    + %s (added)\n", srv)
		}
		for _, srv := range diff.MCPServers.Removed {
			fmt.Fprintf(cmd.OutOrStdout(), "    - %s (removed)\n", srv)
		}
	}

	// Hooks
	if diff.Hooks != nil {
		fmt.Fprintln(cmd.OutOrStdout(), "  hooks:")
		for _, hook := range diff.Hooks.Added {
			fmt.Fprintf(cmd.OutOrStdout(), "    + %s (added)\n", hook)
		}
		for _, hook := range diff.Hooks.Removed {
			fmt.Fprintf(cmd.OutOrStdout(), "    - %s (removed)\n", hook)
		}
	}

	// Settings
	if len(diff.Settings) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  settings:")
		for _, change := range diff.Settings {
			if change.From == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "    + %s: %v (added)\n", change.Key, change.To)
			} else if change.To == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "    - %s: %v (removed)\n", change.Key, change.From)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "    ~ %s: %v → %v\n", change.Key, change.From, change.To)
			}
		}
	}

	// Instructions
	if diff.Instructions != nil {
		fmt.Fprintln(cmd.OutOrStdout(), "  instructions:")
		if diff.Instructions.TokenDelta != 0 {
			sign := "+"
			if diff.Instructions.TokenDelta < 0 {
				sign = ""
			}
			fmt.Fprintf(cmd.OutOrStdout(), "    %s%d tokens\n", sign, diff.Instructions.TokenDelta)
		}
		if diff.Instructions.ByteDelta != 0 {
			sign := "+"
			if diff.Instructions.ByteDelta < 0 {
				sign = ""
			}
			fmt.Fprintf(cmd.OutOrStdout(), "    %s%d bytes\n", sign, diff.Instructions.ByteDelta)
		}
	}

	// Categories
	if len(diff.Categories) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  categories:")
		for _, cat := range diff.Categories {
			sign := "+"
			if cat.CountDelta < 0 {
				sign = ""
			}
			fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s%d files\n", cat.Name, sign, cat.CountDelta)
		}
	}
}

// filterByForceLock returns only drift entries whose paths are force-locked.
// Checks both full path and basename since force-lock keys can be either
// (e.g., "CLAUDE.md" vs "deploy.md" for ".claude/commands/deploy.md").
func filterByForceLock(entries []drift.Record, forceLockSet map[string]bool) []drift.Record {
	var filtered []drift.Record
	for _, e := range entries {
		if forceLockSet[e.Path] || forceLockSet[filepath.Base(e.Path)] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
