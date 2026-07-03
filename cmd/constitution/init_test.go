package main

import (
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// TestParseFoundingFileValid confirms the happy path still parses: one
// principle per "## " heading, an optional nested "## Rule" making a principle
// rule-bearing, and a statement falling back to the title when the body is
// empty.
func TestParseFoundingFileValid(t *testing.T) {
	content := "## Tests are mandatory\n\nEvery change ships with tests.\n\n## Rule\n\nEvery change ships with tests.\n\n## Small focused PRs\n\nKeep PRs small.\n"
	ps, err := parseFoundingFile(content)
	if err != nil {
		t.Fatalf("parseFoundingFile(valid) = %v, want nil", err)
	}
	if len(ps) != 2 {
		t.Fatalf("got %d principles, want 2", len(ps))
	}
	if ps[0].Rule != "Every change ships with tests." {
		t.Errorf("ps[0].Rule = %q, want the rule text", ps[0].Rule)
	}
	if ps[1].Rule != "" {
		t.Errorf("ps[1].Rule = %q, want empty (catalog-only)", ps[1].Rule)
	}
	if ps[1].Statement != "Keep PRs small." {
		t.Errorf("ps[1].Statement = %q, want the body text", ps[1].Statement)
	}
}

// TestParseFoundingFileBlankRule proves a present-but-blank "## Rule" under a
// principle is a hard error, not a silently-dropped catalog-only record (fix
// #1: rule input is validated, never silently swallowed).
func TestParseFoundingFileBlankRule(t *testing.T) {
	content := "## Tests are mandatory\n\nEvery change ships with tests.\n\n## Rule\n\n"
	_, err := parseFoundingFile(content)
	if err == nil {
		t.Fatal("parseFoundingFile(blank Rule) = nil, want error")
	}
	want := `principle "Tests are mandatory" has a "## Rule" section that is present but empty; give it a rule statement or remove the heading (a catalog-only principle has no "## Rule")`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestParseFoundingFileRuleFirstHeading proves "## Rule" may not be the first
// heading: "Rule" is a reserved section name, not a principle title (fix #2).
func TestParseFoundingFileRuleFirstHeading(t *testing.T) {
	content := "## Rule\n\nSome rule.\n\n## Real Principle\n\nBody.\n"
	_, err := parseFoundingFile(content)
	if err == nil {
		t.Fatal("parseFoundingFile(Rule first) = nil, want error")
	}
	want := `"## Rule" may not be the first heading: "Rule" is a reserved section name, not a principle title; a founding file is one principle per "## " heading, with an optional nested "## Rule" beneath each`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestParseFoundingFileDuplicateRule proves two "## Rule" sections under one
// principle are rejected rather than letting the last silently win (fix #3,
// applied symmetrically to the founding-file path).
func TestParseFoundingFileDuplicateRule(t *testing.T) {
	content := "## P1\n\nstmt\n\n## Rule\n\nr1\n\n## Rule\n\nr2\n"
	_, err := parseFoundingFile(content)
	if err == nil {
		t.Fatal("parseFoundingFile(duplicate Rule) = nil, want error")
	}
	want := `principle "P1" has more than one "## Rule" section; a principle may carry at most one`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestParseFoundingFileRuleHeadingLine proves a founding-file rule carrying a
// Markdown heading line is rejected downstream once composed and re-parsed
// (fix #4 inherited via the read-path validator): the "## Rule" content keeps
// the '#' line, which validateRuleSection rejects.
func TestParseFoundingFileRuleHeadingLine(t *testing.T) {
	// A single-'#' line stays inside the Rule section content (a "## " line
	// would instead start a new principle), so it reaches foundingBody verbatim.
	content := "## P1\n\nstmt\n\n## Rule\n\nreal rule\n# Heading\n"
	ps, err := parseFoundingFile(content)
	if err != nil {
		t.Fatalf("parseFoundingFile = %v", err)
	}
	body := foundingBody(ps[0].Statement, ps[0].Rule)
	if err := adr.ValidateBody([]byte(body), "founding"); err == nil {
		t.Fatal("composed founding body with heading-line rule validated; want error")
	}
}
