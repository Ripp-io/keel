package compiler

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/globaldeps"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// processDeps handles the deps: section of a keel.yml layer.
// It injects per-dep variables into the provided variables map regardless of allowInstall.
// When allowInstall is true, it executes install/download commands.
// Returns InstalledDepRecord entries for the manifest.
func processDeps(
	deps map[string]schema.DepSpec,
	outputDir string,
	allowInstall bool,
	requireChecksums bool,
	variables map[string]string,
	preInstalled map[string]string,
) ([]manifest.InstalledDepRecord, error) {
	if variables == nil {
		return nil, fmt.Errorf("processDeps: variables map must not be nil")
	}

	// Iterate in sorted order for determinism.
	names := make([]string, 0, len(deps))
	for name := range deps {
		names = append(names, name)
	}
	sort.Strings(names)

	var records []manifest.InstalledDepRecord

	for _, name := range names {
		dep := deps[name]
		scope := "project"
		if dep.Global {
			scope = "global"
		}

		// Determine target directory.
		var targetDir string
		if dep.Global {
			targetDir = globaldeps.BinDir()
		} else {
			targetDir = filepath.Join(outputDir, ".keel", "bin")
		}

		// Always inject per-dep variables regardless of allowInstall.
		variables["keel.deps."+name+".path"] = filepath.Join(targetDir, name)
		variables["keel.deps."+name+".dir"] = targetDir

		// Determine which method would be used.
		method := "install"
		source := dep.Install
		if dep.Download != "" {
			method = "download"
			source = dep.Download
		}

		// Already processed by a previous adapter pass — just record it.
		if prevMethod, ok := preInstalled[name]; ok {
			records = append(records, manifest.InstalledDepRecord{
				Name:   name,
				Scope:  scope,
				Method: prevMethod,
				Path:   filepath.Join(targetDir, name),
			})
			continue
		}

		if !allowInstall {
			slog.Warn("dep requires --allow-install to install",
				"dep", name, "method", method, "command", source)
			records = append(records, manifest.InstalledDepRecord{
				Name:   name,
				Scope:  scope,
				Method: "skipped",
				Path:   filepath.Join(targetDir, name),
			})
			continue
		}

		// For global deps with verify set: check if already installed.
		if dep.Global && dep.Verify != "" {
			if verifyInstalled(dep.Verify, targetDir) {
				slog.Info("dep already installed (verify passed)", "dep", name)
				records = append(records, manifest.InstalledDepRecord{
					Name:   name,
					Scope:  scope,
					Method: method,
					Path:   filepath.Join(targetDir, name),
				})
				continue
			}
		}

		// Ensure target directory exists.
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, fmt.Errorf("creating target dir for dep %q: %w", name, err)
		}

		if dep.Install != "" {
			if err := executeInstall(name, dep.Install, targetDir); err != nil {
				return nil, fmt.Errorf("installing dep %q: %w", name, err)
			}
		} else if dep.Download != "" {
			if err := executeDownload(name, dep, targetDir, requireChecksums); err != nil {
				return nil, fmt.Errorf("downloading dep %q: %w", name, err)
			}
		}

		// Post-install verify (if verify is set and install happened).
		if dep.Verify != "" {
			if !verifyInstalled(dep.Verify, targetDir) {
				return nil, fmt.Errorf("dep %q: post-install verify failed (command: %s)", name, dep.Verify)
			}
		}

		// Global consumer registration.
		if dep.Global {
			absOutputDir := outputDir
			if !filepath.IsAbs(absOutputDir) {
				if abs, err := filepath.Abs(absOutputDir); err == nil {
					absOutputDir = abs
				}
			}
			manifestPath := globaldeps.ManifestPath()
			mf, err := globaldeps.LoadManifest(manifestPath)
			if err != nil {
				slog.Warn("failed to load global manifest for consumer registration", "error", err)
			} else {
				mf.RegisterConsumer(name, absOutputDir, method, source)
				if err := globaldeps.SaveManifest(mf, manifestPath); err != nil {
					slog.Warn("failed to save global manifest", "error", err)
				}
			}
		}

		records = append(records, manifest.InstalledDepRecord{
			Name:   name,
			Scope:  scope,
			Method: method,
			Path:   filepath.Join(targetDir, name),
		})
	}

	return records, nil
}
