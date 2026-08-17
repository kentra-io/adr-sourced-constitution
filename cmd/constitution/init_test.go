package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
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

// TestInitWritesConfiguredSourceTracking proves `init --source-tracking
// github-issue` writes that value straight into constitution.yml — no
// post-init hand-edit needed to enable source tracking (issue #17/#20).
func TestInitWritesConfiguredSourceTracking(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := runCLI(t, "init", "--source-tracking", "github-issue"); err != nil {
		t.Fatalf("init --source-tracking github-issue = %v, want nil", err)
	}

	cfg, err := config.Load(filepath.Join(dir, "constitution.yml"))
	if err != nil {
		t.Fatalf("reloading constitution.yml: %v", err)
	}
	if cfg.SourceTracking.Type != config.SourceTrackingGitHubIssue {
		t.Errorf("sourceTracking.type = %q, want %q", cfg.SourceTracking.Type, config.SourceTrackingGitHubIssue)
	}
}

// TestInitRejectsUnknownSourceTrackingValue proves the illegal near-miss
// from issue #17 ("github" instead of "github-issue") is refused at exit 2,
// naming the four legal values, with no constitution.yml written at all.
func TestInitRejectsUnknownSourceTrackingValue(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := runCLI(t, "init", "--source-tracking", "github")
	if err == nil {
		t.Fatal("init --source-tracking github = nil, want error")
	}
	if got := exitCode(err); got != 2 {
		t.Errorf("exitCode = %d, want 2", got)
	}
	for _, v := range []string{`"none"`, `"generic"`, `"github-issue"`, `"jira"`} {
		if !strings.Contains(err.Error(), v) {
			t.Errorf("error = %q, want it to name legal value %s", err, v)
		}
	}
	if _, statErr := os.Stat(filepath.Join(dir, "constitution.yml")); !os.IsNotExist(statErr) {
		t.Errorf("constitution.yml exists after a refused init, want nothing written (stat err = %v)", statErr)
	}
}

// TestInitRejectsSourcePatternWithoutTracking proves --source-pattern given
// with no --source-tracking (or with it explicitly "none") is refused at
// exit 2 — a pattern is meaningless with nothing to check it against — and
// nothing is written.
func TestInitRejectsSourcePatternWithoutTracking(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unset", []string{"init", "--source-pattern", `#\d+`}},
		{"explicit-none", []string{"init", "--source-tracking", "none", "--source-pattern", `#\d+`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			err := runCLI(t, c.args...)
			if err == nil {
				t.Fatalf("init(%v) = nil, want error", c.args)
			}
			if got := exitCode(err); got != 2 {
				t.Errorf("exitCode = %d, want 2", got)
			}
			if !strings.Contains(err.Error(), "source-pattern") || !strings.Contains(err.Error(), `"none"`) {
				t.Errorf("error = %q, want it to explain --source-pattern is meaningless under type none", err)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "constitution.yml")); !os.IsNotExist(statErr) {
				t.Errorf("constitution.yml exists after a refused init, want nothing written (stat err = %v)", statErr)
			}
		})
	}
}

// TestInitReinitNoticesIgnoredSourceTrackingFlags proves a re-run against a
// repo that already has a constitution.yml reports --source-tracking (and
// --source-pattern) among the ignored flags — the existing config still
// wins, but the ignoring is honest rather than silent. Without this, the
// illegal near-miss from issue #17 (--source-tracking github) would exit 0
// on a re-run with no signal at all, even though nothing was written.
func TestInitReinitNoticesIgnoredSourceTrackingFlags(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := runCLI(t, "init"); err != nil {
		t.Fatalf("initial init = %v, want nil", err)
	}
	before := mustReadFile(t, "constitution.yml")

	// runCLI (new_test.go) drives run(), which lets cli/v3 default ErrWriter
	// to the real os.Stderr — fine for the other tests here, which only
	// check exit codes and disk state, but this test needs to inspect the
	// notice text, so it builds its own root with ErrWriter captured
	// (same pattern config_test.go's runConfigSchemaCLI uses for stdout).
	var stderr bytes.Buffer
	root := &cli.Command{
		Name:      "constitution",
		Writer:    io.Discard,
		ErrWriter: &stderr,
		Commands:  []*cli.Command{initCommand()},
	}
	if err := root.Run(context.Background(), []string{"constitution", "init", "--source-tracking", "github"}); err != nil {
		t.Fatalf("re-run init --source-tracking github = %v, want nil (existing config wins, not an error)", err)
	}
	if !strings.Contains(stderr.String(), "ignoring") || !strings.Contains(stderr.String(), "--source-tracking") {
		t.Errorf("stderr = %q, want a notice naming --source-tracking as ignored", stderr.String())
	}
	if after := mustReadFile(t, "constitution.yml"); after != before {
		t.Errorf("constitution.yml changed on a re-run despite existing config winning:\nbefore: %s\nafter:  %s", before, after)
	}
}
