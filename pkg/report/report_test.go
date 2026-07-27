package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
	"gopkg.in/yaml.v3"
)

func TestGenerate_FullyPopulated(t *testing.T) {
	manifest := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Name:        "test-project",
		Scope:       "project",
		LayerStack: []manifest.LayerRecord{
			{
				Path:    "/path/to/layer-a",
				Name:    "layer-a",
				Source:  "local",
				Type:    "keel",
				Format:  "keel",
				Version: "1.0",
				Hash:    "abc123def456",
			},
			{
				Path:   "/path/to/layer-b",
				Name:   "layer-b",
				Source: "local",
				Type:   "keel",
				Format: "keel",
				Hash:   "xyz789uvw012",
			},
		},
		Governance: &manifest.GovernanceRecord{
			PermissionsPolicy: &manifest.PermissionsPolicyRecord{
				Merge: "additive",
				SetBy: "layer-a",
			},
			MCPPolicy: &schema.MCPPolicy{
				EnableAll: new(bool),
			},
			HookLockStrategy: "settings-locked",
			OverrideConflicts: []manifest.OverrideConflict{
				{
					Category:    "settings",
					Key:         "settings",
					AttemptedBy: "layer-b",
					LockedBy:    "layer-a",
				},
			},
		},
		ForceLocked: []manifest.ForceLockRecord{
			{
				Category: "settings",
				Key:      "settings.json",
				LockedBy: "layer-a",
			},
		},
		FileOps: []manifest.FileRecord{
			{
				Output:   "settings.json",
				Strategy: "deep-merge",
				Sources: []manifest.FileSource{
					{Layer: "layer-a", File: "settings.json"},
					{Layer: "layer-b", File: "settings.json"},
				},
			},
		},
		InstalledDeps: []manifest.InstalledDepRecord{
			{
				Name:   "tool",
				Scope:  "global",
				Method: "install",
				Path:   "/usr/local/bin/tool",
			},
		},
	}
	*manifest.Governance.MCPPolicy.EnableAll = true

	report := Generate(manifest)

	// Verify schema version
	if report.SchemaVersion != schemaVersion {
		t.Errorf("expected schema version %s, got %s", schemaVersion, report.SchemaVersion)
	}

	// Verify compilation metadata
	if report.Compilation.Agent != "claude" {
		t.Errorf("expected agent claude, got %s", report.Compilation.Agent)
	}
	if report.Compilation.Name != "test-project" {
		t.Errorf("expected name test-project, got %s", report.Compilation.Name)
	}

	// Verify layer BOM
	if len(report.LayerBOM) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(report.LayerBOM))
	}
	if report.LayerBOM[0].Name != "layer-a" {
		t.Errorf("expected layer-a, got %s", report.LayerBOM[0].Name)
	}

	// Verify governance
	if report.Governance == nil {
		t.Fatal("expected governance to be non-nil")
	}
	if report.Governance.PermissionsPolicy.Merge != "additive" {
		t.Errorf("expected additive, got %s", report.Governance.PermissionsPolicy.Merge)
	}
	if len(report.Governance.OverrideConflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(report.Governance.OverrideConflicts))
	}

	// Verify force locks
	if len(report.ForceLocks) != 1 {
		t.Errorf("expected 1 force lock, got %d", len(report.ForceLocks))
	}

	// Verify file ops
	if len(report.FileOps) != 1 {
		t.Errorf("expected 1 file op, got %d", len(report.FileOps))
	}
	if len(report.FileOps[0].Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(report.FileOps[0].Sources))
	}

	// Verify deps
	if len(report.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(report.Dependencies))
	}
}

func TestGenerate_Minimal(t *testing.T) {
	mf := &manifest.CompiledManifest{
		Agent:       "cursor",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "user",
	}

	report := Generate(mf)

	if report.Compilation.Agent != "cursor" {
		t.Errorf("expected agent cursor, got %s", report.Compilation.Agent)
	}
	if report.Governance != nil {
		t.Error("expected governance to be nil for minimal manifest")
	}
	if len(report.LayerBOM) != 0 {
		t.Errorf("expected 0 layers, got %d", len(report.LayerBOM))
	}
}

func TestGenerate_JSONRoundtrip(t *testing.T) {
	mf := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "project",
		LayerStack: []manifest.LayerRecord{
			{
				Path:   "/layer",
				Name:   "test",
				Hash:   "abc123",
				Type:   "keel",
				Format: "keel",
			},
		},
	}

	report := Generate(mf)

	// Marshal to JSON
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Unmarshal back
	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// Verify key fields
	if decoded.Compilation.Agent != "claude" {
		t.Errorf("expected claude, got %s", decoded.Compilation.Agent)
	}
	if len(decoded.LayerBOM) != 1 {
		t.Errorf("expected 1 layer, got %d", len(decoded.LayerBOM))
	}
}

