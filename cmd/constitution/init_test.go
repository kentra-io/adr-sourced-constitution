package main

import (
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// TestParseFoundingFileValid confirms the happy path: one principle per
// "## " heading, with the statement falling back to the title when the body
// is empty. (The founding-file rules grammar lands with M1 Task 5.)
func TestParseFoundingFileValid(t *testing.T) {
	content := "## Tests are mandatory\n\nEvery change ships with tests.\n\n## Small focused PRs\n\nKeep PRs small.\n\n## Title only\n"
	ps, err := parseFoundingFile(content)
	if err != nil {
		t.Fatalf("parseFoundingFile(valid) = %v, want nil", err)
	}
	if len(ps) != 3 {
		t.Fatalf("got %d principles, want 3", len(ps))
	}
	if ps[0].Title != "Tests are mandatory" || ps[0].Statement != "Every change ships with tests." {
		t.Errorf("ps[0] = %+v", ps[0])
	}
	if ps[1].Statement != "Keep PRs small." {
		t.Errorf("ps[1].Statement = %q, want the body text", ps[1].Statement)
	}
	if ps[2].Statement != "Title only" {
		t.Errorf("ps[2].Statement = %q, want the title fallback", ps[2].Statement)
	}
}

// TestParseFoundingFileEmpty proves a file with no "## " headings is a hard
// error rather than a silent no-op seed.
func TestParseFoundingFileEmpty(t *testing.T) {
	_, err := parseFoundingFile("just prose, no headings\n")
	if err == nil {
		t.Fatal("parseFoundingFile(no headings) = nil, want error")
	}
	want := "no principles found (expected one or more '## ' headings)"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestParseFoundingFileRejectsRulesHeading proves the interim guard: a
// "## Rules" heading is not a principle title and must hard-error rather
// than silently seed a bogus ADR titled "Rules" (founding rules land with
// the M1 Task 5 staged-init rework).
func TestParseFoundingFileRejectsRulesHeading(t *testing.T) {
	content := "## Tests are mandatory\n\nEvery change ships with tests.\n\n## Rules\n\n### testing\n\n#### mandatory-tests\n\nEvery change ships with tests.\n"
	_, err := parseFoundingFile(content)
	if err == nil {
		t.Fatal("parseFoundingFile(## Rules heading) = nil, want error")
	}
	want := `"## Rules" is not a principle title: the founding-file format cannot carry standing rules yet (one principle per "## " heading; founding rules land with the staged-init rework)`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestFoundingBodyIsValidMADR proves the composed founding body passes the
// same write-path validation `adr new` applies.
func TestFoundingBodyIsValidMADR(t *testing.T) {
	body := foundingBody("Adopt the thing.")
	if err := adr.ValidateBody([]byte(body), "founding"); err != nil {
		t.Fatalf("foundingBody output does not validate: %v", err)
	}
}
