package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "constitution.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValid(t *testing.T) {
	path := write(t, `
schemaVersion: 1
agentInstructions:
  targets: [claude, agents]
consent:
  policy: strict
sourceTracking:
  type: generic
  pattern: '^FS-[0-9]+$'
phase: sealed
categories:
  - architecture
  - code-style
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Phase != PhaseSealed {
		t.Errorf("Phase = %q, want %q", cfg.Phase, PhaseSealed)
	}
	if len(cfg.Categories) != 2 || cfg.Categories[0] != "architecture" || cfg.Categories[1] != "code-style" {
		t.Errorf("Categories = %v, want [architecture code-style] in order", cfg.Categories)
	}
	if cfg.Consent.Policy != ConsentStrict {
		t.Errorf("Consent.Policy = %q, want %q", cfg.Consent.Policy, ConsentStrict)
	}
	if cfg.SourceTracking.Type != SourceTrackingGeneric {
		t.Errorf("SourceTracking.Type = %q, want %q", cfg.SourceTracking.Type, SourceTrackingGeneric)
	}
}

func TestLoadDefaults(t *testing.T) {
	path := write(t, "schemaVersion: 1\nphase: draft\ncategories: [architecture]\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Phase != PhaseDraft {
		t.Errorf("Phase = %q, want %q", cfg.Phase, PhaseDraft)
	}
	if cfg.Consent.Policy != ConsentStrict {
		t.Errorf("default Consent.Policy = %q, want %q", cfg.Consent.Policy, ConsentStrict)
	}
	if cfg.SourceTracking.Type != SourceTrackingNone {
		t.Errorf("default SourceTracking.Type = %q, want %q", cfg.SourceTracking.Type, SourceTrackingNone)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yml"))
	if err == nil {
		t.Fatal("Load() error = nil, want an error for a missing file")
	}
}

func TestLoadBadYAML(t *testing.T) {
	path := write(t, "schemaVersion: [1\n")
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want a YAML parse error")
	}
	if !strings.Contains(err.Error(), "not valid YAML") {
		t.Errorf("Load() error = %q, want it to mention \"not valid YAML\"", err.Error())
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "unsupported schemaVersion",
			content: "schemaVersion: 2\ncategories: [architecture]\n",
			wantErr: `unsupported schemaVersion 2 (this build only supports schemaVersion 1); refusing to run against an unrecognized config schema`,
		},
		{
			name:    "no categories",
			content: "schemaVersion: 1\nphase: sealed\ncategories: []\n",
			wantErr: `field "categories": at least one category is required`,
		},
		{
			name:    "duplicate category",
			content: "schemaVersion: 1\nphase: sealed\ncategories: [architecture, architecture]\n",
			wantErr: `field "categories": duplicate category "architecture"`,
		},
		{
			name:    "missing phase",
			content: "schemaVersion: 1\ncategories: [architecture]\n",
			wantErr: `field "phase": required — add 'phase: sealed' for an existing (append-only) log, or 'phase: draft' for an unsealed one`,
		},
		{
			name:    "bad phase value",
			content: "schemaVersion: 1\nphase: open\ncategories: [architecture]\n",
			wantErr: `field "phase": must be "draft" or "sealed" (got "open")`,
		},
		{
			name:    "bad consent policy",
			content: "schemaVersion: 1\nphase: sealed\ncategories: [architecture]\nconsent:\n  policy: advisory\n",
			wantErr: `field "consent.policy": must be "strict" or "off" (got "advisory")`,
		},
		{
			name:    "bad sourceTracking type",
			content: "schemaVersion: 1\nphase: sealed\ncategories: [architecture]\nsourceTracking:\n  type: trello\n",
			wantErr: `field "sourceTracking.type": must be one of "none", "generic", "github-issue", "jira" (got "trello")`,
		},
		{
			name:    "unknown agentInstructions target",
			content: "schemaVersion: 1\nphase: sealed\ncategories: [architecture]\nagentInstructions:\n  targets: [claude, claude-md]\n",
			wantErr: `field "agentInstructions.targets": unknown target "claude-md" (allowed: "claude", "agents")`,
		},
		{
			name:    "unknown skills tree",
			content: "schemaVersion: 1\nphase: sealed\ncategories: [architecture]\nskills:\n  trees: [claude, vscode]\n",
			wantErr: `field "skills.trees": unknown skills tree "vscode" (allowed: "claude", "agents", "cursor")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := write(t, tt.content)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("Load() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestValidateRejectsIllegalValue proves the exported in-memory Validate
// (added for `constitution config set`, which mutates a *Config it hasn't
// written yet) enforces the same rules as Load: an illegal enum value is
// rejected, naming the legal values, exactly like Load's own error text.
func TestValidateRejectsIllegalValue(t *testing.T) {
	cfg := &Config{
		SchemaVersion:  SchemaVersion,
		Phase:          PhaseSealed,
		Categories:     []string{"architecture"},
		SourceTracking: SourceTracking{Type: "github"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want the illegal sourceTracking.type rejected")
	}
	want := `field "sourceTracking.type": must be one of "none", "generic", "github-issue", "jira" (got "github")`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Validate() error = %q, want it to contain %q", err.Error(), want)
	}
}

// TestValidateAcceptsLegalValue is TestValidateRejectsIllegalValue's
// control: an otherwise-identical, legal Config passes, and Validate's
// defaulting side effect (empty consent.policy/sourceTracking.type ->
// their defaults) still applies in memory, matching Load's behavior.
func TestValidateAcceptsLegalValue(t *testing.T) {
	cfg := &Config{
		SchemaVersion:  SchemaVersion,
		Phase:          PhaseSealed,
		Categories:     []string{"architecture"},
		SourceTracking: SourceTracking{Type: SourceTrackingGitHubIssue},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if cfg.Consent.Policy != ConsentStrict {
		t.Errorf("Validate() left Consent.Policy = %q, want the default %q applied", cfg.Consent.Policy, ConsentStrict)
	}
}