func TestGenerate_YAMLMarshal(t *testing.T) {
	mf := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "project",
	}

	report := Generate(mf)

	data, err := yaml.Marshal(report)
	if err != nil {
		t.Fatalf("yaml marshal error: %v", err)
	}

	// Verify it contains expected keys
	str := string(data)
	if !strings.Contains(str, "schema_version:") {
		t.Error("expected schema_version in YAML output")
	}
	if !strings.Contains(str, "compilation:") {
		t.Error("expected compilation in YAML output")
	}
}

func TestRenderText(t *testing.T) {
	mf := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Name:        "test-project",
		Scope:       "project",
		LayerStack: []manifest.LayerRecord{
			{
				Path:   "/layer-a",
				Name:   "layer-a",
				Type:   "keel",
				Format: "keel",
				Hash:   "abc123def456",
			},
		},
	}

	report := Generate(mf)

	var buf bytes.Buffer
	if err := RenderText(&buf, report); err != nil {
		t.Fatalf("render error: %v", err)
	}

	output := buf.String()

	// Verify key sections
	if !strings.Contains(output, "Keel Compliance Report") {
		t.Error("expected report title")
	}
	if !strings.Contains(output, "## Compilation") {
		t.Error("expected compilation section")
	}
	if !strings.Contains(output, "## Layer Bill of Materials") {
		t.Error("expected layer BOM section")
	}
	if !strings.Contains(output, "claude") {
		t.Error("expected agent name")
	}
	if !strings.Contains(output, "layer-a") {
		t.Error("expected layer name")
	}
}

func TestRenderText_WithGovernance(t *testing.T) {
	manifest := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "project",
		Governance: &manifest.GovernanceRecord{
			PermissionsPolicy: &manifest.PermissionsPolicyRecord{
				Merge: "additive",
				SetBy: "base-layer",
			},
			MCPPolicy: &schema.MCPPolicy{
				EnableAll: new(bool),
			},
			OverrideConflicts: []manifest.OverrideConflict{
				{
					Category:    "settings",
					Key:         "model",
					AttemptedBy: "override-layer",
					LockedBy:    "base-layer",
				},
			},
		},
	}
	*manifest.Governance.MCPPolicy.EnableAll = true

	report := Generate(manifest)

	var buf bytes.Buffer
	if err := RenderText(&buf, report); err != nil {
		t.Fatalf("render error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "## Governance") {
		t.Error("expected governance section")
	}
	if !strings.Contains(output, "additive") {
		t.Error("expected permissions policy")
	}
	if !strings.Contains(output, "Enable All Servers") {
		t.Error("expected MCP enable all")
	}
	if !strings.Contains(output, "Override Conflicts") {
		t.Error("expected conflicts section")
	}
}

