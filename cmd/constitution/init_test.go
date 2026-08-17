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

// foundingMADRBody is a minimal, valid MADR body (all three mandatory
// sections) around a Decision Outcome, optionally with a "## Rules" section
// appended — the shape --founding-file must carry now that init seeds the
// file verbatim as a single ADR body (milestone #4).
func foundingMADRBody(decision, rules string) string {
	body := "## Context and Problem Statement\n\n" +
		"Established at project bootstrap by `constitution init`.\n\n" +
		"## Considered Options\n\n" +
		"- Adopt this founding decision\n" +
		"- Leave the convention implicit\n\n" +
		"## Decision Outcome\n\n" +
		decision + "\n"
	if rules != "" {
		body += "\n## Rules\n\n" + rules + "\n"
	}
	return body
}

// TestInitSeedsOneFoundingADRFromBody proves init seeds exactly ONE founding
// ADR (ADR-0001) from --founding-file even when its "## Rules" section spans
// multiple categories with multiple rules each — one founding ADR carrying
// every rule, not one ADR per rule/category (the pre-v0.2 per-heading
// grammar this milestone deletes).
func TestInitSeedsOneFoundingADRFromBody(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	rules := "### testing\n\n#### tests-required\nEvery change ships with tests.\n\n" +
		"#### coverage-tracked\nCoverage is tracked per package.\n\n" +
		"### process\n\n#### small-prs\nKeep pull requests small.\n\n" +
		"#### review-required\nEvery change is reviewed before merge."
	founding := foundingMADRBody("Adopt disciplined engineering practices from day one.", rules)
	mustWriteFile(t, "founding.md", founding)

	err := runCLI(t, "init", "--category", "testing", "--category", "process", "--founding-file", "founding.md")
	if err != nil {
		t.Fatalf("init = %v, want nil", err)
	}

	entries, err := os.ReadDir(filepath.Join("constitution", "adr"))
	if err != nil {
		t.Fatalf("reading constitution/adr: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("constitution/adr has %d entries, want exactly 1 (one founding ADR)", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "ADR-0001-") {
		t.Errorf("founding ADR filename = %q, want an ADR-0001-* file", entries[0].Name())
	}

	adrs, err := adr.ParseDir(filepath.Join("constitution", "adr"))
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 || adrs[0].ID != "ADR-0001" {
		t.Fatalf("parsed log = %+v, want exactly ADR-0001", adrs)
	}
	if len(adrs[0].Rules) != 4 {
		t.Fatalf("ADR-0001 carries %d rules, want 4 (two categories, two rules each)", len(adrs[0].Rules))
	}
	cats := map[string]int{}
	for _, r := range adrs[0].Rules {
		cats[r.Category]++
	}
	if cats["testing"] != 2 || cats["process"] != 2 {
		t.Errorf("rule category counts = %+v, want testing:2 process:2", cats)
	}
}

// TestInitRejectsFoundingBodyMissingMandatorySection proves a founding file
// missing a mandatory MADR section (here "## Decision Outcome") is refused
// through the SAME body validator `adr new` uses (adr.ValidateBody), not an
// init-specific error path: exit 2, naming the missing section, nothing
// written.
func TestInitRejectsFoundingBodyMissingMandatorySection(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	founding := "## Context and Problem Statement\n\nx\n\n## Considered Options\n\n- a\n"
	mustWriteFile(t, "founding.md", founding)

	err := runCLI(t, "init", "--founding-file", "founding.md")
	if err == nil {
		t.Fatal("init(missing Decision Outcome) = nil, want error")
	}
	if got := exitCode(err); got != 2 {
		t.Errorf("exitCode = %d, want 2", got)
	}
	if !strings.Contains(err.Error(), `required section "## Decision Outcome" is missing`) {
		t.Errorf("error = %q, want it to name the missing section", err.Error())
	}
	entries, _ := os.ReadDir(filepath.Join("constitution", "adr"))
	if len(entries) != 0 {
		t.Errorf("constitution/adr has %d entries, want none seeded", len(entries))
	}
}

// TestInitFoundingUnknownCategory proves a seed rule filed under a category
// outside the just-chosen vocabulary refuses with nothing seeded (there is
// no --new-category at init).
func TestInitFoundingUnknownCategory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	founding := foundingMADRBody("Every change ships with tests.",
		"### testing\n\n#### mandatory-tests\nEvery change ships with tests.")
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
