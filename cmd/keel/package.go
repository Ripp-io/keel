package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/archive"
)

func newPackageCmd() *cobra.Command {
	var (
		from   string
		format string
		to     string
	)

	cmd := &cobra.Command{
		Use:   "package",
		Short: "Create an archive from a directory",
		Long: `Create a tar.gz or zip archive from a directory.

The archive contains files with paths relative to the source directory.
Symlinks are skipped.

Examples:
  keel package --from /keel/output/claude
  keel package --from /keel/output/claude --format zip
  keel package --from /keel/output/claude --to /tmp/claude-config.tar.gz`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --from exists and is a directory.
			info, err := os.Stat(from)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%q does not exist", from)
				}
				return fmt.Errorf("stat %q: %w", from, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("%q is not a directory", from)
			}

			// Validate format.
			if err := validateArchiveFormat(format); err != nil {
				return err
			}

			// Compute output path.
			archivePath := to
			if archivePath == "" {
				archivePath = from + formatToExtension(format)
			}

			// Create archive.
			if err := createArchive(from, archivePath, format); err != nil {
				return fmt.Errorf("creating archive: %w", err)
			}

			slog.Info("packaged", "from", from, "archive", archivePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Directory to package (required)")
	cmd.Flags().StringVar(&format, "format", "tar.gz", "Archive format: tar.gz, tgz, or zip")
	cmd.Flags().StringVar(&to, "to", "", "Output archive path (default: <from>.<extension>)")

	_ = cmd.MarkFlagRequired("from")

	return cmd
}

// formatToExtension returns the file extension for an archive format.
func formatToExtension(format string) string {
	switch strings.ToLower(format) {
	case "tar.gz", "tgz":
		return ".tar.gz"
	case "zip":
		return ".zip"
	default:
		return "." + format
	}
}

// createArchive creates an archive of srcDir at destPath using the specified format.
func createArchive(srcDir, destPath, format string) error {
	switch strings.ToLower(format) {
	case "tar.gz", "tgz":
		return archive.CreateTarGz(srcDir, destPath)
	case "zip":
		return archive.CreateZip(srcDir, destPath)
	default:
		return fmt.Errorf("unsupported package format %q: use tar.gz or zip", format)
	}
}