func TestRenderText_FullyPopulated(t *testing.T) {
	manifest := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Name:        "full-project",
		Scope:       "project",
		LayerStack: []manifest.LayerRecord{
			{
				Path:    "/layer-a",
				Name:    "layer-a",
				Type:    "keel",
				Format:  "keel",
				Version: "1.0",
				Hash:    "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz",
			},
			{
				Path:   "/layer-b",
				Name:   "layer-b",
				Type:   "keel",
				Format: "keel",
				Hash:   "zzz999yyy888xxx777www666vvv555uuu444ttt333sss222rrr",
			},
		},
		Governance: &manifest.GovernanceRecord{
			PermissionsPolicy: &manifest.PermissionsPolicyRecord{
				Merge: "additive",
				SetBy: "layer-a",
			},
			MCPPolicy: &schema.MCPPolicy{
				EnableAll: new(bool),
				Allowed:   []string{"server-a", "server-b"},
				Denied:    []string{"server-c"},
			},
			HookLockStrategy: "settings-locked",
			OverrideConflicts: []manifest.OverrideConflict{
				{
					Category:    "settings",
					Key:         "model",
					AttemptedBy: "layer-b",
					LockedBy:    "layer-a",
				},
			},
		},
		ForceLocked: []manifest.ForceLockRecord{
			{
				Category: "settings",
				Key:      "settings.json",
				LockedBy: "layer-a",
			},
			{
				Category: "commands",
				Key:      "deploy.md",
				LockedBy: "layer-a",
			},
		},
		FileOps: []manifest.FileRecord{
			{
				Output:   "settings.json",
				Strategy: "deep-merge",
				Sources: []manifest.FileSource{
					{Layer: "layer-a", File: "settings.json"},
				},
			},
			{
				Output:   "CLAUDE.md",
				Strategy: "user-editable",
				Sources: []manifest.FileSource{
					{Layer: "layer-a", File: "instructions.md"},
					{Layer: "layer-b", File: "instructions.md"},
				},
			},
		},
		InstalledDeps: []manifest.InstalledDepRecord{
			{
				Name:   "mytool",
				Scope:  "global",
				Method: "install",
				Path:   "/usr/local/bin/mytool",
			},
		},
		EffectiveState: &manifest.EffectiveState{
			Permissions: &manifest.EffectivePermissions{
				Allow: []string{"Read", "Write"},
				Deny:  []string{"Execute"},
			},
			MCPServers: []manifest.EffectiveMCPServer{
				{Name: "server-a", Type: "stdio"},
				{Name: "server-b", Type: "sse"},
			},
			Hooks: []manifest.EffectiveHook{
				{Event: "SubagentStart", Handlers: 2},
				{Event: "MessageSent", Handlers: 1},
			},
			Settings: map[string]any{
				"model":       "claude-sonnet-4",
				"effortLevel": "high",
				"maxFileSize": 10000,
			},
			Instructions: &manifest.EffectiveInstructions{
				TokenEstimate: 5000,
				Bytes:         20000,
			},
			Categories: []manifest.EffectiveCategory{
				{Name: "commands", FileCount: 3},
				{Name: "skills", FileCount: 5},
			},
		},
	}
	*manifest.Governance.MCPPolicy.EnableAll = true

	report := Generate(manifest)

	var buf bytes.Buffer
	if err := RenderText(&buf, report); err != nil {
		t.Fatalf("render error: %v", err)
	}

	output := buf.String()

	// Verify compilation metadata
	if !strings.Contains(output, "## Compilation") {
		t.Error("expected compilation section")
	}
	if !strings.Contains(output, "full-project") {
		t.Error("expected project name")
	}
	if !strings.Contains(output, "claude") {
		t.Error("expected agent name")
	}

	// Verify layer BOM
	if !strings.Contains(output, "## Layer Bill of Materials (2 layers)") {
		t.Error("expected layer BOM section with count")
	}
	if !strings.Contains(output, "layer-a") {
		t.Error("expected layer-a")
	}
	if !strings.Contains(output, "layer-b") {
		t.Error("expected layer-b")
	}

	// Verify governance section
	if !strings.Contains(output, "## Governance") {
		t.Error("expected governance section")
	}
	if !strings.Contains(output, "Permissions Policy") {
		t.Error("expected permissions policy")
	}
	if !strings.Contains(output, "additive") {
		t.Error("expected additive merge strategy")
	}
	if !strings.Contains(output, "MCP Policy") {
		t.Error("expected MCP policy")
	}
	if !strings.Contains(output, "Enable All Servers") {
		t.Error("expected enable all servers")
	}
	if !strings.Contains(output, "Hook Lock Strategy") {
		t.Error("expected hook lock strategy")
	}
	if !strings.Contains(output, "Override Conflicts (1)") {
		t.Error("expected override conflicts section")
	}

	// Verify force locks
	if !strings.Contains(output, "## Force Locks (2)") {
		t.Error("expected force locks section with count")
	}
	if !strings.Contains(output, "settings.json") {
		t.Error("expected settings.json in force locks")
	}
	if !strings.Contains(output, "deploy.md") {
		t.Error("expected deploy.md in force locks")
	}

	// Verify file operations
	if !strings.Contains(output, "## File Operations (2 files)") {
		t.Error("expected file operations section with count")
	}
	if !strings.Contains(output, "deep-merge") {
		t.Error("expected deep-merge strategy")
	}
	if !strings.Contains(output, "user-editable") {
		t.Error("expected user-editable strategy")
	}

	// Verify dependencies
	if !strings.Contains(output, "## Dependencies (1)") {
		t.Error("expected dependencies section")
	}
	if !strings.Contains(output, "mytool") {
		t.Error("expected mytool dependency")
	}

	// Verify effective state
	if !strings.Contains(output, "## Effective State") {
		t.Error("expected effective state section")
	}

	// Permissions
	if !strings.Contains(output, "Permissions:") {
		t.Error("expected permissions in effective state")
	}
	if !strings.Contains(output, "Read, Write") {
		t.Error("expected allowed permissions")
	}
	if !strings.Contains(output, "Execute") {
		t.Error("expected denied permissions")
	}

	// MCP servers
	if !strings.Contains(output, "MCP Servers (2)") {
		t.Error("expected MCP servers section")
	}
	if !strings.Contains(output, "server-a") {
		t.Error("expected server-a")
	}
	if !strings.Contains(output, "stdio") {
		t.Error("expected stdio type")
	}

	// Hooks
	if !strings.Contains(output, "Hooks (2)") {
		t.Error("expected hooks section")
	}
	if !strings.Contains(output, "SubagentStart") {
		t.Error("expected SubagentStart hook")
	}
	if !strings.Contains(output, "2 handlers") {
		t.Error("expected handler count")
	}

	// Settings
	if !strings.Contains(output, "Settings:") {
		t.Error("expected settings section")
	}
	if !strings.Contains(output, "model") {
		t.Error("expected model setting")
	}
	if !strings.Contains(output, "effortLevel") {
		t.Error("expected effortLevel setting")
	}

	// Instructions
	if !strings.Contains(output, "Instructions:") {
		t.Error("expected instructions section")
	}
	if !strings.Contains(output, "5000 tokens") {
		t.Error("expected token count")
	}
	if !strings.Contains(output, "20000 bytes") {
		t.Error("expected byte count")
	}

	// Categories
	if !strings.Contains(output, "commands") {
		t.Error("expected commands category")
	}
	if !strings.Contains(output, "skills") {
		t.Error("expected skills category")
	}
}

