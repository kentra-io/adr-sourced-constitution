package patch

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// FuzzSupersede hardens the hand-rolled byte editor (plan §8, fuzz target
// #1). The baseline contract is "never panic" for arbitrary bytes. When the
// input happens to be a valid *accepted* ADR, it additionally asserts the
// two properties the byte-preservation guarantee rests on:
//
//	(b) patch minimality — the output differs from the input by exactly one
//	    removed line (the old status) and two added lines (the new status
//	    and the superseded-by back-link); every other byte is untouched.
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

		// (b) minimality on the raw bytes.
		o, p := lines(string(data)), lines(string(out))
		if len(p) != len(o)+1 {
			t.Fatalf("line count changed by %d, want +1", len(p)-len(o))
		}
		removed, added := multisetDiff(o, p)
		if len(removed) != 1 || len(added) != 2 {
			t.Fatalf("diff not minimal: removed=%q added=%q", removed, added)
		}
		if !strings.HasPrefix(strings.TrimSpace(removed[0]), "status:") {
			t.Fatalf("removed line is not the status line: %q", removed[0])
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
	if a.ID != b.ID || a.Title != b.Title || a.Category != b.Category ||
		a.Date != b.Date || a.Source != b.Source || a.Supersedes != b.Supersedes {
		t.Fatalf("frozen frontmatter changed:\n%+v\n%+v", a, b)
	}
	if !reflect.DeepEqual(a.Sections, b.Sections) {
		t.Fatalf("body sections changed:\n%q\n%q", a.Sections, b.Sections)
	}
}
