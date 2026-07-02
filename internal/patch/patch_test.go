package patch

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
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

func TestAcceptsYamlKeySpacing(t *testing.T) {
	// The yaml parser (go.yaml.in/yaml/v3, verified empirically) accepts
	// whitespace BEFORE the colon: "status :", "status  :", and "status\t:"
	// all parse as key "status". The patch grammar must accept exactly the
	// same lines, or an ADR the parser accepts could never be superseded or
	// deprecated. The original spacing is preserved verbatim in the output.
	tests := []struct {
		name       string
		statusLine string // the raw status line in the input
		wantLine   string // the expected patched line
	}{
		{"space before colon", "status : accepted", "status : deprecated"},
		{"two spaces before colon", "status  : accepted", "status  : deprecated"},
		{"tab before colon", "status\t: accepted", "status\t: deprecated"},
		{"tab after colon", "status :\taccepted", "status :\tdeprecated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := "---\nid: ADR-0003\n" + tt.statusLine + "\n---\n\nbody\n"
			out, err := SetStatus([]byte(in), "deprecated")
			if err != nil {
				t.Fatalf("SetStatus(%q): %v", tt.statusLine, err)
			}
			if !strings.Contains(string(out), tt.wantLine+"\n") {
				t.Errorf("patched line not found; want %q in:\n%s", tt.wantLine, out)
			}
			// Minimality still holds: only the status line changed.
			removed, added := multisetDiff(lines(in), lines(string(out)))
			assertEqual(t, "removed lines", removed, []string{tt.statusLine})
			assertEqual(t, "added lines", added, []string{tt.wantLine})
		})
	}
}

func TestInteriorWhitespaceKeyIsNotTheField(t *testing.T) {
	// "sta tus:" parses in yaml as the literal key "sta tus" — a different
	// key — so patch must not treat it as the status field.
	in := "---\nid: ADR-0003\nsta tus: accepted\n---\n\nbody\n"
	if _, err := SetStatus([]byte(in), "deprecated"); !errors.Is(err, ErrFieldNotFound) {
		t.Errorf("SetStatus error = %v, want ErrFieldNotFound", err)
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

// quotedContinuationADR is the review-found corruption repro: a
// double-quoted status scalar with a backslash line continuation. yaml
// folds it to "accepted" (verified empirically against go.yaml.in), the
// on-line value `"accep\` is non-empty and not a comment — so the friendly
// heuristic cannot see it — and a naive single-line edit orphans `ted"`,
// producing a file that no longer parses. The full ADR is valid, so the
// post-edit verification (ErrUnsafeEdit) must engage and refuse.
const quotedContinuationADR = "---\n" +
	"id: ADR-0001\n" +
	"title: Quoted continuation\n" +
	"category: architecture\n" +
	"date: 2026-06-01\n" +
	"status: \"accep\\\nted\"\n" +
	"---\n" +
	"\n" +
	"## Context and Problem Statement\n\nx\n\n" +
	"## Considered Options\n\n- a\n\n" +
	"## Decision Outcome\n\ny\n"

func TestQuotedContinuationRefused(t *testing.T) {
	// Precondition: the fixture really is a valid accepted ADR.
	a, err := adr.ParseBytesUnnamed([]byte(quotedContinuationADR), "fixture")
	if err != nil {
		t.Fatalf("fixture broken, does not parse: %v", err)
	}
	if a.Status != adr.StatusAccepted {
		t.Fatalf("fixture broken: status = %q, want accepted", a.Status)
	}

	// Supersede and SetStatus target the status field and must refuse with
	// ErrUnsafeEdit — the mechanical backstop; the heuristic cannot catch
	// this form.
	if _, err := Supersede([]byte(quotedContinuationADR), "ADR-0002"); !errors.Is(err, ErrUnsafeEdit) {
		t.Errorf("Supersede error = %v, want ErrUnsafeEdit", err)
	}
	if _, err := SetStatus([]byte(quotedContinuationADR), "deprecated"); !errors.Is(err, ErrUnsafeEdit) {
		t.Errorf("SetStatus error = %v, want ErrUnsafeEdit", err)
	}
	// SetID edits only the (single-line) id line here and leaves the status
	// line untouched, so it must succeed — pinning the guard's precision:
	// it refuses corruption, not the file per se.
	if _, err := SetID([]byte(quotedContinuationADR), "ADR-0042"); err != nil {
		t.Errorf("SetID error = %v, want nil (the id line itself is single-line)", err)
	}
}

func TestQuotedContinuationIDRefused(t *testing.T) {
	// The same construct on the id field must make SetID refuse.
	in := "---\n" +
		"id: \"ADR-\\\n0001\"\n" +
		"title: T\n" +
		"category: c\n" +
		"date: 2026-06-01\n" +
		"status: accepted\n" +
		"---\n\n## Decision Outcome\n\ny\n"
	if a, err := adr.ParseBytesUnnamed([]byte(in), "fixture"); err != nil || a.ID != "ADR-0001" {
		t.Fatalf("fixture broken: a=%+v err=%v", a, err)
	}
	if _, err := SetID([]byte(in), "ADR-0042"); !errors.Is(err, ErrUnsafeEdit) {
		t.Errorf("SetID error = %v, want ErrUnsafeEdit", err)
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
		// yaml multi-line plain scalar: "status:\n accepted" parses as
		// status=accepted, but the value is on a continuation line a
		// single-line edit would corrupt (found by FuzzSupersede). The
		// editor must refuse, leaving the file untouched.
		{"value on continuation line", "---\nid: ADR-0003\nstatus:\n accepted\n---\n\nbody\n", ErrValueNotOnKeyLine},
		{"empty value with trailing space", "---\nid: ADR-0003\nstatus: \n accepted\n---\n\nbody\n", ErrValueNotOnKeyLine},
		// Same shape hidden behind a comment: yaml strips "# note", so the
		// real value again lives on the continuation line.
		{"comment then continuation", "---\nid: ADR-0003\nstatus: # note\n accepted\n---\n\nbody\n", ErrValueNotOnKeyLine},
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