func TestRenderText_Minimal(t *testing.T) {
	mf := &manifest.CompiledManifest{
		Agent:       "cursor",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "user",
		LayerStack:  []manifest.LayerRecord{}, // empty
	}

	report := Generate(mf)

	var buf bytes.Buffer
	if err := RenderText(&buf, report); err != nil {
		t.Fatalf("render error: %v", err)
	}

	output := buf.String()

	// Verify header
	if !strings.Contains(output, "Keel Compliance Report") {
		t.Error("expected report title")
	}

	// Verify compilation section
	if !strings.Contains(output, "## Compilation") {
		t.Error("expected compilation section")
	}
	if !strings.Contains(output, "cursor") {
		t.Error("expected agent name")
	}
	if !strings.Contains(output, "user") {
		t.Error("expected scope")
	}

	// Verify empty layer BOM
	if !strings.Contains(output, "## Layer Bill of Materials (0 layers)") {
		t.Error("expected empty layer BOM section")
	}

	// Verify no governance section (it should be omitted when nil)
	if strings.Contains(output, "## Governance") {
		t.Error("governance section should be omitted when nil")
	}

	// Verify no force locks section
	if strings.Contains(output, "## Force Locks") {
		t.Error("force locks section should be omitted when empty")
	}

	// Verify file ops section exists but with count 0
	if !strings.Contains(output, "## File Operations (0 files)") {
		t.Error("expected file operations section with 0 count")
	}

	// Verify no dependencies section
	if strings.Contains(output, "## Dependencies") {
		t.Error("dependencies section should be omitted when empty")
	}

	// Verify no effective state section
	if strings.Contains(output, "## Effective State") {
		t.Error("effective state section should be omitted when nil")
	}
}

func TestGenerate_WithSignatures(t *testing.T) {
	m := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-06-18T00:00:00Z",
		Scope:       "project",
		Signature: &manifest.ManifestSignature{
			Algorithm:   "ed25519",
			KeyID:       "keel:sha256:abc123",
			Signature:   "base64sig",
			SignedAt:    "2026-06-18T00:00:00Z",
			ContentHash: "sha256:xyz789",
		},
		LayerStack: []manifest.LayerRecord{
			{
				Path:              "/path/to/layer-a",
				Name:              "layer-a",
				Type:              "keel",
				Format:            "keel",
				Hash:              "abc123",
				LayerSignature:    "layersig1",
				SignerKeyID:       "keel:sha256:def456",
				SignatureVerified: true,
			},
			{
				Path:   "/path/to/layer-b",
				Name:   "layer-b",
				Type:   "keel",
				Format: "keel",
				Hash:   "def456",
			},
		},
	}

	report := Generate(m)

	if report.Signatures == nil {
		t.Fatal("expected signatures section to be present")
	}

	if !report.Signatures.ManifestSigned {
		t.Error("expected manifest to be marked as signed")
	}

	if report.Signatures.SignerKeyID != "keel:sha256:abc123" {
		t.Errorf("expected key ID abc123, got %s", report.Signatures.SignerKeyID)
	}

	if len(report.Signatures.LayerSignatures) != 2 {
		t.Fatalf("expected 2 layer signatures, got %d", len(report.Signatures.LayerSignatures))
	}

	if !report.Signatures.LayerSignatures[0].Signed {
		t.Error("expected layer-a to be marked as signed")
	}

	if !report.Signatures.LayerSignatures[0].Verified {
		t.Error("expected layer-a to be marked as verified")
	}

	if report.Signatures.LayerSignatures[1].Signed {
		t.Error("expected layer-b to not be marked as signed")
	}
}

func TestGenerate_WithoutSignatures(t *testing.T) {
	m := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-06-18T00:00:00Z",
		Scope:       "project",
		LayerStack:  []manifest.LayerRecord{},
	}

	report := Generate(m)

	if report.Signatures != nil {
		t.Error("expected signatures section to be nil when no signatures present")
	}
}
