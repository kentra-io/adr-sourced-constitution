package render_test

import (
	"strings"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
	"github.com/kentra-io/adr-sourced-constitution/internal/render"
)

// newADR builds a rule-bearing ADR carrying one rule filed under category
// (so it projects into constitution.md). Use newRecordADR for a
// record-only entry that must not project.
func newADR(id string, num int, category string, status adr.Status) adr.ADR {
	return adr.ADR{
		ID: id, Num: num, Title: "Title " + id, Date: "2026-07-01",
		Status: status, Path: "constitution/adr/" + id + "-x.md",
		Rules: []adr.Rule{{
			Category: category,
			Slug:     "rule-" + strings.ToLower(id),
			Text:     "Rule for " + id + ".",
		}},
		Sections: map[string]string{
			adr.DecisionOutcomeSection: "Outcome for " + id + ".",
		},
	}
}

// newRecordADR builds a record-only ADR: it has a Decision Outcome but no
// rules (hence no category anywhere), so it stays in the log and never
// projects. It always builds an ACTIVE (accepted) record; a future test
// needing a non-accepted record should re-add a status parameter rather
// than work around a fixed one.
func newRecordADR(id string, num int) adr.ADR {
	return adr.ADR{
		ID: id, Num: num, Title: "Title " + id, Date: "2026-07-01",
		Status: adr.StatusAccepted, Path: "constitution/adr/" + id + "-x.md",
		Sections: map[string]string{adr.DecisionOutcomeSection: "Outcome for " + id + "."},
	}
}

// TestUnknownCategory is the M1 malformed-ADR-table case that needs
// config to detect (spec §2.5's hard-error stance): regen fails when an
// ADR's category isn't in the project's configured vocabulary. This
// cross-check can't live in internal/adr (which never sees
// constitution.yml), hence it's asserted here rather than in
// internal/adr's parse_test.go table.
func TestUnknownCategory(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture", "code-style"}}
	adrs := []adr.ADR{newADR("ADR-0001", 1, "bogus", adr.StatusAccepted)}

	_, _, err := render.Render(cfg, adrs)
	if err == nil {
		t.Fatal("Render() error = nil, want an unknown-category error")
	}
	// The error names the offending rule (<category>/<slug>) so a multi-rule
	// ADR points the author at the exact entry.
	want := `constitution/adr/ADR-0001-x.md: field "category": rule bogus/rule-adr-0001: not in the configured category vocabulary [architecture, code-style] (got "bogus")`
	if err.Error() != want {
		t.Errorf("Render() error = %q, want %q", err.Error(), want)
	}
	if !strings.Contains(err.Error(), "bogus/rule-adr-0001") {
		t.Errorf("Render() error should cite the rule ref: %q", err.Error())
	}
}

// TestUnknownCategoryOnInactiveADR proves the category vocabulary check
// covers the whole log, not just the active set (implementation-plan.md:
// "the log is a validated input" in its entirety) — a superseded ADR
// with a bad category must still fail regen.
func TestUnknownCategoryOnInactiveADR(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture"}}
	adrs := []adr.ADR{newADR("ADR-0001", 1, "bogus", adr.StatusSuperseded)}

	if _, _, err := render.Render(cfg, adrs); err == nil {
		t.Fatal("Render() error = nil, want an unknown-category error even for a non-active ADR")
	}
}

// TestRenderRecordOnlyDoesNotProject proves the curated-projection rule (plan
// §2.12): an active ADR with no "## Rule" section stays in the log but never
// appears in constitution.md.
func TestRenderRecordOnlyDoesNotProject(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture"}}
	rule := newADR("ADR-0001", 1, "architecture", adr.StatusAccepted)
	record := newRecordADR("ADR-0002", 2)

	out, _, err := render.Render(cfg, []adr.ADR{rule, record})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Rule for ADR-0001.") {
		t.Errorf("rule-bearing ADR-0001 should project:\n%s", out)
	}
	if strings.Contains(string(out), "ADR-0002") {
		t.Errorf("record-only ADR-0002 must not project:\n%s", out)
	}
}

