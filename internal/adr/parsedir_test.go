package adr

import (
	"path/filepath"
	"testing"
)

func TestParseDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "ADR-0002-second.md", `---
id: ADR-0002
title: Second
date: 2026-07-02
status: accepted
---

## Decision Outcome

Second decision.
`)
	writeFile(t, dir, "ADR-0001-first.md", `---
id: ADR-0001
title: First
date: 2026-07-01
status: accepted
---

## Decision Outcome

First decision.
`)
	// A non-".md" file in the same directory must be ignored.
	writeFile(t, dir, "README.txt", "not an ADR")

	adrs, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir() error = %v", err)
	}
	if len(adrs) != 2 {
		t.Fatalf("len(ParseDir()) = %d, want 2", len(adrs))
	}
	// Sorted by filename, so ADR-0001 (first alphabetically) comes first,
	// independent of numeric id.
	if adrs[0].ID != "ADR-0001" || adrs[1].ID != "ADR-0002" {
		t.Errorf("ParseDir() ids = %q, %q, want ADR-0001, ADR-0002", adrs[0].ID, adrs[1].ID)
	}
}

func TestParseDirFailsFastOnMalformedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "ADR-0001-good.md", `---
id: ADR-0001
title: Good
date: 2026-07-01
status: accepted
---

## Decision Outcome

Fine.
`)
	writeFile(t, dir, "ADR-0002-bad.md", "not frontmatter at all\n")

	_, err := ParseDir(dir)
	if err == nil {
		t.Fatal("ParseDir() error = nil, want the malformed file's error")
	}
	want := filepath.Join(dir, "ADR-0002-bad.md") + ":1: file must start with a \"---\" frontmatter delimiter line"
	if err.Error() != want {
		t.Errorf("ParseDir() error = %q, want %q", err.Error(), want)
	}
}

// TestParseDirDuplicateID proves ids are unique across the log: two
// files sharing an id (both filename-encoded and in frontmatter, so the
// per-file filename<->id cross-check passes) must hard-error, with a
// precise ParseError naming BOTH files.
func TestParseDirDuplicateID(t *testing.T) {
	dir := t.TempDir()
	adrContent := func(title string) string {
		return `---
id: ADR-0001
title: ` + title + `
date: 2026-07-01
status: accepted
---

## Decision Outcome

Outcome.
`
	}
	writeFile(t, dir, "ADR-0001-first-claimant.md", adrContent("First claimant"))
	writeFile(t, dir, "ADR-0001-second-claimant.md", adrContent("Second claimant"))

	_, err := ParseDir(dir)
	if err == nil {
		t.Fatal("ParseDir() error = nil, want a duplicate-id error")
	}
	want := filepath.Join(dir, "ADR-0001-second-claimant.md") +
		`: field "id": duplicate id "ADR-0001" (already used by ` +
		filepath.Join(dir, "ADR-0001-first-claimant.md") + `)`
	if err.Error() != want {
		t.Errorf("ParseDir() error = %q, want %q", err.Error(), want)
	}
}

func TestParseDirMissingDirectory(t *testing.T) {
	_, err := ParseDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("ParseDir() error = nil, want an error for a missing directory")
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("constitution/adr/ADR-0001-x.md", 3, "category", "not in vocabulary")
	want := `constitution/adr/ADR-0001-x.md:3: field "category": not in vocabulary`
	if err.Error() != want {
		t.Errorf("NewValidationError().Error() = %q, want %q", err.Error(), want)
	}
}
