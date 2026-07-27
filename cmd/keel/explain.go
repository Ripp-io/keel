package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func newExplainCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "explain [dir] [section]",
		Short: "Show human-readable compilation breakdown",
		Long: `Read the compiled .keel.json and present a human-readable summary of what
was compiled, where each piece came from, and what's locked.

Without arguments, uses the current directory and prints all sections.
With a section name (files, locks, layers, name, repos, repo, init-dir, deps, signatures), prints only that section.

Auto-discovers .keel.json in agent subdirectories (.claude, .cursor).

Use --format json to output structured JSON instead of human-readable text.
For JSON output without a section, the full .keel.json manifest is returned.

Examples:
  keel explain
  keel explain files
  keel explain ./output
  keel explain .claude
  keel explain ./output files
  keel explain --format json
  keel explain --format json deps
  keel explain repo gateway --format json`,
		Args:      cobra.MaximumNArgs(3),
		ValidArgs: []string{"files", "locks", "layers", "deps", "name", "repos", "repo", "init-dir", "signatures"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			section := ""
			repoName := ""

			switch len(args) {
			case 3:
				// repo <name> <dir>
				if args[0] == "repo" {
					section = args[0]
					repoName = args[1]
					dir = args[2]
				} else {
					return fmt.Errorf("invalid arguments: expected 'repo <name> [dir]'")
				}
			case 2:
				// Either: <dir> <section> OR repo <name>
				if args[0] == "repo" {
					section = args[0]
					repoName = args[1]
				} else if isExplainSection(args[0]) {
					section = args[0]
					dir = args[1]
				} else {
					dir = args[0]
					section = args[1]
				}
			case 1:
				if isExplainSection(args[0]) {
					section = args[0]
				} else {
					dir = args[0]
				}
			}

			cfg, err := loadCompiledConfig(dir)
			if err != nil {
				return err
			}

			if format == "json" {
				return renderExplainJSON(cmd, cfg, section, repoName)
			}
			return renderExplain(cmd, cfg, section, repoName)
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")

	return cmd
}

func isExplainSection(s string) bool {
	return s == "files" || s == "locks" || s == "layers" || s == "deps" ||
		s == "name" || s == "repos" || s == "repo" || s == "init-dir" || s == "signatures"
}

func renderExplain(cmd *cobra.Command, cfg *manifest.CompiledManifest, section, repoName string) error {
	w := cmd.OutOrStdout()

	switch section {
	case "":
		renderHeader(cmd, cfg)
		fmt.Fprintln(w)
		if err := renderLayers(cmd, cfg); err != nil {
			return err
		}
		fmt.Fprintln(w)
		if err := renderFiles(cmd, cfg); err != nil {
			return err
		}
		fmt.Fprintln(w)
		if err := renderLocks(cmd, cfg); err != nil {
			return err
		}
		if len(cfg.Repos) > 0 {
			fmt.Fprintln(w)
			if err := renderRepos(cmd, cfg); err != nil {
				return err
			}
		}
		if len(cfg.InstalledDeps) > 0 {
			fmt.Fprintln(w)
			if err := renderDeps(cmd, cfg); err != nil {
				return err
			}
		}
		if cfg.Signature != nil || len(cfg.LayerStack) > 0 {
			fmt.Fprintln(w)
			if err := renderSignatures(cmd, cfg); err != nil {
				return err
			}
		}
	case "files":
		return renderFiles(cmd, cfg)
	case "locks":
		return renderLocks(cmd, cfg)
	case "layers":
		return renderLayers(cmd, cfg)
	case "deps":
		return renderDeps(cmd, cfg)
	case "name":
		fmt.Fprintln(w, cfg.Name)
	case "repos":
		if len(cfg.Repos) == 0 {
			fmt.Fprintln(w, "(no repos defined)")
		} else {
			return renderRepos(cmd, cfg)
		}
	case "repo":
		repo, ok := cfg.Repos[repoName]
		if !ok {
			return fmt.Errorf("repo %q not found", repoName)
		}
		return writeJSON(w, repo)
	case "init-dir":
		if cfg.InitDir == "" {
			fmt.Fprintln(w, "(no init.d directory)")
		} else {
			fmt.Fprintln(w, cfg.InitDir)
		}
	case "signatures":
		return renderSignatures(cmd, cfg)
	default:
		return fmt.Errorf("unknown section %q (valid: files, locks, layers, deps, name, repos, init-dir, signatures)", section)
	}

	return nil
}

