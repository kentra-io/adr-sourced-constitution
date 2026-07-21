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
// projects.
func newRecordADR(id string, num int, status adr.Status) adr.ADR {
	return adr.ADR{
		ID: id, Num: num, Title: "Title " + id, Date: "2026-07-01",
		Status: status, Path: "constitution/adr/" + id + "-x.md",
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

	_, err := render.Render(cfg, adrs)
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

	if _, err := render.Render(cfg, adrs); err == nil {
		t.Fatal("Render() error = nil, want an unknown-category error even for a non-active ADR")
	}
}

// TestRenderRecordOnlyDoesNotProject proves the curated-projection rule (plan
// §2.12): an active ADR with no "## Rule" section stays in the log but never
// appears in constitution.md.
func TestRenderRecordOnlyDoesNotProject(t *testing.T) {
	cfg := &config.Config{Categories: []string{"architecture"}}
	rule := newADR("ADR-0001", 1, "architecture", adr.StatusAccepted)
	record := newRecordADR("ADR-0002", 2, adr.StatusAccepted)

	out, err := render.Render(cfg, []adr.ADR{rule, record})
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
		newRecordADR("ADR-0001", 1, adr.StatusAccepted),
		newADR("ADR-0002", 2, "architecture", adr.StatusSuperseded), // rule-bearing but inactive
	}
	out, err := render.Render(cfg, adrs)
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
		newRecordADR("ADR-0002", 2, adr.StatusAccepted),           // record-only
	}
	out, err := render.Render(cfg, adrs)
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

	sections := render.Group(cfg, render.ActiveSet(adrs))

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

	sections := render.Group(cfg, []adr.ADR{a})
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

	out, err := render.Render(cfg, []adr.ADR{a})
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
	out, err := render.Render(cfg, []adr.ADR{*a})
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

	out, err := render.Render(cfg, []adr.ADR{a})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "ADR-0001 · 2026-07-01 · source FS-0042\n") {
		t.Errorf("expected metadata line with source segment, got:\n%s", out)
	}
}
