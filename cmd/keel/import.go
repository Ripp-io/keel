package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/importer"
)

func newImportCmd() *cobra.Command {
	var (
		from  string
		agent string
		to    string
		name  string
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import native agent config into a Keel DSL layer",
		Long: `Read a native AI agent configuration directory and produce a keel.yml
layer. Supports Claude Code (.claude/ + CLAUDE.md) and Cursor (.cursor/)
native formats.

The source agent format is auto-detected from marker files unless --agent
is specified. The output is a self-contained Keel DSL layer directory with
a keel.yml and all referenced companion files.

Examples:
  keel import --to ./my-layer
  keel import --from ./project --to ./my-layer
  keel import --from ./project --agent claude --to ./my-layer`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImport(cmd.OutOrStdout(), from, agent, to, name)
		},
	}

	cmd.Flags().StringVar(&from, "from", ".", "Source directory containing native agent config")
	cmd.Flags().StringVar(&agent, "agent", "", "Source agent format (auto-detected if omitted)")
	cmd.Flags().StringVar(&to, "to", "", "Output directory for generated keel.yml layer")
	cmd.Flags().StringVar(&name, "name", "", "Layer name (default: source directory basename)")

	_ = cmd.MarkFlagRequired("to")

	return cmd
}

func runImport(w io.Writer, from, agent, to, name string) error {
	// Resolve source directory to absolute path
	absFrom, err := filepath.Abs(from)
	if err != nil {
		return fmt.Errorf("resolving source directory: %w", err)
	}

	info, err := os.Stat(absFrom)
	if err != nil {
		return fmt.Errorf("source directory %q: %w", absFrom, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source path %q is not a directory", absFrom)
	}

	// Detect or validate agent format
	if agent == "" {
		detected, _ := config.DetectLayerFormat(absFrom)
		if detected == config.FormatKeel {
			return fmt.Errorf("could not detect native agent format in %q (use --agent to specify)", absFrom)
		}
		agent = string(detected)
		fmt.Fprintf(w, "Detected agent format: %s\n", agent)
	}

	// Look up reader
	reader, err := compiler.GetReader(agent)
	if err != nil {
		return fmt.Errorf("no reader for agent %q: %w", agent, err)
	}

	// Read native config into IR
	ir, err := reader.Read(absFrom)
	if err != nil {
		return fmt.Errorf("reading %s config from %q: %w", agent, absFrom, err)
	}

	// Set layer name
	if name != "" {
		ir.Name = name
	} else if ir.Name == "" {
		ir.Name = filepath.Base(absFrom)
	}

	// Resolve output directory
	absTo, err := filepath.Abs(to)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}

	// Serialize IR to keel.yml layer
	if err := importer.SerializeIR(ir, absTo); err != nil {
		return fmt.Errorf("writing layer: %w", err)
	}

	// Print summary
	fmt.Fprintf(w, "\nImported %s config from %s\n", agent, absFrom)
	fmt.Fprintf(w, "Output: %s/keel.yml\n", absTo)
	printImportSummary(w, ir)

	return nil
}

// printImportSummary writes a brief summary of what was imported.
func printImportSummary(w io.Writer, ir *compiler.LayerIR) {
	fmt.Fprintln(w)
	if len(ir.Instructions) > 0 {
		fmt.Fprintf(w, "  instructions:  %d file(s)\n", len(ir.Instructions))
	}
	if len(ir.Permissions) > 0 {
		fmt.Fprintln(w, "  permissions:   yes")
	}
	if len(ir.MCP) > 0 {
		fmt.Fprintln(w, "  mcp:           yes")
	}
	if len(ir.Hooks) > 0 {
		fmt.Fprintf(w, "  hooks:         %d event(s)\n", len(ir.Hooks))
	}
	if len(ir.Settings) > 0 {
		fmt.Fprintf(w, "  settings:      %d key(s)\n", len(ir.Settings))
	}
	for agent, data := range ir.AgentSpecific {
		if len(data) > 0 {
			fmt.Fprintf(w, "  %s-specific: %d key(s)\n", agent, len(data))
		}
	}
	for cat, files := range ir.Categories {
		count := 0
		for _, f := range files {
			if !f.IsDir {
				count++
			}
		}
		if count > 0 {
			fmt.Fprintf(w, "  %-14s %d file(s)\n", cat+":", count)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next: review the generated keel.yml, then compile with 'keel compile'")
}
