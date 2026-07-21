package adr

import "testing"

// FuzzParseBytes hardens the full parse pipeline — normalization
// (BOM strip, CRLF->LF), the hand-rolled frontmatter split, yaml schema
// validation, section extraction, and the filename<->id cross-check
// (implementation-plan.md §3's rationale for fuzzing the byte-level
// code). It subsumes the earlier FuzzSplitFrontmatter target; that
// corpus was migrated to testdata/fuzz/FuzzParseBytes/ (same
// single-[]byte input shape). The only property under fuzzing is "never
// panics"; malformed input returning an error is expected and correct.
//
// The path argument is fixed: it only feeds error strings and the
// filename<->id cross-check, so varying it adds no interesting coverage,
// and a fixed "ADR-0001-*.md" name lets the valid ADR-0001 seed exercise
// the full happy path.
//
// Seed corpus runs on every `go test` (native seed-only mode, no -fuzz
// flag needed) per implementation-plan.md §7/§8 ("seed-only on every PR").
func FuzzParseBytes(f *testing.F) {
	seeds := []string{
		"",
		"---\n---\n",
		"---\nid: ADR-0001\n---\n\n## Decision Outcome\n\nBody.\n",
		"---\nid: ADR-0001\n",          // no closing delimiter
		"no frontmatter here at all\n", // no opening delimiter
		"---",                          // just the opening line, no newline
		"---\r\nid: ADR-0001\r\n---\r\n\r\nbody\r\n", // CRLF
		"---\n---\n---\n",                                // extra delimiter-looking lines in body
		"----\nid: ADR-0001\n----\n",                     // delimiter look-alikes (4 dashes)
		"---\n" + string([]byte{0x00, 0xff}) + "\n---\n", // binary bytes
		// A fully valid, BOM-prefixed ADR (id matches the fixed fuzz
		// path) so the happy path through parseMeta/ExtractSections/
		// filenameID is in the baseline corpus.
		"\xef\xbb\xbf---\nid: ADR-0001\ntitle: T\ndate: 2026-07-01\nstatus: accepted\n---\n\n## Decision Outcome\n\nBody.\n",
		// A rule-bearing ADR under the h3/h4 Rules grammar, plus a
		// rule-retirement frontmatter list, so the Rules pipeline is in
		// the baseline corpus.
		"---\nid: ADR-0001\ntitle: T\ndate: 2026-07-01\nstatus: accepted\nsupersedes-rules: [ADR-0002/testing/old-tiers]\n---\n\n## Decision Outcome\n\nBody.\n\n## Rules\n\n### testing\n\n#### t\n\ntext\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(_ *testing.T, data []byte) {
		// The only contract under fuzzing: never panic. Any (*ADR, error)
		// combination is an acceptable outcome.
		_, _ = ParseBytes(data, "ADR-0001-fuzz-input.md")
	})
}
