package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/report"
	"gopkg.in/yaml.v3"
)

func newReportCmd() *cobra.Command {
	var format string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "report [dir]",
		Short: "Generate compliance report from compiled manifest",
		Long: `Generate a structured compliance report from the .keel.json manifest.

The report captures governance decisions, layer provenance (BOM), force-locks,
file operations, and dependencies in a structured format suitable for auditing
and compliance workflows.

Auto-discovers .keel.json in agent subdirectories (.claude, .cursor).

Examples:
  keel report                          # JSON output to stdout
  keel report --format yaml            # YAML output
  keel report --format text            # Human-readable text
  keel report -o report.json           # Write to file
  keel report .claude --format text    # Specific directory`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			// Load compiled manifest
			manifest, err := loadCompiledConfig(dir)
			if err != nil {
				return err
			}

			// Generate report
			r := report.Generate(manifest)

			// Render output
			var output []byte
			switch format {
			case "json":
				output, err = json.MarshalIndent(r, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling JSON: %w", err)
				}
				output = append(output, '\n')
			case "yaml":
				output, err = yaml.Marshal(r)
				if err != nil {
					return fmt.Errorf("marshaling YAML: %w", err)
				}
			case "text":
				// Write directly for text format
				if outputPath != "" {
					out, err := os.Create(outputPath)
					if err != nil {
						return fmt.Errorf("creating output file: %w", err)
					}
					defer func() { _ = out.Close() }()
					if err := report.RenderText(out, r); err != nil {
						return err
					}
					if err := out.Close(); err != nil {
						return fmt.Errorf("closing output file: %w", err)
					}
					return nil
				}
				return report.RenderText(cmd.OutOrStdout(), r)
			default:
				return fmt.Errorf("unsupported format: %s (use json, yaml, or text)", format)
			}

			// Write output
			if outputPath != "" {
				if err := os.WriteFile(outputPath, output, 0644); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
			} else {
				if _, err := cmd.OutOrStdout().Write(output); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml, or text")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Write output to file instead of stdout")

	return cmd
}
