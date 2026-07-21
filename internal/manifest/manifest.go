// Package manifest maintains constitution/adr/.manifest.sha256 — a
// per-ADR content hash over each record's *frozen* content, rewritten by
// every mutating command (plan §2.7) and cross-checked by `constitution
// guard`. The canonicalization rule this package implements is finalized,
// with its injection-proofing and advisory-only rationale, in the design
// note docs/manifest-canonicalization.md — read that for the authoritative
// account; the summary below is the in-code companion. The file format is
// sha256sum-style ("<hex>  <filename>" lines, sorted by filename) so it is
// human-diffable and reproducible.
//
// # Canonicalization (the load-bearing decision)
//
// The hash covers only the fields that are *immutable* once an ADR is
// accepted: the body and the frozen frontmatter fields (id, title, date,
// source, supersedes, supersedes-rules, removes-rules). It deliberately
// EXCLUDES status and superseded-by — the two fields a legal status
// transition may change (spec §5.3). The consequence is exactly what the
// immutability model wants: a legal supersede/deprecate does not alter the
// target ADR's manifest hash, so the manifest changes only by *gaining* the
// new ADR's line, while any illegal edit to frozen content (a reworded
// Decision Outcome, a retargeted supersedes-rules list) does change the
// hash and is therefore detectable.
//
// Canonicalization runs off the parsed model, not the raw bytes, so it is
// inherently line-ending- and BOM-independent: a CRLF-authored ADR and its
// LF twin hash identically (a tested invariant), because the parser
// normalizes both to the same model before Canonicalize ever sees them.
package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
)

// FileName is the manifest's fixed name under constitution/adr/. It is not
// a "*.md" file, so ParseDir ignores it and it never enters the ADR set.
const FileName = ".manifest.sha256"

// Canonicalize returns the deterministic byte representation of an ADR's
// frozen content that Hash sums over. See the package doc for the rationale
// behind which fields are included. The format is internal — never written
// to disk — so it optimizes purely for being unambiguous and stable across
// runs and platforms.
//
// Every value is length-prefixed (netstring-style "name <len>:<bytes>\n").
// This makes the encoding injection-proof: a value containing embedded
// newlines or text that mimics the framing (yaml block/quoted scalars allow
// both) cannot shift a boundary, so two distinct field/section tuples can
// never produce the same canonical bytes — the length pins each value's
// exact extent. A naive "name:value\n" scheme is forgeable: title
// "T\nsource:X" with source "c" and title "T" with source "X\nsource:c"
// would collide (asserted in TestCanonicalizeInjection).
func Canonicalize(a adr.ADR) []byte {
	var b bytes.Buffer
	field := func(name, value string) {
		fmt.Fprintf(&b, "%s %d:%s\n", name, len(value), value)
	}
	// Frozen frontmatter fields, fixed order. status and superseded-by are
	// intentionally absent (see package doc). The rule-retirement ref lists
	// are frozen too; refs cannot contain commas, so a comma join is
	// unambiguous inside the length-prefixed value.
	field("id", a.ID)
	field("title", a.Title)
	field("date", a.Date)
	field("source", a.Source)
	field("supersedes", a.Supersedes)
	field("supersedes-rules", adr.JoinRefs(a.SupersedesRules))
	field("removes-rules", adr.JoinRefs(a.RemovesRules))
	// Body sections in the order they appeared in the file, normalized to
	// heading + trimmed content (exactly the model the parser exposes). The
	// section count pins the structure; each heading/content pair is
	// length-prefixed like the fields above.
	fmt.Fprintf(&b, "sections %d\n", len(a.SectionOrder))
	for _, h := range a.SectionOrder {
		field("heading", h)
		field("content", a.Sections[h])
	}
	return b.Bytes()
}

// Hash returns the hex-encoded SHA-256 of an ADR's canonical content.
func Hash(a adr.ADR) string {
	sum := sha256.Sum256(Canonicalize(a))
	return hex.EncodeToString(sum[:])
}

// Render produces the manifest file bytes for the given ADR set: one
// "<hex>  <filename>" line per ADR, sorted by filename for determinism.
func Render(adrs []adr.ADR) []byte {
	type row struct{ name, hash string }
	rows := make([]row, 0, len(adrs))
	for i := range adrs {
		rows = append(rows, row{name: filepath.Base(adrs[i].Path), hash: Hash(adrs[i])})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	var b bytes.Buffer
	for _, r := range rows {
		fmt.Fprintf(&b, "%s  %s\n", r.hash, r.name)
	}
	return b.Bytes()
}

// Write atomically (re)writes the manifest under adrDir from the given ADR
// set. Called by `regen`, hence by every mutating command that ends in a
// regen.
func Write(adrDir string, adrs []adr.ADR) error {
	return atomicwrite.WriteFile(filepath.Join(adrDir, FileName), Render(adrs), 0o644)
}
