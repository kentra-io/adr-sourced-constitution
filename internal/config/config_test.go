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
  targets: [claude-md, agents-md]
consent:
  policy: strict
sourceTracking:
  type: generic
  pattern: '^FS-[0-9]+$'
categories:
  - architecture
  - code-style
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
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
	path := write(t, "schemaVersion: 1\ncategories: [architecture]\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
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
			content: "schemaVersion: 1\ncategories: []\n",
			wantErr: `field "categories": at least one category is required`,
		},
		{
			name:    "duplicate category",
			content: "schemaVersion: 1\ncategories: [architecture, architecture]\n",
			wantErr: `field "categories": duplicate category "architecture"`,
		},
		{
			name:    "bad consent policy",
			content: "schemaVersion: 1\ncategories: [architecture]\nconsent:\n  policy: advisory\n",
			wantErr: `field "consent.policy": must be "strict" or "off" (got "advisory")`,
		},
		{
			name:    "bad sourceTracking type",
			content: "schemaVersion: 1\ncategories: [architecture]\nsourceTracking:\n  type: trello\n",
			wantErr: `field "sourceTracking.type": must be one of "none", "generic", "github-issue", "jira" (got "trello")`,
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