// TestRenderEmptyConstitutionPlaceholder proves the placeholder render (plan
// §2.12): when no active ADR is rule-bearing, constitution.md carries the
// header, the # Constitution heading, and one placeholder line — no category.
func TestRenderEmptyConstitutionPlaceholder(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture"}}
	adrs := []adr.ADR{
		newRecordADR("ADR-0001", 1),
		newADR("ADR-0002", 2, "architecture", adr.StatusSuperseded), // rule-bearing but inactive
	}
	out, _, err := render.Render(cfg, adrs)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "No standing rules yet. Decision log: constitution/adr/.\n") {
		t.Errorf("expected the placeholder line, got:\n%s", got)
	}
	if strings.Contains(got, "## architecture") {
		t.Errorf("no category heading should appear in the placeholder form:\n%s", got)
	}
}

// TestRenderOmitsRecordOnlyCategory proves category omission (plan §2.12): a
// category whose only active ADRs are record-only is dropped entirely.
func TestRenderOmitsRecordOnlyCategory(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture", "process"}}
	adrs := []adr.ADR{
		newADR("ADR-0001", 1, "architecture", adr.StatusAccepted), // projects
		newRecordADR("ADR-0002", 2),                               // record-only
	}
	out, _, err := render.Render(cfg, adrs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "## architecture") {
		t.Errorf("architecture (has a rule) should appear:\n%s", out)
	}
	if strings.Contains(string(out), "## process") {
		t.Errorf("process (record-only) must be omitted:\n%s", out)
	}
}

func TestActiveSet(t *testing.T) {
	adrs := []adr.ADR{
		newADR("ADR-0001", 1, "architecture", adr.StatusAccepted),
		newADR("ADR-0002", 2, "architecture", adr.StatusSuperseded),
		newADR("ADR-0003", 3, "architecture", adr.StatusDeprecated),
		newADR("ADR-0004", 4, "architecture", adr.StatusAccepted),
	}
	active := render.ActiveSet(adrs)
	if len(active) != 2 {
		t.Fatalf("len(ActiveSet) = %d, want 2", len(active))
	}
	if active[0].ID != "ADR-0001" || active[1].ID != "ADR-0004" {
		t.Errorf("ActiveSet ids = %q, %q, want ADR-0001, ADR-0004", active[0].ID, active[1].ID)
	}
}

// TestGroupOrder proves categories render in config order (not
// alphabetical, not first-seen) and ADRs sort numerically within a
// category (not by parse/file order) — the determinism contract
// (implementation-plan.md §3).
func TestGroupOrder(t *testing.T) {
	cfg := &config.Config{Categories: []string{"process", "architecture", "testing"}}
	// Deliberately out of numeric order and interleaved by category.
	adrs := []adr.ADR{
		newADR("ADR-0010", 10, "architecture", adr.StatusAccepted),
		newADR("ADR-0002", 2, "process", adr.StatusAccepted),
		newADR("ADR-0003", 3, "architecture", adr.StatusAccepted),
	}

	sections := render.Group(cfg, render.ActiveSet(adrs), nil)

	var gotNames []string
	for _, s := range sections {
		gotNames = append(gotNames, s.Name)
	}
	wantNames := []string{"process", "architecture"} // "testing" omitted: no active ADRs
	if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("section order = %v, want %v", gotNames, wantNames)
	}

	arch := sections[1]
	if arch.Entries[0].ADR.ID != "ADR-0003" || arch.Entries[1].ADR.ID != "ADR-0010" {
		t.Errorf("architecture entry order = %q, %q, want ADR-0003, ADR-0010 (numeric sort)",
			arch.Entries[0].ADR.ID, arch.Entries[1].ADR.ID)
	}
}

// TestGroupMultiRuleADR proves per-rule grouping: one ADR carrying rules in
// two categories contributes one entry to each, and two rules in the same
// category keep their file order.
func TestGroupMultiRuleADR(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture", "testing"}}
	a := adr.ADR{
		ID: "ADR-0001", Num: 1, Title: "Multi", Date: "2026-07-01",
		Status: adr.StatusAccepted, Path: "constitution/adr/ADR-0001-x.md",
		Rules: []adr.Rule{
			{Category: "testing", Slug: "first", Text: "First."},
			{Category: "architecture", Slug: "arch", Text: "Arch."},
			{Category: "testing", Slug: "second", Text: "Second."},
		},
	}

	sections := render.Group(cfg, []adr.ADR{a}, nil)
	if len(sections) != 2 || sections[0].Name != "architecture" || sections[1].Name != "testing" {
		t.Fatalf("sections = %+v, want [architecture testing]", sections)
	}
	if len(sections[0].Entries) != 1 || sections[0].Entries[0].Rule.Slug != "arch" {
		t.Errorf("architecture entries = %+v", sections[0].Entries)
	}
	testingSec := sections[1]
	if len(testingSec.Entries) != 2 || testingSec.Entries[0].Rule.Slug != "first" || testingSec.Entries[1].Rule.Slug != "second" {
		t.Errorf("testing entries out of file order: %+v", testingSec.Entries)
	}
}

