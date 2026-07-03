package guard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/manifest"
)

func writeManifest(t *testing.T, adrDir string, adrs []adr.ADR) {
	t.Helper()
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, manifest.FileName), manifest.Render(adrs), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sampleADR() adr.ADR {
	return adr.ADR{
		ID: "ADR-0001", Title: "T", Category: "architecture", Date: "2026-07-01",
		Status:       adr.StatusAccepted,
		Sections:     map[string]string{"Decision Outcome": "y"},
		SectionOrder: []string{"Decision Outcome"},
		Path:         filepath.Join("constitution", "adr", "ADR-0001-a.md"),
	}
}

func TestCheckManifest(t *testing.T) {
	t.Run("matching hash is clean", func(t *testing.T) {
		dir := t.TempDir()
		a := sampleADR()
		writeManifest(t, dir, []adr.ADR{a})

		got, err := checkManifest(dir, []adr.ADR{a})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("checkManifest() = %+v, want no violations", got)
		}
	})

	t.Run("changed frozen content mismatches the recorded hash", func(t *testing.T) {
		dir := t.TempDir()
		original := sampleADR()
		writeManifest(t, dir, []adr.ADR{original})

		edited := original
		edited.Sections = map[string]string{"Decision Outcome": "TAMPERED"}

		got, err := checkManifest(dir, []adr.ADR{edited})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Kind != KindManifestMismatch {
			t.Fatalf("checkManifest() = %+v, want exactly one manifest_mismatch", got)
		}
		if got[0].ID != "ADR-0001" {
			t.Errorf("violation.ID = %q, want ADR-0001", got[0].ID)
		}
	})

	t.Run("a legal status-only change does not trip the manifest (status is excluded from the hash)", func(t *testing.T) {
		dir := t.TempDir()
		original := sampleADR()
		writeManifest(t, dir, []adr.ADR{original})

		superseded := original
		superseded.Status = adr.StatusSuperseded
		superseded.SupersededBy = "ADR-0002"

		got, err := checkManifest(dir, []adr.ADR{superseded})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("checkManifest() = %+v, want no violations (status/superseded-by are excluded from the manifest hash)", got)
		}
	})

	t.Run("file recorded in the manifest but missing on disk is file_deleted", func(t *testing.T) {
		dir := t.TempDir()
		original := sampleADR()
		writeManifest(t, dir, []adr.ADR{original})

		got, err := checkManifest(dir, nil) // nothing on disk anymore
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Kind != KindFileDeleted {
			t.Fatalf("checkManifest() = %+v, want exactly one file_deleted", got)
		}
		if got[0].ID != "ADR-0001" {
			t.Errorf("violation.ID = %q, want ADR-0001 (derived from the filename)", got[0].ID)
		}
	})

	t.Run("a file present on disk but absent from the manifest is a manifest_mismatch (reverse check)", func(t *testing.T) {
		// The manifest cross-check is bidirectional: an on-disk ADR that no
		// manifest line records — whether a benign hand-added ADR never
		// regen'd, or a tamper that deleted the line to hide an edit — must be
		// flagged, not passed silently.
		dir := t.TempDir()
		writeManifest(t, dir, nil) // empty manifest
		fresh := sampleADR()

		got, err := checkManifest(dir, []adr.ADR{fresh})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Kind != KindManifestMismatch {
			t.Fatalf("checkManifest() = %+v, want exactly one manifest_mismatch (on-disk ADR not recorded)", got)
		}
		if got[0].ID != "ADR-0001" {
			t.Errorf("violation.ID = %q, want ADR-0001", got[0].ID)
		}
	})

	t.Run("a malformed manifest line is a could-not-run error (M6: the manifest is machine-written)", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		a := sampleADR()
		// A well-formed line followed by a garbled one: the CLI never writes a
		// line missing the two-space separator, so it is corruption/tampering.
		content := manifest.Render([]adr.ADR{a})
		content = append(content, []byte("deadbeef-no-separator\n")...)
		if err := os.WriteFile(filepath.Join(dir, manifest.FileName), content, 0o644); err != nil {
			t.Fatal(err)
		}

		if _, err := checkManifest(dir, []adr.ADR{a}); err == nil {
			t.Error("checkManifest() = nil error, want a could-not-run error on a malformed manifest line")
		}
	})

	t.Run("missing manifest with existing ADRs is a could-not-run error", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		a := sampleADR()

		if _, err := checkManifest(dir, []adr.ADR{a}); err == nil {
			t.Error("checkManifest() = nil error, want an error (manifest missing but ADRs exist)")
		}
	})

	t.Run("missing manifest with zero ADRs is not an error", func(t *testing.T) {
		dir := t.TempDir()
		got, err := checkManifest(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("checkManifest() = %+v, want no violations", got)
		}
	})
}

func TestIDFromFilename(t *testing.T) {
	tests := []struct{ name, want string }{
		{"ADR-0001-a-title.md", "ADR-0001"},
		{"ADR-10023-big-id.md", "ADR-10023"},
		{"not-a-adr-file.md", "not-a-adr-file.md"},
	}
	for _, tt := range tests {
		if got := idFromFilename(tt.name); got != tt.want {
			t.Errorf("idFromFilename(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
