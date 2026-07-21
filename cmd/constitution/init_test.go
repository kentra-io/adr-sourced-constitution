package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// TestParseFoundingFileValid confirms the happy path: one principle per
// "## " heading, with the statement falling back to the title when the body
// is empty.
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
	if ps[0].HasRules {
		t.Errorf("ps[0].HasRules = true, want catalog-only")
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

// TestParseFoundingFileRulesAttachment proves a "## Rules" heading attaches
// its content verbatim to the PRECEDING principle, and that a following
// principle starts fresh (catalog-only).
func TestParseFoundingFileRulesAttachment(t *testing.T) {
	content := "## Tests are mandatory\n\nEvery change ships with tests.\n\n" +
		"## Rules\n\n### testing\n\n#### mandatory-tests\n\nEvery change ships with tests.\n\n" +
		"## Small focused PRs\n\nKeep PRs small.\n"
	ps, err := parseFoundingFile(content)
	if err != nil {
		t.Fatalf("parseFoundingFile(rules attachment) = %v, want nil", err)
	}
	if len(ps) != 2 {
		t.Fatalf("got %d principles, want 2", len(ps))
	}
	if !ps[0].HasRules {
		t.Fatal("ps[0].HasRules = false, want the Rules section attached")
	}
	wantRules := "### testing\n\n#### mandatory-tests\n\nEvery change ships with tests."
	if ps[0].Rules != wantRules {
		t.Errorf("ps[0].Rules = %q, want %q", ps[0].Rules, wantRules)
	}
	if ps[0].Statement != "Every change ships with tests." {
		t.Errorf("ps[0].Statement = %q", ps[0].Statement)
	}
	if ps[1].HasRules || ps[1].Title != "Small focused PRs" {
		t.Errorf("ps[1] = %+v, want a catalog-only principle", ps[1])
	}
}

// TestParseFoundingFileRulesFirst proves a "## Rules" with no preceding
// principle is a hard error.
func TestParseFoundingFileRulesFirst(t *testing.T) {
	_, err := parseFoundingFile("## Rules\n\n### testing\n\n#### x\n\ntext\n")
	if err == nil {
		t.Fatal("parseFoundingFile(Rules first) = nil, want error")
	}
	if !strings.Contains(err.Error(), `"## Rules" cannot be the first heading`) {
		t.Errorf("error = %q, want the Rules-first rejection", err.Error())
	}
}

// TestParseFoundingFileRulesAfterRules proves two consecutive "## Rules"
// sections are a hard error (each principle carries at most one).
func TestParseFoundingFileRulesAfterRules(t *testing.T) {
	content := "## A principle\n\ntext\n\n## Rules\n\n### testing\n\n#### x\n\nt\n\n## Rules\n\n### testing\n\n#### y\n\nt\n"
	_, err := parseFoundingFile(content)
	if err == nil {
		t.Fatal("parseFoundingFile(Rules after Rules) = nil, want error")
	}
	if !strings.Contains(err.Error(), `directly after another "## Rules"`) {
		t.Errorf("error = %q, want the Rules-after-Rules rejection", err.Error())
	}
}

// TestFoundingBodyIsValidMADR proves composed founding bodies — catalog-only
// and rule-bearing — pass the same write-path validation `adr new` applies.
func TestFoundingBodyIsValidMADR(t *testing.T) {
	catalog := foundingBody(principle{Statement: "Adopt the thing."})
	if err := adr.ValidateBody([]byte(catalog), "founding"); err != nil {
		t.Fatalf("catalog-only foundingBody does not validate: %v", err)
	}
	ruled := foundingBody(principle{
		Statement: "Adopt the thing.",
		Rules:     "### testing\n\n#### adopt-thing\n\nAdopt the thing.",
		HasRules:  true,
	})
	if err := adr.ValidateBody([]byte(ruled), "founding"); err != nil {
		t.Fatalf("rule-bearing foundingBody does not validate: %v", err)
	}
	// A Rules heading that was present but empty must compose an INVALID
	// body (the grammar rejects an empty section) rather than silently
	// seeding a record-only ADR.
	empty := foundingBody(principle{Statement: "Adopt.", HasRules: true})
	if err := adr.ValidateBody([]byte(empty), "founding"); err == nil {
		t.Fatal("empty-Rules foundingBody validates, want the empty-section rejection")
	}
}

// TestInitFoundingRulesSeed drives `init` end-to-end: a founding file with a
// rule-bearing principle and a catalog-only one seeds two ADRs, and the
// rule projects into constitution.md.
func TestInitFoundingRulesSeed(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	founding := "## Tests are mandatory\n\nEvery change ships with tests.\n\n" +
		"## Rules\n\n### testing\n\n#### mandatory-tests\n\nEvery change ships with tests.\n\n" +
		"## Small focused PRs\n\nKeep PRs small.\n"
	mustWriteFile(t, "founding.md", founding)

	err := runCLI(t, "init", "--category", "testing", "--founding-file", "founding.md")
	if err != nil {
		t.Fatalf("init = %v, want nil", err)
	}

	first := mustReadFile(t, filepath.Join("constitution", "adr", "ADR-0001-tests-are-mandatory.md"))
	if !strings.Contains(first, "## Rules") || !strings.Contains(first, "#### mandatory-tests") {
		t.Errorf("ADR-0001 missing the Rules section:\n%s", first)
	}
	second := mustReadFile(t, filepath.Join("constitution", "adr", "ADR-0002-small-focused-prs.md"))
	if strings.Contains(second, "## Rules") {
		t.Errorf("ADR-0002 should be catalog-only:\n%s", second)
	}
	con := mustReadFile(t, filepath.Join("constitution", "constitution.md"))
	if !strings.Contains(con, "Every change ships with tests.") {
		t.Errorf("constitution.md missing the projected rule:\n%s", con)
	}
}

// TestInitFoundingUnknownCategory proves a seed rule filed under a category
// outside the just-chosen vocabulary refuses with nothing seeded (there is
// no --new-category at init).
func TestInitFoundingUnknownCategory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	founding := "## Tests are mandatory\n\nEvery change ships with tests.\n\n" +
		"## Rules\n\n### testing\n\n#### mandatory-tests\n\nEvery change ships with tests.\n"
	mustWriteFile(t, "founding.md", founding)

	err := runCLI(t, "init", "--category", "architecture", "--founding-file", "founding.md")
	if err == nil {
		t.Fatal("init(unknown founding category) = nil, want error")
	}
	if !strings.Contains(err.Error(), `rule category "testing" is not in the configured vocabulary`) {
		t.Errorf("error = %q, want the vocabulary rejection", err.Error())
	}
	if strings.Contains(err.Error(), "--new-category") {
		t.Errorf("error = %q, must not hint at --new-category (init has none)", err.Error())
	}
	entries, _ := os.ReadDir(filepath.Join("constitution", "adr"))
	if len(entries) != 0 {
		t.Errorf("constitution/adr has %d entries, want none seeded", len(entries))
	}
}
