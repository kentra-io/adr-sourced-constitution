package adr

import "testing"

// FuzzSplitFrontmatter hardens the hand-rolled byte-level frontmatter
// split (implementation-plan.md §3: "no YAML lib guarantees byte-for-byte
// round-trip... only a raw-line edit satisfies §5.3 exactly" — the same
// reasoning that makes this code fuzz-worthy rather than delegated to a
// library). The only property under fuzzing is "never panics"; malformed
// input returning an error is expected and correct.
//
// Seed corpus lives in testdata/fuzz/FuzzSplitFrontmatter/ (go's native
// convention) and runs on every `go test` (seed-only mode, no -fuzz flag
// needed) per implementation-plan.md §7/§8 ("seed-only on every PR").
func FuzzSplitFrontmatter(f *testing.F) {
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
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(_ *testing.T, data []byte) {
		// The only contract under fuzzing: never panic. Any (frontmatter,
		// body, error) combination is an acceptable outcome.
		_, _, _ = SplitFrontmatter(data)
	})
}
