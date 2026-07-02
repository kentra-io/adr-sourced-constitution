package patch

import (
	"errors"
	"sort"
	"strings"
	"testing"
)

const sampleADR = `---
id: ADR-0003
title: Use event sourcing
category: architecture
date: 2026-07-01
status: accepted
---

## Context and Problem Statement

We need supersede semantics.

## Considered Options

- A
- B

## Decision Outcome

Chosen: event sourcing.
`

func TestSetStatus(t *testing.T) {
	out, err := SetStatus([]byte(sampleADR), "deprecated")
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if !strings.Contains(string(out), "status: deprecated\n") {
		t.Fatalf("status not updated:\n%s", out)
	}
	// Exactly one line differs (the status line); nothing added or removed.
	only, added := multisetDiff(lines(sampleADR), lines(string(out)))
	assertEqual(t, "removed lines", only, []string{"status: accepted"})
	assertEqual(t, "added lines", added, []string{"status: deprecated"})
}

func TestSupersedeMinimality(t *testing.T) {
	out, err := Supersede([]byte(sampleADR), "ADR-0009")
	if err != nil {
		t.Fatalf("Supersede: %v", err)
	}
	orig, patched := lines(sampleADR), lines(string(out))
	if len(patched) != len(orig)+1 {
		t.Fatalf("expected exactly one inserted line, got %d -> %d", len(orig), len(patched))
	}
	// The mechanical byte-preservation assertion (plan §8 property (b)): the
	// only lines that appear/disappear are the status flip and the inserted
	// back-link — every other byte is untouched.
	removed, added := multisetDiff(orig, patched)
	assertEqual(t, "removed lines", removed, []string{"status: accepted"})
	assertEqual(t, "added lines", added, []string{"status: superseded", "superseded-by: ADR-0009"})

	// And positionally: superseded-by sits immediately after status.
	joined := strings.Join(patched, "\n")
	if !strings.Contains(joined, "status: superseded\nsuperseded-by: ADR-0009\n") {
		t.Fatalf("superseded-by not inserted immediately after status:\n%s", out)
	}
}

func TestSetID(t *testing.T) {
	out, err := SetID([]byte(sampleADR), "ADR-0100")
	if err != nil {
		t.Fatalf("SetID: %v", err)
	}
	removed, added := multisetDiff(lines(sampleADR), lines(string(out)))
	assertEqual(t, "removed lines", removed, []string{"id: ADR-0003"})
	assertEqual(t, "added lines", added, []string{"id: ADR-0100"})
}

func TestPreservesCRLFAndBOM(t *testing.T) {
	in := "\ufeff---\r\nid: ADR-0003\r\ntitle: T\r\nstatus: accepted\r\n---\r\n\r\n## Decision Outcome\r\n\r\nBody.\r\n"
	out, err := Supersede([]byte(in), "ADR-0009")
	if err != nil {
		t.Fatalf("Supersede: %v", err)
	}
	s := string(out)
	if !strings.HasPrefix(s, "\ufeff") {
		t.Error("BOM not preserved")
	}
	// The inserted line inherits the status line's CRLF terminator.
	if !strings.Contains(s, "status: superseded\r\nsuperseded-by: ADR-0009\r\n") {
		t.Errorf("CRLF terminator not preserved on inserted line:\n%q", s)
	}
	// Body bytes untouched (still CRLF).
	if !strings.Contains(s, "## Decision Outcome\r\n\r\nBody.\r\n") {
		t.Errorf("body CRLF not preserved:\n%q", s)
	}
}

func TestPreservesUnusualSpacing(t *testing.T) {
	// A status line with extra spacing around the value must keep the key +
	// separator exactly; only the value changes.
	in := "---\nid: ADR-0003\nstatus:   accepted\n---\n\nbody\n"
	out, err := SetStatus([]byte(in), "deprecated")
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if !strings.Contains(string(out), "status:   deprecated\n") {
		t.Errorf("separator whitespace not preserved:\n%q", out)
	}
}

func TestDoesNotTouchBodyStatusLine(t *testing.T) {
	// A "status:" occurring in the body must never be edited — only the
	// frontmatter field is.
	in := "---\nid: ADR-0003\nstatus: accepted\n---\n\n## Decision Outcome\n\nstatus: whatever\n"
	out, err := SetStatus([]byte(in), "deprecated")
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if !strings.Contains(string(out), "status: whatever\n") {
		t.Errorf("body status line was clobbered:\n%s", out)
	}
	if strings.Count(string(out), "status: deprecated") != 1 {
		t.Errorf("expected exactly one frontmatter status edit:\n%s", out)
	}
}

func TestSkipsNonFieldFrontmatterLines(t *testing.T) {
	// A blank line and a comment-like line inside the frontmatter must be
	// skipped, not mistaken for the target field.
	in := "---\n# a comment, not a field\nid: ADR-0003\n\nstatus: accepted\n---\n\nbody\n"
	out, err := SetStatus([]byte(in), "deprecated")
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if !strings.Contains(string(out), "status: deprecated\n") {
		t.Errorf("status not updated past non-field lines:\n%s", out)
	}
	// The comment and blank line are byte-preserved.
	if !strings.Contains(string(out), "# a comment, not a field\n") {
		t.Errorf("comment line not preserved:\n%s", out)
	}
}

func TestPreservesMissingTrailingNewline(t *testing.T) {
	// An ADR whose final body line has no trailing newline must round-trip
	// that exact shape through an edit.
	in := "---\nid: ADR-0003\nstatus: accepted\n---\n\nbody with no trailing newline"
	out, err := SetStatus([]byte(in), "deprecated")
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if strings.HasSuffix(string(out), "\n") {
		t.Errorf("a trailing newline was introduced:\n%q", out)
	}
	if !strings.HasSuffix(string(out), "body with no trailing newline") {
		t.Errorf("final line not preserved:\n%q", out)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"no frontmatter", "just a body, no delimiters\n", ErrNoFrontmatter},
		{"unterminated frontmatter", "---\nid: ADR-0003\nstatus: accepted\n", ErrNoFrontmatter},
		{"missing field", "---\nid: ADR-0003\n---\n\nbody\n", ErrFieldNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := SetStatus([]byte(tt.in), "deprecated"); !errors.Is(err, tt.want) {
				t.Errorf("SetStatus error = %v, want %v", err, tt.want)
			}
		})
	}
}

// --- helpers ---

func lines(s string) []string { return strings.Split(s, "\n") }

// multisetDiff returns the lines that are present a different number of
// times between orig and patched: removed (net-negative in patched) and
// added (net-positive), each sorted. It is the mechanical basis for the
// byte-preservation assertion — a line that is byte-identical and equally
// frequent on both sides cancels out.
func multisetDiff(orig, patched []string) (removed, added []string) {
	count := map[string]int{}
	for _, l := range orig {
		count[l]--
	}
	for _, l := range patched {
		count[l]++
	}
	for l, n := range count {
		for ; n > 0; n-- {
			added = append(added, l)
		}
		for ; n < 0; n++ {
			removed = append(removed, l)
		}
	}
	sort.Strings(removed)
	sort.Strings(added)
	return removed, added
}

func assertEqual(t *testing.T, what string, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("%s = %q, want %q", what, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s = %q, want %q", what, got, want)
		}
	}
}