func renderHeader(cmd *cobra.Command, cfg *manifest.CompiledManifest) {
	w := cmd.OutOrStdout()

	agent := cfg.Agent
	if agent == "" {
		agent = "unknown"
	}
	compiledAt := cfg.CompiledAt
	if compiledAt == "" {
		compiledAt = "unknown"
	}
	keelVersion := cfg.KeelVersion
	if keelVersion == "" {
		keelVersion = "unknown"
	}

	fmt.Fprintf(w, "Compiled: %s · %s · keel %s\n", agent, compiledAt, keelVersion)
}

func renderLayers(cmd *cobra.Command, cfg *manifest.CompiledManifest) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Layers (%d):\n", len(cfg.LayerStack))
	if len(cfg.LayerStack) == 0 {
		fmt.Fprintln(w, "  (none)")
		return nil
	}

	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	for i, layer := range cfg.LayerStack {
		version := layer.Version
		if version == "" {
			version = "-"
		}
		fmt.Fprintf(tw, "  %d.\t%s\t%s %s\n", i+1, layer.Path, layer.Format, version)
	}
	return tw.Flush()
}

func renderFiles(cmd *cobra.Command, cfg *manifest.CompiledManifest) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Files (%d):\n", len(cfg.FileOps))
	if len(cfg.FileOps) == 0 {
		fmt.Fprintln(w, "  (none)")
		return nil
	}

	// Build a set of locked output files for the [locked] annotation.
	lockedFiles := make(map[string]bool)
	for _, fl := range cfg.ForceLocked {
		if fl.Category == "file" {
			lockedFiles[fl.Key] = true
		}
	}

	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	for _, f := range cfg.FileOps {
		layers := make([]string, 0, len(f.Sources))
		for _, s := range f.Sources {
			layers = append(layers, s.Layer)
		}

		suffix := ""
		if lockedFiles[f.Output] {
			suffix = " [locked]"
		}

		// Check if any source layer is force-locked for overlay files.
		if suffix == "" {
			for _, s := range f.Sources {
				base := filepath.Base(s.File)
				for _, fl := range cfg.ForceLocked {
					if fl.Key == base {
						suffix = " [locked]"
						break
					}
				}
				if suffix != "" {
					break
				}
			}
		}

		fmt.Fprintf(tw, "  %s\t%s\t<- %s%s\n", f.Output, f.Strategy, strings.Join(layers, ", "), suffix)
	}
	return tw.Flush()
}

func renderLocks(cmd *cobra.Command, cfg *manifest.CompiledManifest) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Force-locks (%d):\n", len(cfg.ForceLocked))
	if len(cfg.ForceLocked) == 0 {
		fmt.Fprintln(w, "  (none)")
		return nil
	}

	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	for _, fl := range cfg.ForceLocked {
		fmt.Fprintf(tw, "  %s\t%s\tlocked by %s\n", fl.Category, fl.Key, fl.LockedBy)
	}
	return tw.Flush()
}

func renderRepos(cmd *cobra.Command, cfg *manifest.CompiledManifest) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Repos (%d):\n", len(cfg.Repos))

	names := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		names = append(names, name)
	}
	sort.Strings(names)

	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	for _, name := range names {
		repo := cfg.Repos[name]
		branch := repo.Branch
		if branch == "" {
			branch = "-"
		}

		required := "optional"
		if repo.Required != nil && *repo.Required {
			required = "required"
		}

		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", name, repo.URL, branch, required)
	}
	return tw.Flush()
}