// TestRenderOmitsSourceWhenAbsent covers the metadata-line normalization
// rule (implementation-plan.md §4): "omit source part when absent".
func TestRenderOmitsSourceWhenAbsent(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture"}}
	a := newADR("ADR-0001", 1, "architecture", adr.StatusAccepted)
	a.Source = "" // sourceTracking: none, or simply not set on this ADR

	out, _, err := render.Render(cfg, []adr.ADR{a})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "ADR-0001 · 2026-07-01\n") {
		t.Errorf("expected metadata line without a source segment, got:\n%s", out)
	}
	// "· source" targets the metadata segment specifically — the preamble
	// legitimately contains the word "source".
	if strings.Contains(string(out), "· source") {
		t.Errorf("expected no source segment when Source is empty, got:\n%s", out)
	}
}

// TestRenderCRLFInputIsLFOnly runs the full ParseBytes -> Render path on
// a CRLF-authored ADR and asserts the rendered constitution.md contains
// no \r bytes: the projection's bytes must not depend on the line
// endings an ADR author's editor happened to write.
func TestRenderCRLFInputIsLFOnly(t *testing.T) {
	crlf := strings.ReplaceAll(`---
id: ADR-0001
title: CRLF-authored rule
date: 2026-07-01
status: accepted
---

## Context and Problem Statement

Why.

## Considered Options

- Option

## Decision Outcome

The decision, at length.

## Rules

### architecture

#### crlf-rule

First line of the rule.
Second line of the rule.
`, "\n", "\r\n")

	a, err := adr.ParseBytes([]byte(crlf), "constitution/adr/ADR-0001-crlf-authored-rule.md")
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	cfg := &config.Config{Categories: []string{"architecture"}}
	out, _, err := render.Render(cfg, []adr.ADR{*a})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(string(out), "\r") {
		t.Errorf("rendered constitution.md contains \\r bytes from CRLF input:\n%q", out)
	}
	if !strings.Contains(string(out), "First line of the rule.\nSecond line of the rule.") {
		t.Errorf("rendered output missing the LF-normalized Rule body:\n%s", out)
	}
	if strings.Contains(string(out), "The decision, at length.") {
		t.Errorf("Decision Outcome must not project; only the Rule section does:\n%s", out)
	}
}

func TestRenderIncludesSourceWhenPresent(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture"}}
	a := newADR("ADR-0001", 1, "architecture", adr.StatusAccepted)
	a.Source = "FS-0042"

	out, _, err := render.Render(cfg, []adr.ADR{a})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "ADR-0001 · 2026-07-01 · source FS-0042\n") {
		t.Errorf("expected metadata line with source segment, got:\n%s", out)
	}
}

// ---- Fold tests (retirement / A6/A7) ------------------------------------

// cfgWith builds a config with the given category vocabulary.
func cfgWith(cats ...string) *config.Config {
	return &config.Config{Categories: cats}
}

// mkADR composes a complete, valid MADR file string — frontmatter
// (id/title/date/status + optional extra lines like "supersedes-rules:
// [...]"), the mandatory body sections, and an optional "## Rules" body —
// and parses it through the package's real parse pipeline
// (adr.ParseBytesUnnamed), so fold tests exercise ADRs exactly as the CLI
// would see them. The status goes straight into the frontmatter: all three
// stored statuses are valid on parse, and the fold reads the Status field.
func mkADR(t *testing.T, id string, status adr.Status, fmExtra, rulesBody string) adr.ADR {
	t.Helper()
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + id + "\n")
	b.WriteString("title: Title " + id + "\n")
	b.WriteString("date: 2026-07-01\n")
	b.WriteString("status: " + string(status) + "\n")
	if fmExtra != "" {
		b.WriteString(fmExtra + "\n")
	}
	b.WriteString("---\n\n")
	b.WriteString("## Context and Problem Statement\n\nWhy.\n\n")
	b.WriteString("## Considered Options\n\n- Option\n\n")
	b.WriteString("## Decision Outcome\n\nOutcome for " + id + ".\n")
	if rulesBody != "" {
		b.WriteString("\n## Rules\n\n" + rulesBody + "\n")
	}

	path := "constitution/adr/" + id + "-x.md"
	a, err := adr.ParseBytesUnnamed([]byte(b.String()), path)
	if err != nil {
		t.Fatalf("mkADR(%s): %v", id, err)
	}
	return *a
}

