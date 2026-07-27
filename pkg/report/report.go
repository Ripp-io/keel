package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

const schemaVersion = "1.0"

// Report is the Keel compliance report schema.
type Report struct {
	SchemaVersion  string                     `json:"schema_version" yaml:"schema_version"`
	GeneratedAt    string                     `json:"generated_at"   yaml:"generated_at"`
	Compilation    CompilationMetadata        `json:"compilation"    yaml:"compilation"`
	LayerBOM       []LayerEntry               `json:"layer_bom"      yaml:"layer_bom"`
	Governance     *GovernanceSection         `json:"governance,omitempty" yaml:"governance,omitempty"`
	ForceLocks     []manifest.ForceLockRecord `json:"force_locks,omitempty" yaml:"force_locks,omitempty"`
	FileOps        []FileOpEntry              `json:"file_ops"       yaml:"file_ops"`
	Dependencies   []DependencyEntry          `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	EffectiveState *manifest.EffectiveState   `json:"effective_state,omitempty" yaml:"effective_state,omitempty"`
	Signatures     *SignatureSection          `json:"signatures,omitempty" yaml:"signatures,omitempty"`
}

// CompilationMetadata captures metadata about the compilation.
type CompilationMetadata struct {
	Agent       string `json:"agent"       yaml:"agent"`
	KeelVersion string `json:"keel_version" yaml:"keel_version"`
	CompiledAt  string `json:"compiled_at" yaml:"compiled_at"`
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Scope       string `json:"scope"       yaml:"scope"`
}

// LayerEntry represents a layer in the Bill of Materials.
type LayerEntry struct {
	Index   int    `json:"index"   yaml:"index"`
	Path    string `json:"path"    yaml:"path"`
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Source  string `json:"source"  yaml:"source"`
	Type    string `json:"type"    yaml:"type"`
	Format  string `json:"format"  yaml:"format"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Hash    string `json:"hash"    yaml:"hash"`
}

// GovernanceSection captures governance decisions from the manifest.
type GovernanceSection struct {
	PermissionsPolicy *manifest.PermissionsPolicyRecord `json:"permissions_policy,omitempty" yaml:"permissions_policy,omitempty"`
	MCPPolicy         *schema.MCPPolicy                 `json:"mcp_policy,omitempty"         yaml:"mcp_policy,omitempty"`
	HookLockStrategy  string                            `json:"hook_lock_strategy,omitempty" yaml:"hook_lock_strategy,omitempty"`
	OverrideConflicts []manifest.OverrideConflict       `json:"override_conflicts,omitempty" yaml:"override_conflicts,omitempty"`
}

// FileOpEntry represents a file operation.
type FileOpEntry struct {
	Output   string   `json:"output"   yaml:"output"`
	Strategy string   `json:"strategy" yaml:"strategy"`
	Sources  []string `json:"sources,omitempty" yaml:"sources,omitempty"`
}

// DependencyEntry represents an installed dependency.
type DependencyEntry struct {
	Name   string `json:"name"   yaml:"name"`
	Scope  string `json:"scope"  yaml:"scope"`
	Method string `json:"method" yaml:"method"`
	Path   string `json:"path"   yaml:"path"`
}

// SignatureSection captures signature status for compliance reporting.
type SignatureSection struct {
	ManifestSigned  bool            `json:"manifest_signed" yaml:"manifest_signed"`
	SignerKeyID     string          `json:"signer_key_id,omitempty" yaml:"signer_key_id,omitempty"`
	SignedAt        string          `json:"signed_at,omitempty" yaml:"signed_at,omitempty"`
	LayerSignatures []LayerSigEntry `json:"layer_signatures,omitempty" yaml:"layer_signatures,omitempty"`
}

