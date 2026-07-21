package patch

import (
	"reflect"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// FuzzSupersede hardens the hand-rolled byte editor (plan §8, fuzz target
// #1). The baseline contract is "never panic" for arbitrary bytes. When the
// input happens to be a valid *accepted* ADR, it additionally asserts the
// two properties the byte-preservation guarantee rests on:
//
//	(b) positional patch minimality — exactly one line (the status field)
//	    changed in place and exactly one line (the back-link) was inserted
//	    right after it; reverting those two edits reconstructs the input
//	    byte-for-byte, so every other byte is provably untouched.
//	(c) parse↔render agreement — the patched bytes re-parse to the same
//	    model as the original, changed only in Status and SupersededBy.
const fuzzPath = "ADR-0001-fuzz.md"

func FuzzSupersede(f *testing.F) {
	seeds := []string{
		sampleADR,
		"---\nid: ADR-0001\ntitle: T\ncategory: c\ndate: 2026-07-01\nstatus: accepted\n---\n\n## Context and Problem Statement\n\nx\n\n## Considered Options\n\n- a\n\n## Decision Outcome\n\ny\n",
		"\ufeff---\r\nid: ADR-0001\r\ntitle: T\r\ncategory: c\r\ndate: 2026-07-01\r\nstatus: accepted\r\n---\r\n\r\n## Decision Outcome\r\n\r\ny\r\n",
		"",
		"---\n---\n",
		"not an adr\n",
		// Multi-line plain-scalar status forms (value on a continuation
		// line): the editor must refuse these rather than corrupt them.
		// The empty-value variant is the original FuzzSupersede find
		// (testdata/fuzz corpus entry fbf36b2d47bfee47).
		"---\nid: ADR-0001\ntitle: T\ncategory: c\ndate: 2026-07-01\nstatus:\n accepted\n---\n\n## Decision Outcome\n\ny\n",
		"---\nid: ADR-0001\ntitle: T\ncategory: c\ndate: 2026-07-01\nstatus: # note\n accepted\n---\n\n## Decision Outcome\n\ny\n",
		// Review-found corruption repro: double-quoted scalar with a
		// backslash line continuation folds to "accepted"; the on-line
		// value is non-empty, so only the post-edit verification
		// (ErrUnsafeEdit) catches it.
		quotedContinuationADR,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		out, err := Supersede(data, "ADR-0002")
		// Whatever the input, the editor must not panic. An error on
		// malformed input is fine.
		if err != nil {
			return
		}

		orig, ok := parseAccepted(data)
		if !ok {
			return // output produced, but input wasn't a valid accepted ADR
		}

		// (b) POSITIONAL minimality: exactly one line changed in place and
		// exactly one line was inserted right after it. Proven by
		// reconstruction — revert the changed line to the original and
		// delete the inserted line; the result must be byte-identical to
		// the input. This is strictly stronger than a multiset line-diff:
		// it pins WHERE the edit happened, not just which line values
		// appeared/disappeared.
		oLines := splitKeepEnds(string(data))
		pLines := splitKeepEnds(string(out))
		if len(pLines) != len(oLines)+1 {
			t.Fatalf("line count changed by %d, want +1", len(pLines)-len(oLines))
		}
		i := 0
		for i < len(oLines) && oLines[i] == pLines[i] {
			i++
		}
		if i == len(oLines) {
			t.Fatalf("no divergent line found; status line was not changed:\n%s", out)
		}
		// The changed line must be the frontmatter status field, and the
		// inserted line the derived back-link.
		if _, key, _, fieldOK := splitField(pLines[i].text); !fieldOK || key != "status" {
			t.Fatalf("changed line is not the status field: %q", pLines[i].text)
		}
		if pLines[i+1].text != "superseded-by: ADR-0002" {
			t.Fatalf("line after the status field is not the inserted back-link: %q", pLines[i+1].text)
		}
		rec := make([]line, 0, len(oLines))
		rec = append(rec, pLines[:i]...)
		rec = append(rec, oLines[i])
		rec = append(rec, pLines[i+2:]...)
		if join(rec) != string(data) {
			t.Fatalf("reverting the status line and deleting the inserted line does not reconstruct the input:\ninput:  %q\nrebuilt: %q", data, join(rec))
		}

		// (c) parse↔render agreement.
		patched, ok := parseAny(out)
		if !ok {
			t.Fatalf("patched ADR no longer parses:\n%s", out)
		}
		if patched.Status != adr.StatusSuperseded {
			t.Fatalf("status = %q, want superseded", patched.Status)
		}
		if patched.SupersededBy != "ADR-0002" {
			t.Fatalf("superseded-by = %q, want ADR-0002", patched.SupersededBy)
		}
		assertSameExceptStatus(t, orig, patched)
	})
}

func parseAccepted(data []byte) (*adr.ADR, bool) {
	a, err := adr.ParseBytes(data, fuzzPath)
	if err != nil || a.Status != adr.StatusAccepted {
		return nil, false
	}
	return a, true
}

func parseAny(data []byte) (*adr.ADR, bool) {
	a, err := adr.ParseBytes(data, fuzzPath)
	if err != nil {
		return nil, false
	}
	return a, true
}

func assertSameExceptStatus(t *testing.T, a, b *adr.ADR) {
	t.Helper()
	if a.ID != b.ID || a.Title != b.Title ||
		a.Date != b.Date || a.Source != b.Source || a.Supersedes != b.Supersedes ||
		!reflect.DeepEqual(a.SupersedesRules, b.SupersedesRules) ||
		!reflect.DeepEqual(a.RemovesRules, b.RemovesRules) {
		t.Fatalf("frozen frontmatter changed:\n%+v\n%+v", a, b)
	}
	if !reflect.DeepEqual(a.Sections, b.Sections) {
		t.Fatalf("body sections changed:\n%q\n%q", a.Sections, b.Sections)
	}
}