// TestFoldRetirementMasksRule is the core A6 fold: an accepted ADR's
// supersedes-rules ref masks exactly the targeted rule of the earlier ADR;
// the earlier ADR's other rules and the retirer's own rules still project.
func TestFoldRetirementMasksRule(t *testing.T) {
	a2 := mkADR(t, "ADR-0002", adr.StatusAccepted, "",
		"### testing\n\n#### old-tiers\nOld.\n\n#### keep-me\nKeep.")
	a9 := mkADR(t, "ADR-0009", adr.StatusAccepted,
		"supersedes-rules: [ADR-0002/testing/old-tiers]",
		"### testing\n\n#### new-tiers\nNew.")

	out, warns, err := render.Render(cfgWith("testing"), []adr.ADR{a2, a9})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("Render() warnings = %q, want none", warns)
	}
	got := string(out)
	if !strings.Contains(got, "### keep-me") {
		t.Errorf("unretired rule keep-me must still project:\n%s", got)
	}
	if !strings.Contains(got, "### new-tiers") {
		t.Errorf("the retirer's own rule new-tiers must project:\n%s", got)
	}
	if strings.Contains(got, "Old.") || strings.Contains(got, "### old-tiers") {
		t.Errorf("retired rule old-tiers must be masked:\n%s", got)
	}
}

// TestFoldRemovesRulesMasksToo proves removes-rules has the identical fold
// effect (A6: the verb documents intent, the mechanics are the same).
func TestFoldRemovesRulesMasksToo(t *testing.T) {
	a2 := mkADR(t, "ADR-0002", adr.StatusAccepted, "",
		"### testing\n\n#### old-tiers\nOld.")
	a9 := mkADR(t, "ADR-0009", adr.StatusAccepted,
		"removes-rules: [ADR-0002/testing/old-tiers]", "")

	out, warns, err := render.Render(cfgWith("testing"), []adr.ADR{a2, a9})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("Render() warnings = %q, want none", warns)
	}
	if strings.Contains(string(out), "Old.") {
		t.Errorf("removes-rules-retired rule must be masked:\n%s", out)
	}
}

func TestFoldDanglingRefIsError(t *testing.T) {
	a9 := mkADR(t, "ADR-0009", adr.StatusAccepted,
		"supersedes-rules: [ADR-0002/testing/ghost]",
		"### testing\n\n#### x\nX.")

	_, _, err := render.Render(cfgWith("testing"), []adr.ADR{a9})
	if err == nil {
		t.Fatal("Render() error = nil, want a dangling-ref error")
	}
	want := `retirement ref "ADR-0002/testing/ghost" does not resolve to any rule in the log`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Render() error = %q, want it to contain %q", err.Error(), want)
	}
	if !strings.Contains(err.Error(), `field "supersedes-rules"`) {
		t.Errorf("Render() error should name the originating list: %q", err.Error())
	}
}

// TestFoldDanglingRefAnyStatus proves dangling detection resolves against
// the WHOLE log (any status): a ref to a rule of a superseded ADR is not
// dangling.
func TestFoldDanglingRefAnyStatus(t *testing.T) {
	a2 := mkADR(t, "ADR-0002", adr.StatusSuperseded, "",
		"### testing\n\n#### old-tiers\nOld.")
	a9 := mkADR(t, "ADR-0009", adr.StatusAccepted,
		"supersedes-rules: [ADR-0002/testing/old-tiers]",
		"### testing\n\n#### new-tiers\nNew.")

	_, warns, err := render.Render(cfgWith("testing"), []adr.ADR{a2, a9})
	if err != nil {
		t.Fatalf("Render() error = %v, want nil (target exists, just not accepted)", err)
	}
	if len(warns) != 0 {
		t.Errorf("Render() warnings = %q, want none", warns)
	}
}