// LayerSigEntry represents a layer's signature status in the report.
type LayerSigEntry struct {
	Path     string `json:"path" yaml:"path"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Signed   bool   `json:"signed" yaml:"signed"`
	KeyID    string `json:"key_id,omitempty" yaml:"key_id,omitempty"`
	Verified bool   `json:"verified,omitempty" yaml:"verified,omitempty"`
}

// Generate creates a compliance report from the compiled manifest.
func Generate(m *manifest.CompiledManifest) *Report {
	report := &Report{
		SchemaVersion: schemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Compilation: CompilationMetadata{
			Agent:       m.Agent,
			KeelVersion: m.KeelVersion,
			CompiledAt:  m.CompiledAt,
			Name:        m.Name,
			Scope:       m.Scope,
		},
	}

	// Layer BOM
	for i, layer := range m.LayerStack {
		report.LayerBOM = append(report.LayerBOM, LayerEntry{
			Index:   i,
			Path:    layer.Path,
			Name:    layer.Name,
			Source:  layer.Source,
			Type:    layer.Type,
			Format:  layer.Format,
			Version: layer.Version,
			Hash:    layer.Hash,
		})
	}

	// Governance
	if m.Governance != nil {
		gov := &GovernanceSection{
			PermissionsPolicy: m.Governance.PermissionsPolicy,
			MCPPolicy:         m.Governance.MCPPolicy,
			HookLockStrategy:  m.Governance.HookLockStrategy,
			OverrideConflicts: m.Governance.OverrideConflicts,
		}
		report.Governance = gov
	}

	// Force locks
	report.ForceLocks = m.ForceLocked

	// File operations
	for _, file := range m.FileOps {
		var sources []string
		for _, src := range file.Sources {
			sources = append(sources, src.Layer)
		}
		report.FileOps = append(report.FileOps, FileOpEntry{
			Output:   file.Output,
			Strategy: file.Strategy,
			Sources:  sources,
		})
	}

	// Dependencies
	for _, dep := range m.InstalledDeps {
		report.Dependencies = append(report.Dependencies, DependencyEntry{
			Name:   dep.Name,
			Scope:  dep.Scope,
			Method: dep.Method,
			Path:   dep.Path,
		})
	}

	// Effective state
	report.EffectiveState = m.EffectiveState

	// Build signature section.
	sigSection := &SignatureSection{}
	if m.Signature != nil {
		sigSection.ManifestSigned = true
		sigSection.SignerKeyID = m.Signature.KeyID
		sigSection.SignedAt = m.Signature.SignedAt
	}
	for _, layer := range m.LayerStack {
		entry := LayerSigEntry{
			Path:   layer.Path,
			Name:   layer.Name,
			Signed: layer.LayerSignature != "",
		}
		if layer.LayerSignature != "" {
			entry.KeyID = layer.SignerKeyID
			entry.Verified = layer.SignatureVerified
		}
		sigSection.LayerSignatures = append(sigSection.LayerSignatures, entry)
	}
	// Only include if there's signing activity.
	if sigSection.ManifestSigned || len(sigSection.LayerSignatures) > 0 {
		report.Signatures = sigSection
	}

	return report
}

// RenderText writes a human-readable report to the provided writer.
func RenderText(w io.Writer, r *Report) (err error) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() {
		if ferr := tw.Flush(); ferr != nil && err == nil {
			err = fmt.Errorf("flushing final output: %w", ferr)
		}
	}()

	// Header
	fmt.Fprintf(tw, "Keel Compliance Report\n")
	fmt.Fprintf(tw, "Generated: %s\n\n", r.GeneratedAt)

	// Compilation metadata
	fmt.Fprintf(tw, "## Compilation\n\n")
	fmt.Fprintf(tw, "Agent:\t%s\n", r.Compilation.Agent)
	fmt.Fprintf(tw, "Keel Version:\t%s\n", r.Compilation.KeelVersion)
	fmt.Fprintf(tw, "Compiled:\t%s\n", r.Compilation.CompiledAt)
	if r.Compilation.Name != "" {
		fmt.Fprintf(tw, "Name:\t%s\n", r.Compilation.Name)
	}
	fmt.Fprintf(tw, "Scope:\t%s\n", r.Compilation.Scope)
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing compilation section: %w", err)
	}

	// Layer BOM
	fmt.Fprintf(w, "\n## Layer Bill of Materials (%d layers)\n\n", len(r.LayerBOM))
	for _, layer := range r.LayerBOM {
		fmt.Fprintf(tw, "%d.\t%s\n", layer.Index, layer.Path)
		if layer.Name != "" {
			fmt.Fprintf(tw, "\tName:\t%s\n", layer.Name)
		}
		fmt.Fprintf(tw, "\tType:\t%s\n", layer.Type)
		fmt.Fprintf(tw, "\tFormat:\t%s\n", layer.Format)
		if layer.Version != "" {
			fmt.Fprintf(tw, "\tVersion:\t%s\n", layer.Version)
		}
		hashDisplay := layer.Hash
		if len(hashDisplay) > 12 {
			hashDisplay = hashDisplay[:12] + "..."
		}
		fmt.Fprintf(tw, "\tHash:\t%s\n", hashDisplay)
		if err := tw.Flush(); err != nil {
			return fmt.Errorf("flushing layer BOM: %w", err)
		}
		fmt.Fprintln(w)
	}

	// Governance
	if r.Governance != nil {
		fmt.Fprintf(w, "## Governance\n\n")
		if r.Governance.PermissionsPolicy != nil {
			fmt.Fprintf(tw, "Permissions Policy:\t%s (set by %s)\n", r.Governance.PermissionsPolicy.Merge, r.Governance.PermissionsPolicy.SetBy)
		}
		if r.Governance.MCPPolicy != nil {
			if r.Governance.MCPPolicy.EnableAll != nil && *r.Governance.MCPPolicy.EnableAll {
				fmt.Fprintf(tw, "MCP Policy:\tEnable All Servers\n")
			}
			if len(r.Governance.MCPPolicy.Allowed) > 0 {
				fmt.Fprintf(tw, "MCP Allowed:\t%s\n", strings.Join(r.Governance.MCPPolicy.Allowed, ", "))
			}
			if len(r.Governance.MCPPolicy.Denied) > 0 {
				fmt.Fprintf(tw, "MCP Denied:\t%s\n", strings.Join(r.Governance.MCPPolicy.Denied, ", "))
			}
		}
		if r.Governance.HookLockStrategy != "" {
			fmt.Fprintf(tw, "Hook Lock Strategy:\t%s\n", r.Governance.HookLockStrategy)
		}
		if err := tw.Flush(); err != nil {
			return fmt.Errorf("flushing governance section: %w", err)
		}

		if len(r.Governance.OverrideConflicts) > 0 {
			fmt.Fprintf(w, "\n### Override Conflicts (%d)\n\n", len(r.Governance.OverrideConflicts))
			for _, conflict := range r.Governance.OverrideConflicts {
				fmt.Fprintf(tw, "%s/%s:\tAttempted by %s, locked by %s\n",
					conflict.Category, conflict.Key, conflict.AttemptedBy, conflict.LockedBy)
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing override conflicts: %w", err)
			}
		}
		fmt.Fprintln(w)
	}

	// Force locks
	if len(r.ForceLocks) > 0 {
		fmt.Fprintf(w, "## Force Locks (%d)\n\n", len(r.ForceLocks))
		// Group by category
		byCategory := make(map[string][]manifest.ForceLockRecord)
		for _, lock := range r.ForceLocks {
			byCategory[lock.Category] = append(byCategory[lock.Category], lock)
		}
		categories := make([]string, 0, len(byCategory))
		for cat := range byCategory {
			categories = append(categories, cat)
		}
		sort.Strings(categories)

		for _, cat := range categories {
			locks := byCategory[cat]
			fmt.Fprintf(tw, "%s:\n", cat)
			for _, lock := range locks {
				fmt.Fprintf(tw, "\t%s\t(locked by %s)\n", lock.Key, lock.LockedBy)
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing force locks: %w", err)
			}
		}
		fmt.Fprintln(w)
	}

	// File operations summary
	fmt.Fprintf(w, "## File Operations (%d files)\n\n", len(r.FileOps))
	// Group by strategy
	byStrategy := make(map[string]int)
	for _, op := range r.FileOps {
		byStrategy[op.Strategy]++
	}
	strategies := make([]string, 0, len(byStrategy))
	for strat := range byStrategy {
		strategies = append(strategies, strat)
	}
	sort.Strings(strategies)
	for _, strat := range strategies {
		fmt.Fprintf(tw, "%s:\t%d\n", strat, byStrategy[strat])
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing file operations summary: %w", err)
	}
	fmt.Fprintln(w)

	// Dependencies
	if len(r.Dependencies) > 0 {
		fmt.Fprintf(w, "## Dependencies (%d)\n\n", len(r.Dependencies))
		for _, dep := range r.Dependencies {
			fmt.Fprintf(tw, "%s:\n", dep.Name)
			fmt.Fprintf(tw, "\tScope:\t%s\n", dep.Scope)
			fmt.Fprintf(tw, "\tMethod:\t%s\n", dep.Method)
			fmt.Fprintf(tw, "\tPath:\t%s\n", dep.Path)
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing dependencies: %w", err)
			}
			fmt.Fprintln(w)
		}
	}

	// Effective State
	if r.EffectiveState != nil {
		fmt.Fprintf(w, "## Effective State\n\n")

		// Permissions
		if r.EffectiveState.Permissions != nil {
			fmt.Fprintf(tw, "Permissions:\n")
			if len(r.EffectiveState.Permissions.Allow) > 0 {
				fmt.Fprintf(tw, "\tAllow:\t%s\n", strings.Join(r.EffectiveState.Permissions.Allow, ", "))
			} else {
				fmt.Fprintf(tw, "\tAllow:\t(none)\n")
			}
			if len(r.EffectiveState.Permissions.Deny) > 0 {
				fmt.Fprintf(tw, "\tDeny:\t%s\n", strings.Join(r.EffectiveState.Permissions.Deny, ", "))
			} else {
				fmt.Fprintf(tw, "\tDeny:\t(none)\n")
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing permissions: %w", err)
			}
			fmt.Fprintln(w)
		}

		// MCP Servers
		if len(r.EffectiveState.MCPServers) > 0 {
			fmt.Fprintf(w, "MCP Servers (%d):\n", len(r.EffectiveState.MCPServers))
			for _, srv := range r.EffectiveState.MCPServers {
				fmt.Fprintf(tw, "\t%s\t%s\n", srv.Name, srv.Type)
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing MCP servers: %w", err)
			}
			fmt.Fprintln(w)
		}

		// Hooks
		if len(r.EffectiveState.Hooks) > 0 {
			fmt.Fprintf(w, "Hooks (%d):\n", len(r.EffectiveState.Hooks))
			for _, hook := range r.EffectiveState.Hooks {
				var handlerText string
				if hook.Handlers == 1 {
					handlerText = "1 handler"
				} else {
					handlerText = fmt.Sprintf("%d handlers", hook.Handlers)
				}
				fmt.Fprintf(tw, "\t%s\t%s\n", hook.Event, handlerText)
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing hooks: %w", err)
			}
			fmt.Fprintln(w)
		}

		// Settings
		if len(r.EffectiveState.Settings) > 0 {
			fmt.Fprintf(w, "Settings:\n")
			// Sort keys for deterministic output
			keys := make([]string, 0, len(r.EffectiveState.Settings))
			for k := range r.EffectiveState.Settings {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(tw, "\t%s\t%v\n", k, r.EffectiveState.Settings[k])
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing settings: %w", err)
			}
			fmt.Fprintln(w)
		}

		// Instructions
		if r.EffectiveState.Instructions != nil {
			fmt.Fprintf(tw, "Instructions:\n")
			fmt.Fprintf(tw, "\t%d tokens (%d bytes)\n",
				r.EffectiveState.Instructions.TokenEstimate,
				r.EffectiveState.Instructions.Bytes)
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing instructions: %w", err)
			}
			fmt.Fprintln(w)
		}

		// Categories
		if len(r.EffectiveState.Categories) > 0 {
			fmt.Fprintf(w, "Categories:\n")
			for _, cat := range r.EffectiveState.Categories {
				var fileText string
				if cat.FileCount == 1 {
					fileText = "1 file"
				} else {
					fileText = fmt.Sprintf("%d files", cat.FileCount)
				}
				fmt.Fprintf(tw, "\t%s\t%s\n", cat.Name, fileText)
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing categories: %w", err)
			}
			fmt.Fprintln(w)
		}
	}

	// Signatures
	if r.Signatures != nil {
		fmt.Fprintf(w, "## Signatures\n\n")
		if r.Signatures.ManifestSigned {
			fmt.Fprintf(tw, "Manifest:\tsigned by %s at %s\n",
				r.Signatures.SignerKeyID, r.Signatures.SignedAt)
		} else {
			fmt.Fprintf(tw, "Manifest:\tnot signed\n")
		}
		if err := tw.Flush(); err != nil {
			return fmt.Errorf("flushing signatures: %w", err)
		}

		if len(r.Signatures.LayerSignatures) > 0 {
			fmt.Fprintf(w, "\nLayers:\n")
			for _, layer := range r.Signatures.LayerSignatures {
				if layer.Signed {
					status := "signed"
					if layer.Verified {
						status = "signed (verified)"
					}
					fmt.Fprintf(tw, "\t%s\t%s by %s\n", layer.Path, status, layer.KeyID)
				} else {
					fmt.Fprintf(tw, "\t%s\tnot signed\n", layer.Path)
				}
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flushing layer signatures: %w", err)
			}
		}
		fmt.Fprintln(w)
	}

	return nil
}