func renderDeps(cmd *cobra.Command, cfg *manifest.CompiledManifest) error {
	w := cmd.OutOrStdout()

	if len(cfg.InstalledDeps) == 0 {
		fmt.Fprintln(w, "No dependencies defined.")
		return nil
	}

	fmt.Fprintf(w, "Dependencies (%d):\n", len(cfg.InstalledDeps))

	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  NAME\tSCOPE\tMETHOD\tPATH\n")
	for _, dep := range cfg.InstalledDeps {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", dep.Name, dep.Scope, dep.Method, dep.Path)
	}
	return tw.Flush()
}

func renderSignatures(cmd *cobra.Command, cfg *manifest.CompiledManifest) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Signatures:\n")

	// Manifest signature
	if cfg.Signature != nil {
		fmt.Fprintf(w, "  Manifest: signed by %s at %s\n", cfg.Signature.KeyID, cfg.Signature.SignedAt)
	} else {
		fmt.Fprintln(w, "  Manifest: not signed")
	}

	// Layer signatures
	if len(cfg.LayerStack) > 0 {
		fmt.Fprintln(w, "  Layers:")
		tw := tabwriter.NewWriter(w, 4, 0, 2, ' ', 0)
		for _, layer := range cfg.LayerStack {
			if layer.LayerSignature != "" {
				status := "signed"
				if layer.SignatureVerified {
					status = "signed (verified)"
				}
				fmt.Fprintf(tw, "    %s\t%s by %s\n", layer.Path, status, layer.SignerKeyID)
			} else {
				fmt.Fprintf(tw, "    %s\tnot signed\n", layer.Path)
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	return nil
}

func renderExplainJSON(cmd *cobra.Command, cfg *manifest.CompiledManifest, section, repoName string) error {
	w := cmd.OutOrStdout()

	switch section {
	case "":
		// Full manifest
		return writeJSON(w, cfg)
	case "name":
		fmt.Fprintln(w, cfg.Name)
		return nil
	case "repos":
		if len(cfg.Repos) == 0 {
			fmt.Fprintln(w, "{}")
			return nil
		}
		return writeJSON(w, cfg.Repos)
	case "repo":
		repo, ok := cfg.Repos[repoName]
		if !ok {
			return fmt.Errorf("repo %q not found", repoName)
		}
		return writeJSON(w, repo)
	case "init-dir":
		if cfg.InitDir == "" {
			fmt.Fprintln(w, "")
			return nil
		}
		fmt.Fprintln(w, cfg.InitDir)
		return nil
	case "layers":
		return writeJSON(w, cfg.LayerStack)
	case "deps":
		if len(cfg.InstalledDeps) == 0 {
			fmt.Fprintln(w, "[]")
			return nil
		}
		return writeJSON(w, cfg.InstalledDeps)
	case "files":
		return writeJSON(w, cfg.FileOps)
	case "locks":
		return writeJSON(w, cfg.ForceLocked)
	case "signatures":
		// Build a structure for signatures output
		sig := map[string]any{
			"manifest": map[string]any{
				"signed": cfg.Signature != nil,
			},
		}
		if cfg.Signature != nil {
			sig["manifest"].(map[string]any)["key_id"] = cfg.Signature.KeyID
			sig["manifest"].(map[string]any)["signed_at"] = cfg.Signature.SignedAt
		}
		layers := []map[string]any{}
		for _, layer := range cfg.LayerStack {
			layerSig := map[string]any{
				"path":   layer.Path,
				"signed": layer.LayerSignature != "",
			}
			if layer.LayerSignature != "" {
				layerSig["key_id"] = layer.SignerKeyID
				layerSig["verified"] = layer.SignatureVerified
			}
			layers = append(layers, layerSig)
		}
		sig["layers"] = layers
		return writeJSON(w, sig)
	default:
		return fmt.Errorf("unknown section %q (valid: files, locks, layers, deps, name, repos, repo, init-dir, signatures)", section)
	}
}