func TestFoldForwardOrSelfRefIsError(t *testing.T) {
	t.Run("forward", func(t *testing.T) {
		// ADR-0002 tries to retire a rule of the LATER ADR-0009.
		a2 := mkADR(t, "ADR-0002", adr.StatusAccepted,
			"supersedes-rules: [ADR-0009/testing/new-tiers]",
			"### testing\n\n#### old-tiers\nOld.")
		a9 := mkADR(t, "ADR-0009", adr.StatusAccepted, "",
			"### testing\n\n#### new-tiers\nNew.")

		_, _, err := render.Render(cfgWith("testing"), []adr.ADR{a2, a9})
		if err == nil {
			t.Fatal("Render() error = nil, want a forward-ref error")
		}
		if !strings.Contains(err.Error(), "may only retire rules of an earlier ADR") {
			t.Errorf("Render() error = %q, want the earlier-ADR message", err.Error())
		}
	})
	t.Run("self", func(t *testing.T) {
		// ADR-0009 tries to retire its own rule.
		a9 := mkADR(t, "ADR-0009", adr.StatusAccepted,
			"supersedes-rules: [ADR-0009/testing/new-tiers]",
			"### testing\n\n#### new-tiers\nNew.")

		_, _, err := render.Render(cfgWith("testing"), []adr.ADR{a9})
		if err == nil {
			t.Fatal("Render() error = nil, want a self-ref error")
		}
		if !strings.Contains(err.Error(), "may only retire rules of an earlier ADR") {
			t.Errorf("Render() error = %q, want the earlier-ADR message", err.Error())
		}
	})
}

func TestFoldDoubleRetireIsError(t *testing.T) {
	a2 := mkADR(t, "ADR-0002", adr.StatusAccepted, "",
		"### testing\n\n#### old-tiers\nOld.")
	a8 := mkADR(t, "ADR-0008", adr.StatusAccepted,
		"supersedes-rules: [ADR-0002/testing/old-tiers]",
		"### testing\n\n#### mid-tiers\nMid.")
	a9 := mkADR(t, "ADR-0009", adr.StatusAccepted,
		"removes-rules: [ADR-0002/testing/old-tiers]", "")

	_, _, err := render.Render(cfgWith("testing"), []adr.ADR{a2, a8, a9})
	if err == nil {
		t.Fatal("Render() error = nil, want a double-retire error")
	}
	if !strings.Contains(err.Error(), `already retired by ADR-0008`) {
		t.Errorf("Render() error = %q, want it to name the first retirer (already retired by ADR-0008)", err.Error())
	}
}

// TestFoldIntraADRDuplicateRef: the same ref listed twice within ONE ADR's
// supersedes-rules is rejected (carried Task-3 review item).
func TestFoldIntraADRDuplicateRef(t *testing.T) {
	a2 := mkADR(t, "ADR-0002", adr.StatusAccepted, "",
		"### testing\n\n#### old-tiers\nOld.")
	a9 := mkADR(t, "ADR-0009", adr.StatusAccepted,
		"supersedes-rules: [ADR-0002/testing/old-tiers, ADR-0002/testing/old-tiers]", "")

	_, _, err := render.Render(cfgWith("testing"), []adr.ADR{a2, a9})
	if err == nil {
		t.Fatal("Render() error = nil, want an intra-ADR duplicate error")
	}
	want := `retirement ref "ADR-0002/testing/old-tiers": listed more than once by ADR-0009`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Render() error = %q, want it to contain %q", err.Error(), want)
	}
}

// TestFoldCrossListDuplicateRef: one ADR listing the same ref in BOTH
// supersedes-rules and removes-rules is rejected; the error names the list
// the second occurrence came from.
func TestFoldCrossListDuplicateRef(t *testing.T) {
	a2 := mkADR(t, "ADR-0002", adr.StatusAccepted, "",
		"### testing\n\n#### old-tiers\nOld.")
	a9 := mkADR(t, "ADR-0009", adr.StatusAccepted,
		"supersedes-rules: [ADR-0002/testing/old-tiers]\nremoves-rules: [ADR-0002/testing/old-tiers]", "")

	_, _, err := render.Render(cfgWith("testing"), []adr.ADR{a2, a9})
	if err == nil {
		t.Fatal("Render() error = nil, want a cross-list duplicate error")
	}
	if !strings.Contains(err.Error(), "listed more than once by ADR-0009") {
		t.Errorf("Render() error = %q, want the listed-more-than-once message", err.Error())
	}
	if !strings.Contains(err.Error(), `field "removes-rules"`) {
		t.Errorf("Render() error should name the list the duplicate occurrence came from: %q", err.Error())
	}
}

