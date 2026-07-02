// Package manifest maintains constitution/adr/.manifest.sha256 — a
// per-ADR content hash over each record's *frozen* content, rewritten by
// every mutating command (plan §2.7). In v1 this is write-path only; the
// guard-side cross-check that turns it into tamper-evidence is M3, which
// finalizes the design note around Canonicalize. The file format is
// sha256sum-style ("<hex>  <filename>" lines, sorted by filename) so it is
// human-diffable and reproducible.
//
// # Canonicalization (the load-bearing decision)
//
// The hash covers only the fields that are *immutable* once an ADR is
// accepted: the body and the frozen frontmatter fields (id, title,
// category, date, source, supersedes). It deliberately EXCLUDES status and
// superseded-by — the two fields a legal status transition may change (spec
// §5.3). The consequence is exactly what the immutability model wants: a
// legal supersede/deprecate does not alter the target ADR's manifest hash,
// so the manifest changes only by *gaining* the new ADR's line, while any
// illegal edit to frozen content (a reworded Decision Outcome, a changed
// category) does change the hash and is therefore detectable.
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
// "T\ncategory:X" with category "c" and title "T" with category
// "X\ncategory:c" would collide (asserted in TestCanonicalizeInjection).
func Canonicalize(a adr.ADR) []byte {
	var b bytes.Buffer
	field := func(name, value string) {
		fmt.Fprintf(&b, "%s %d:%s\n", name, len(value), value)
	}
	// Frozen frontmatter fields, fixed order. status and superseded-by are
	// intentionally absent (see package doc).
	field("id", a.ID)
	field("title", a.Title)
	field("category", a.Category)
	field("date", a.Date)
	field("source", a.Source)
	field("supersedes", a.Supersedes)
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
