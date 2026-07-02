package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

func parse(t *testing.T, data, path string) adr.ADR {
	t.Helper()
	a, err := adr.ParseBytes([]byte(data), path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return *a
}

const acceptedADR = `---
id: ADR-0001
title: Use event sourcing
category: architecture
date: 2026-07-01
status: accepted
---

## Context and Problem Statement

x

## Considered Options

- a

## Decision Outcome

Do it.
`

// TestHashIgnoresStatusFields is the load-bearing property (plan §2.7): a
// legal status transition must NOT change an ADR's manifest hash, so the
// manifest only ever gains lines and a superseded/deprecated ADR isn't
// flagged as tampered.
func TestHashIgnoresStatusFields(t *testing.T) {
	accepted := parse(t, acceptedADR, "ADR-0001-x.md")

	superseded := strings.Replace(acceptedADR,
		"status: accepted\n", "status: superseded\nsuperseded-by: ADR-0002\n", 1)
	sup := parse(t, superseded, "ADR-0001-x.md")

	deprecated := strings.Replace(acceptedADR, "status: accepted", "status: deprecated", 1)
	dep := parse(t, deprecated, "ADR-0001-x.md")

	if Hash(accepted) != Hash(sup) {
		t.Error("hash changed after supersede transition; status/superseded-by must be excluded from the frozen-content hash")
	}
	if Hash(accepted) != Hash(dep) {
		t.Error("hash changed after deprecate transition; status must be excluded from the frozen-content hash")
	}
}

// TestHashCRLFEqualsLF is the M2 DoD unit: CRLF-authored content hashes the
// same as its LF twin, because canonicalization runs off the normalized
// parsed model.
func TestHashCRLFEqualsLF(t *testing.T) {
	lf := parse(t, acceptedADR, "ADR-0001-x.md")
	crlf := parse(t, strings.ReplaceAll(acceptedADR, "\n", "\r\n"), "ADR-0001-x.md")
	if Hash(lf) != Hash(crlf) {
		t.Errorf("CRLF hash %s != LF hash %s", Hash(crlf), Hash(lf))
	}
}

// TestHashDetectsFrozenEdits proves the guard's future value: an edit to
// frozen content (the Decision Outcome, the category) does change the hash.
func TestHashDetectsFrozenEdits(t *testing.T) {
	base := parse(t, acceptedADR, "ADR-0001-x.md")

	bodyEdit := parse(t, strings.Replace(acceptedADR, "Do it.", "Do it differently.", 1), "ADR-0001-x.md")
	if Hash(base) == Hash(bodyEdit) {
		t.Error("hash unchanged after a body edit; frozen content must be covered")
	}

	catEdit := parse(t, strings.Replace(acceptedADR, "category: architecture", "category: process", 1), "ADR-0001-x.md")
	if Hash(base) == Hash(catEdit) {
		t.Error("hash unchanged after a category edit; frozen frontmatter must be covered")
	}
}

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	a := parse(t, acceptedADR, "ADR-0001-x.md")
	if err := Write(dir, []adr.ADR{a}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if string(got) != string(Render([]adr.ADR{a})) {
		t.Errorf("written manifest = %q, want it to equal Render output", got)
	}
}

func TestRenderFormatAndOrder(t *testing.T) {
	a1 := parse(t, strings.ReplaceAll(acceptedADR, "ADR-0001", "ADR-0002"), "ADR-0002-b.md")
	a2 := parse(t, acceptedADR, "ADR-0001-a.md")

	out := string(Render([]adr.ADR{a1, a2})) // deliberately out of order

	gotLines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(gotLines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(gotLines), out)
	}
	// Sorted by filename: ADR-0001-a.md before ADR-0002-b.md.
	if !strings.HasSuffix(gotLines[0], "  ADR-0001-a.md") {
		t.Errorf("line 0 = %q, want it to end with ADR-0001-a.md", gotLines[0])
	}
	if !strings.HasSuffix(gotLines[1], "  ADR-0002-b.md") {
		t.Errorf("line 1 = %q, want it to end with ADR-0002-b.md", gotLines[1])
	}
	// sha256sum-style: 64 hex chars + two spaces + filename.
	if len(gotLines[0]) < 66 || gotLines[0][64:66] != "  " {
		t.Errorf("line format not '<64hex>  <name>': %q", gotLines[0])
	}
}