// TestFoldResurrectionWarns is A7: a no-longer-accepted retirer's
// retirements stop applying — the target rule projects again, with exactly
// one warning naming the retirer and the resurrected ref.
func TestFoldResurrectionWarns(t *testing.T) {
	a2 := mkADR(t, "ADR-0002", adr.StatusAccepted, "",
		"### testing\n\n#### old-tiers\nOld.")
	a9 := mkADR(t, "ADR-0009", adr.StatusSuperseded,
		"supersedes-rules: [ADR-0002/testing/old-tiers]",
		"### testing\n\n#### new-tiers\nNew.")

	out, warns, err := render.Render(cfgWith("testing"), []adr.ADR{a2, a9})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "Old.") {
		t.Errorf("resurrected rule old-tiers must project again:\n%s", got)
	}
	if strings.Contains(got, "New.") {
		t.Errorf("superseded ADR-0009's own rule must not project:\n%s", got)
	}
	if len(warns) != 1 {
		t.Fatalf("Render() warnings = %q, want exactly one", warns)
	}
	w := warns[0]
	if !strings.Contains(w, "ADR-0009") || !strings.Contains(w, "ADR-0002/testing/old-tiers") {
		t.Errorf("warning must name retirer and ref: %q", w)
	}
	if !strings.Contains(w, "resurrect") {
		t.Errorf("warning must say the rule is resurrected: %q", w)
	}
}

// TestFoldResurrectionSuppressedWhenReRetired: no warning when some
// ACCEPTED ADR also retires the same ref — the rule does not actually
// become visible, so there is nothing to warn about.
func TestFoldResurrectionSuppressedWhenReRetired(t *testing.T) {
	a2 := mkADR(t, "ADR-0002", adr.StatusAccepted, "",
		"### testing\n\n#### old-tiers\nOld.")
	a9 := mkADR(t, "ADR-0009", adr.StatusSuperseded,
		"supersedes-rules: [ADR-0002/testing/old-tiers]",
		"### testing\n\n#### new-tiers\nNew.")
	a10 := mkADR(t, "ADR-0010", adr.StatusAccepted,
		"supersedes-rules: [ADR-0002/testing/old-tiers]",
		"### testing\n\n#### newest-tiers\nNewest.")

	out, warns, err := render.Render(cfgWith("testing"), []adr.ADR{a2, a9, a10})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(string(out), "Old.") {
		t.Errorf("re-retired rule must stay masked:\n%s", out)
	}
	if len(warns) != 0 {
		t.Errorf("Render() warnings = %q, want none (ref re-retired by accepted ADR-0010)", warns)
	}
}

// TestRenderPopulatedPointsAtTheDecisionLog is issue #24: the EMPTY
// projection tells the reader where the decision log lives, the
// populated one did not. Since only rule-bearing ADRs project, every
// record-only ADR — the deliberate deferrals, the "we have not
// decided X yet" records — was invisible to anyone reading
// constitution.md, which is the artefact agents @-import.
func TestRenderPopulatedPointsAtTheDecisionLog(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture"}}
	adrs := []adr.ADR{
		newADR("ADR-0001", 1, "architecture", adr.StatusAccepted),
		newRecordADR("ADR-0002", 2), // record-only: never projects
	}

	out, _, err := render.Render(cfg, adrs)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	if !strings.Contains(got, "\nDecision log: constitution/adr/.\n") {
		t.Errorf("populated projection carries no decision-log pointer:\n%s", got)
	}
	// The pointer orients the reader, so it belongs with the preamble,
	// not trailing a rule list of unbounded length.
	if strings.Index(got, "Decision log:") > strings.Index(got, "## architecture") {
		t.Errorf("pointer must precede the first category heading:\n%s", got)
	}
	if strings.Contains(got, "No standing rules yet") {
		t.Errorf("populated projection must not carry the placeholder text:\n%s", got)
	}
}
