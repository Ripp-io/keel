package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/agentplugins"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/agentsmd"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/aider"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/amp"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/antigravity"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/claude"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cline"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/codex"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/copilot"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cursor"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/goose"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/junie"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/kilo"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/kiro"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/opencode"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/openhands"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/qwen"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/vibe"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/signing"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/snapshot"
)

// compileFlags groups CLI-only flags that control compilation behavior.
// These are execution modes, not compilation parameters (those are in CompileConfig).
type compileFlags struct {
	DryRun   bool
	Summary  bool
	ShowVars bool
	Stats    bool
	Clean    bool
	NoPrune  bool
	Force    bool
	Strict   bool
	Snapshot bool
}

func newCompileCmd() *cobra.Command {
	var (
		cfg        config.CompileConfig
		configFile string
		flags      compileFlags
	)

	cmd := &cobra.Command{
		Use:   "compile",
		Short: "Compile config layers into tool-native output",
		Long: `Compile configuration layers into tool-native output for one or more agents.

Specify targets with --output agent=path (repeatable for multi-agent compilation):
  keel compile --output claude=/keel
  keel compile --output claude=/out/c --output cursor=/out/k

Produces tool-native files (CLAUDE.md, .cursor/rules/, etc.) plus a compiled
.keel.json inside the agent's config directory (e.g. .claude/.keel.json).

Layer resolution order:
  1. --layer paths (repeatable) or CONFIG_LAYERS env var (colon-separated, default: /keel/layers/*)
  2. User config from USER_CONFIG (default: /mnt/user-config)
  3. USER_PRIORITY controls user config position (high=last, low=first)

Remote layers are fetched automatically and cached locally:
  keel compile --layer /local/path --layer 'https://git.example.com/org/policy.git#main:layers/base'

Note: Quote URLs containing '#' to prevent shell interpretation.
Use --offline to compile from cached remote layers without network access.
Use 'keel cache list' to see cached entries and 'keel cache clean' to clear them.

The --layer flag overrides CONFIG_LAYERS.
Use --distributable with --name to preserve _keel policies and frontmatter for downstream layers.
Use --dry-run to preview what compilation would produce without writing to disk.
Use --summary to print the compilation report after writing output (auto-enabled with --debug).
Use --config <path> to load compilation parameters from a YAML file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Auto-discover config file when --config is not set.
			// For --scope user, check ~/.keel/config.yml first.
			userConfigUsed := false
			if configFile == "" {
				if cfg.Scope == "user" {
					if home, err := os.UserHomeDir(); err == nil {
						candidate := filepath.Join(home, ".keel", "config.yml")
						if _, err := os.Stat(candidate); err == nil {
							configFile = candidate
							userConfigUsed = true
						}
					}
				}
				if configFile == "" {
					candidate := filepath.Join(".keel", "config.yml")
					if _, err := os.Stat(candidate); err == nil {
						configFile = candidate
					}
				}
			}

			// If --config was provided (or auto-discovered), load values from the file and merge.
			if configFile != "" {
				fileCfg, err := config.LoadConfigFile(configFile)
				if err != nil {
					return fmt.Errorf("loading config file: %w", err)
				}
				mergeConfigFile(cmd, &cfg, fileCfg)
			}

			// User scope: redirect output to home directory when not explicitly
			// configured. Project config output paths target the project dir —
			// user-scope output belongs in ~, same principle as skipping .keel/local/.
			if cfg.Scope == "user" && !cmd.Flags().Changed("output") && !userConfigUsed {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("determining home directory for user scope: %w", err)
				}
				if len(cfg.Output) > 0 {
					// Project config specified agents — remap paths to home.
					for agent := range cfg.Output {
						cfg.Output[agent] = home
					}
				} else {
					// No output configured — detect at home.
					if detected := config.DetectOutputTargets(home); len(detected) > 0 {
						cfg.Output = detected
					}
				}
			}

			cfg.ApplyDefaults()

			// Validate scope.
			if cfg.Scope != "" {
				s := compiler.Scope(cfg.Scope)
				if s != compiler.ScopeProject && s != compiler.ScopeUser {
					return fmt.Errorf("invalid --scope %q: must be \"project\" or \"user\"", cfg.Scope)
				}
			}

			// Validate package format.
			if cfg.Package != "" {
				if err := validateArchiveFormat(cfg.Package); err != nil {
					return fmt.Errorf("invalid --package %q: %w", cfg.Package, err)
				}
			}

			if len(cfg.Output) == 0 {
				return fmt.Errorf("no output targets: provide --output agent=path, add output to config file, or run from a directory containing .claude/ or .cursor/")
			}

			// Auto-enable summary when debug logging is active.
			if !flags.Summary {
				if v, _ := cmd.Flags().GetCount("debug"); v > 0 {
					flags.Summary = true
				}
			}

			// Build external vars with correct precedence:
			//   config file vars < var-file vars < CLI --var flags
			//
			// After mergeConfigFile, cfg.Var has the winning values at the map
			// level (CLI if --var was set, else config file). We layer var-files
			// underneath cfg.Var so that CLI/config values take precedence.
			vars := make(map[string]string)
			for _, vf := range cfg.VarFile {
				fileVars, err := config.LoadVarFile(vf)
				if err != nil {
					return fmt.Errorf("loading var-file %q: %w", vf, err)
				}
				for k, v := range fileVars {
					vars[k] = v
				}
			}
			// CLI/config vars overlay (higher precedence than var-files).
			for k, v := range cfg.Var {
				vars[k] = v
			}

			return runCompile(cmd.OutOrStdout(), cfg, vars, flags)
		},
	}

	config.RegisterFlags(cmd, &cfg)

	// Suppress unhelpful "(default [])" display for map/slice flags.
	for _, name := range []string{"output", "var"} {
		if f := cmd.Flags().Lookup(name); f != nil {
			f.DefValue = ""
		}
	}

	// The --config flag is meta (tells WHERE to load config, not WHAT the config is).
	cmd.Flags().StringVar(&configFile, "config", "", "Path to YAML config file for compilation parameters")

	// CLI-only flags (execution modes, not compilation parameters).
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false, "Preview compilation output without writing to disk")
	cmd.Flags().BoolVar(&flags.Summary, "summary", false, "Print compilation summary after writing output (auto-enabled with --debug)")
	cmd.Flags().BoolVar(&flags.ShowVars, "show-vars", false, "Print resolved variable map after compilation")
	cmd.Flags().BoolVar(&flags.Stats, "stats", false, "Print compiled content metrics (token estimates, counts, sizes)")
	cmd.Flags().BoolVar(&flags.Clean, "clean", false, "Remove all previously compiled output before recompiling")
	cmd.Flags().BoolVar(&flags.NoPrune, "no-prune", false, "Skip automatic removal of stale files from previous compilations")
	cmd.Flags().BoolVar(&flags.Force, "force", false, "Overwrite user-edited instruction files (CLAUDE.md, etc.) without warning")
	cmd.Flags().BoolVar(&flags.Strict, "strict", false, "Exit with error when instruction files conflict (for CI)")
	cmd.Flags().BoolVar(&flags.Snapshot, "snapshot", false, "Snapshot current state before compiling (for rollback)")

	return cmd
}

// mergeConfigFile copies values from the file config into cfg for any field
// whose corresponding CLI flag was NOT explicitly set by the user.
// This implements the precedence: CLI flags > config file.
//
// It uses reflection to iterate over CompileConfig fields by their "flag" tag,
// so adding a new field to CompileConfig requires zero changes here.
//
// NOTE: This function handles string, bool, and map field types via reflection,
// matching the types in CompileConfig. Adding a new field type requires
// updating both RegisterFlags and this function.
func mergeConfigFile(cmd *cobra.Command, cfg *config.CompileConfig, fileCfg *config.CompileConfig) {
	cfgVal := reflect.ValueOf(cfg).Elem()
	fileVal := reflect.ValueOf(fileCfg).Elem()
	t := cfgVal.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		flagName := field.Tag.Get("flag")

		// Fields with a flag tag: only merge if the CLI flag was not explicitly set.
		if flagName != "" {
			if cmd.Flags().Changed(flagName) {
				continue
			}
		} else if field.Tag.Get("yaml") == "-" || field.Tag.Get("yaml") == "" {
			// No flag, no yaml — skip (e.g. unexported or transient fields).
			continue
		}

		fileField := fileVal.Field(i)
		if fileField.Kind() == reflect.Map || fileField.Kind() == reflect.Slice {
			if fileField.IsNil() || fileField.Len() == 0 {
				continue
			}
		} else if fileField.IsZero() {
			continue
		}

		cfgVal.Field(i).Set(fileField)
	}
}

func runCompile(w io.Writer, cfg config.CompileConfig, vars map[string]string, f compileFlags) error {
	layers, err := config.ResolveLayers(cfg.NormalizeLayers(), cfg.UserConfig, cfg.UserPriority, cfg.Offline, cfg.MaxRemoteLayers)
	if err != nil {
		return fmt.Errorf("resolving layers: %w", err)
	}

	if len(layers) == 0 {
		slog.Warn("no config layers found")
		if f.Strict {
			return fmt.Errorf("--strict: no config layers found")
		}
	}

	// Default scope to "project" when not set.
	scope := compiler.ScopeProject
	if cfg.Scope != "" {
		scope = compiler.Scope(cfg.Scope)
	}

	opts := compiler.CompileOptions{
		Distributable:     cfg.Distributable,
		Name:              cfg.Name,
		DryRun:            f.DryRun,
		Scope:             scope,
		Offline:           cfg.Offline,
		NoPrune:           f.NoPrune,
		AllowInstall:      cfg.AllowInstall,
		ForceInstructions: f.Force,
		Vars:              vars,
	}

	// Dry-run still uses per-agent loop (each needs a temp dir)
	if f.DryRun {
		for agent := range cfg.Output {
			slog.Debug("compiling (dry-run)", "agent", agent, "layers", len(layers))
			for i, l := range layers {
				slog.Debug("layer", "index", i+1, "path", l.Path, "type", l.Type, "format", l.Format)
			}
			if err := compileDryRun(agent, layers, opts, w); err != nil {
				return err
			}
		}
		return nil
	}

	// --clean and --locked: pre-flight checks for all agents
	for agent, outputDir := range cfg.Output {
		agentMeta := config.LookupAgent(agent)
		agentDir := ""
		if agentMeta != nil && len(agentMeta.Dirs) > 0 {
			agentDir = agentMeta.Dirs[0]
		}

		// --clean: remove all previously compiled output before recompiling.
		if f.Clean {
			cleaned, cleanErr := manifest.CleanAllOutput(outputDir, agentDir, f.Force)
			if cleanErr != nil {
				return fmt.Errorf("cleaning output for %s: %w", agent, cleanErr)
			}
			if cleaned != nil && len(cleaned.Removed) > 0 {
				slog.Info("cleaned previous output", "agent", agent, "removed", len(cleaned.Removed))
			}
		}

		// Verify layer hashes before compilation when --locked is set.
		if cfg.Locked {
			keelStateDir := filepath.Join(outputDir, agentDir)
			if err := compiler.VerifyKeelState(keelStateDir, layers); err != nil {
				return fmt.Errorf("state verification failed: %w", err)
			}
		}
	}

	// --verify-layers: verify layer signatures before compilation.
	// Skips the implicit .keel/local layer (user-local overrides, never signed).
	if cfg.VerifyLayers {
		ks := signing.DefaultKeyStore()
		for _, layer := range layers {
			if strings.HasSuffix(filepath.ToSlash(layer.Path), ".keel/local") {
				slog.Debug("skipping implicit local layer for signature verification", "layer", layer.Path)
				continue
			}
			result, err := signing.VerifyLayerSignature(layer.Path, ks, nil)
			if err != nil {
				return fmt.Errorf("verifying layer %s: %w", layer.Path, err)
			}
			if !result.Valid {
				return fmt.Errorf("layer signature verification failed for %s: %s", layer.Path, result.Error)
			}
			slog.Debug("layer signature verified", "layer", layer.Path, "key_id", result.KeyID)
		}
	}

	// --snapshot: capture current state before compilation overwrites it.
	// Runs after --clean so we snapshot the last known state, not mid-clean.
	if f.Snapshot && !f.DryRun {
		for agent, outputDir := range cfg.Output {
			agentMeta := config.LookupAgent(agent)
			if agentMeta == nil || len(agentMeta.Dirs) == 0 {
				continue
			}
			agentDir := filepath.Join(outputDir, agentMeta.Dirs[0])
			if _, loadErr := manifest.LoadKeelState(agentDir); loadErr == nil {
				snap, snapErr := snapshot.Create(outputDir, agentDir)
				if snapErr != nil {
					slog.Warn("failed to create snapshot", "agent", agent, "error", snapErr)
					continue
				}
				slog.Info("snapshot created", "id", snap.ID, "agent", agent)
			}
		}
	}

	// Compile all agents with shared layer cache
	results, err := compiler.CompileMulti(cfg.Output, layers, opts)
	if err != nil {
		return err
	}

	// --sign: sign compiled manifests and augment layer records with signature info.
	if cfg.Sign {
		ks := signing.DefaultKeyStore()
		keyName := cfg.SignKey
		if keyName == "" {
			keyName = "default"
		}
		for agent, result := range results {
			outputDir := cfg.Output[agent]
			agentDir := filepath.Join(outputDir, result.Output.OutputRoot)
			statePath := filepath.Join(agentDir, ".keel.json")

			// Load the manifest that was just written.
			cm, loadErr := manifest.LoadKeelState(agentDir)
			if loadErr != nil {
				return fmt.Errorf("loading manifest for signing (%s): %w", agent, loadErr)
			}
			if cm == nil {
				continue
			}

			// Augment layer records with signature info from keel.sig files.
			for i, lr := range cm.LayerStack {
				layerSig, sigErr := signing.ReadLayerSignature(lr.Path)
				if sigErr != nil {
					slog.Warn("failed to read layer signature", "layer", lr.Path, "error", sigErr)
					continue
				}
				if layerSig != nil {
					cm.LayerStack[i].LayerSignature = layerSig.Signature
					cm.LayerStack[i].SignerKeyID = layerSig.KeyID
					// If we verified layers earlier, mark them as verified.
					cm.LayerStack[i].SignatureVerified = cfg.VerifyLayers
				}
			}

			// Sign the manifest.
			sig, signErr := signing.SignManifest(cm, keyName, ks)
			if signErr != nil {
				return fmt.Errorf("signing manifest for %s: %w", agent, signErr)
			}
			cm.Signature = sig

			// Rewrite the manifest with signature.
			if writeErr := manifest.WriteKeelState(statePath, cm); writeErr != nil {
				return fmt.Errorf("writing signed manifest for %s: %w", agent, writeErr)
			}

			slog.Info("signed manifest", "agent", agent, "key_id", sig.KeyID)
		}
	}

	// Strict mode: fail on instruction conflicts.
	if f.Strict {
		var conflicts []string
		for agent, result := range results {
			for _, c := range result.Governance.InstructionConflicts {
				conflicts = append(conflicts, fmt.Sprintf("%s: %s", agent, c))
			}
		}
		if len(conflicts) > 0 {
			sort.Strings(conflicts)
			return fmt.Errorf("--strict: instruction file conflicts detected (use --force to overwrite):\n  %s",
				strings.Join(conflicts, "\n  "))
		}
	}

	// Post-compilation: summary, stats, and packaging
	for agent, result := range results {
		if f.Summary || f.ShowVars {
			printCompileSummary(w, agent, layers, result, f.ShowVars)
		}

		if f.Stats && result.State.Stats != nil {
			printCompileStats(w, result.State.Stats)
		}

		slog.Debug("compilation complete", "agent", agent, "files", len(result.Output.Files))

		if cfg.Package != "" {
			outputDir := cfg.Output[agent]
			ext := formatToExtension(cfg.Package)
			archivePath := outputDir + ext
			if err := createArchive(outputDir, archivePath, cfg.Package); err != nil {
				return fmt.Errorf("packaging %s output: %w", agent, err)
			}
			slog.Info("packaged", "agent", agent, "archive", archivePath)
		}
	}

	return nil
}

// compileDryRun compiles into a temp directory and prints a summary without persisting output.
func compileDryRun(agent string, layers []config.Layer, opts compiler.CompileOptions, w io.Writer) error {
	tmpDir, err := os.MkdirTemp("", "keel-dry-run-*")
	if err != nil {
		return fmt.Errorf("creating temp directory for dry run: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	result, err := compiler.Compile(agent, layers, tmpDir, opts)
	if err != nil {
		return fmt.Errorf("compiling config for %s (dry run): %w", agent, err)
	}
	printDryRunSummary(w, agent, layers, result)
	return nil
}

// printCompileStats writes content metrics (token estimates, counts, sizes) to w.
func printCompileStats(w io.Writer, stats *compiler.CompileStats) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Compiled content:")

	for _, section := range stats.Sections {
		name := section.Name
		if section.Count > 0 {
			name = fmt.Sprintf("%s (%d)", section.Name, section.Count)
		}
		fmt.Fprintf(w, "    %-25s ~%s tokens   (%s bytes)\n",
			name,
			formatNumber(section.Tokens),
			formatNumber(section.Bytes))
	}

	if stats.MCPServers > 0 {
		fmt.Fprintf(w, "    MCP servers: %d\n", stats.MCPServers)
	}
	if stats.HookEvents > 0 {
		fmt.Fprintf(w, "    hook events: %d\n", stats.HookEvents)
	}

	if stats.TotalTokens > 0 {
		fmt.Fprintf(w, "    %-25s ~%s tokens   (%s bytes)\n",
			"Total",
			formatNumber(stats.TotalTokens),
			formatNumber(stats.TotalBytes))
	}
}

// formatNumber formats an integer with comma separators for readability.
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

// printCompileSummary writes the same report as printDryRunSummary but with a
// header that reflects a real (non-dry-run) compilation.
func printCompileSummary(w io.Writer, agent string, layers []config.Layer, result *compiler.CompileResult, showVars bool) {
	fmt.Fprintf(w, "Compiled %s with %d layers:\n", agent, len(layers))
	printCompileReport(w, layers, result, showVars)
}

// printDryRunSummary writes a human-readable compilation plan to w.
func printDryRunSummary(w io.Writer, agent string, layers []config.Layer, result *compiler.CompileResult) {
	fmt.Fprintf(w, "Dry run — compile for %s with %d layers:\n", agent, len(layers))
	printCompileReport(w, layers, result, false)
	if result.State.Stats != nil {
		printCompileStats(w, result.State.Stats)
	}
}

// printCompileReport writes the shared body of a compilation summary: layers,
// output files with strategies and force-lock tags, and totals.
func printCompileReport(w io.Writer, layers []config.Layer, result *compiler.CompileResult, showVars bool) {
	// Layers section
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Layers:")
	for i, l := range layers {
		version := l.Version
		if version == "" {
			version = "-"
		}
		if l.Source != "" {
			fmt.Fprintf(w, "    %d. %s (%s, %s) <- %s\n", i+1, l.Path, l.Format, version, l.Source)
		} else {
			fmt.Fprintf(w, "    %d. %s (%s, %s)\n", i+1, l.Path, l.Format, version)
		}
	}

	// Output files section
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Output files:")
	if len(result.Output.Files) == 0 {
		fmt.Fprintln(w, "    (none)")
	}
	for _, f := range result.Output.Files {
		sources := formatSources(f.Sources, layers)
		lockTag := ""
		if isOutputForceLocked(f.Output, result.Governance.ForceLocked) {
			lockTag = " [force-locked]"
		}
		fmt.Fprintf(w, "    %-40s <- %s (%s)%s\n", f.Output, f.Strategy, sources, lockTag)
	}

	// Force-locked section
	if len(result.Governance.ForceLocked) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Force-locked:")
		for _, fl := range result.Governance.ForceLocked {
			fmt.Fprintf(w, "    %s/%s by %s\n", fl.Category, fl.Key, fl.LockedBy)
		}
	}

	// Shadowed files section
	shadowed := collectShadowed(result.Output.Overlays)
	if len(shadowed) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Shadowed files:")
		for _, s := range shadowed {
			fmt.Fprintf(w, "    %-40s %s\n", s.path, s.chain)
		}
	}

	// Pruned files section
	if result.State.Pruned != nil && len(result.State.Pruned.Removed) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Pruned (stale from previous compile):")
		for _, f := range result.State.Pruned.Removed {
			fmt.Fprintf(w, "    %s\n", f)
		}
	}

	// Variables section
	if showVars && len(result.Tracking.Variables) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Variables:")
		varNames := make([]string, 0, len(result.Tracking.Variables))
		for k := range result.Tracking.Variables {
			varNames = append(varNames, k)
		}
		sort.Strings(varNames)
		for _, k := range varNames {
			fmt.Fprintf(w, "    %-30s = %s\n", k, result.Tracking.Variables[k])
		}
	}

	// Summary line
	fmt.Fprintln(w)
	shadowCount := len(shadowed)
	pruneCount := 0
	if result.State.Pruned != nil {
		pruneCount = len(result.State.Pruned.Removed)
	}
	parts := []string{
		fmt.Sprintf("%d layers compiled", len(layers)),
		fmt.Sprintf("%d output files", len(result.Output.Files)),
		fmt.Sprintf("%d force-locks", len(result.Governance.ForceLocked)),
	}
	if shadowCount > 0 {
		parts = append(parts, fmt.Sprintf("%d shadowed", shadowCount))
	}
	if pruneCount > 0 {
		parts = append(parts, fmt.Sprintf("%d pruned", pruneCount))
	}
	fmt.Fprintf(w, "  %s\n", strings.Join(parts, ", "))
}

// formatSources returns a human-readable list of source layer names for a file record.
func formatSources(sources []manifest.FileSource, layers []config.Layer) string {
	if len(sources) == 0 {
		return "none"
	}

	// Build a map from layer path to short name (directory basename).
	layerNames := make(map[string]string, len(layers))
	for _, l := range layers {
		layerNames[l.Path] = filepath.Base(l.Path)
	}

	names := make([]string, 0, len(sources))
	for _, s := range sources {
		name := s.Layer
		if short, ok := layerNames[s.Layer]; ok {
			name = short
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// isOutputForceLocked checks whether any force-lock record's key matches the
// output path.
// It uses exact match against the full output path or its base filename to
// avoid false positives from substring matches (e.g. a lock for "deploy.md"
// must not match "pre-deploy.md").
func isOutputForceLocked(output string, locks []manifest.ForceLockRecord) bool {
	base := filepath.Base(output)
	for _, fl := range locks {
		if output == fl.Key || base == fl.Key {
			return true
		}
	}
	return false
}

// shadowEntry represents a file that was written by multiple layers.
type shadowEntry struct {
	path  string
	chain string
}

// collectShadowed returns overlay entries where 2+ layers wrote the same file.
// The chain format is: "01-company → 03-team" (using layer basenames).
// Overlay writers are already stored as layer basenames by recordOverlay.
func collectShadowed(overlays map[string][]string) []shadowEntry {
	if len(overlays) == 0 {
		return nil
	}

	var result []shadowEntry
	// Sort keys for deterministic output
	paths := make([]string, 0, len(overlays))
	for p := range overlays {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		writers := overlays[p]
		if len(writers) < 2 {
			continue
		}
		result = append(result, shadowEntry{
			path:  p,
			chain: strings.Join(writers, " → "),
		})
	}
	return result
}
