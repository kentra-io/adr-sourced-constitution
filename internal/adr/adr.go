// Package adr parses the ADR log — MADR-compliant, YAML-frontmatter
// Markdown files under constitution/adr/ — into an in-memory model, and
// exposes the schema/section validation the write path (M2) reuses.
//
// See adr-sourced-constitution.md §4/§5 for the record schema and
// immutability model, and implementation-plan.md §3 for the parsing
// approach (manual frontmatter split; yaml used to validate, never to
// rewrite).
package adr

import "strings"

// Status is an ADR's lifecycle state (spec §5). "proposed"/"rejected"
// never appear in the store — proposals are ephemeral and live only in
// conversation until accepted (spec §5.1) — so those are not modeled here.
type Status string

// The three statuses a stored ADR may carry (spec §5). "proposed" and
// "rejected" are deliberately absent — see the Status doc comment.
const (
	StatusAccepted   Status = "accepted"
	StatusSuperseded Status = "superseded"
	StatusDeprecated Status = "deprecated"
)

// MandatorySections are the MADR v4 body sections (spec §4.1, plan §1
// errata #1: Considered Options is mandatory, Consequences is optional).
// M2's `adr new` enforces all of these on write. M1's read path (Parse)
// only requires DecisionOutcomeSection, the sole section the projection
// renders — see requiredForRegen in parse.go.
var MandatorySections = []string{
	"Context and Problem Statement",
	"Considered Options",
	"Decision Outcome",
}

// DecisionOutcomeSection is the mandatory MADR body heading carrying the
// decision content. Since M5.5 (plan §2.12) it is no longer what the
// projection renders — RuleSection is — but it remains mandatory on every
// ADR (MADR v4; erratum #1).
const DecisionOutcomeSection = "Decision Outcome"

// RuleSection is the optional body heading whose presence makes an ADR
// rule-bearing (plan §2.12). Its content — a short normative statement — is
// what the constitution projection renders verbatim. An ADR without a Rule
// section is a catalog-only record: it stays in the log and never projects.
// An empty/whitespace-only Rule section is a validation error, so a present
// Rule section always carries content.
const RuleSection = "Rule"

// ADR is the parsed, validated in-memory model of one ADR record (spec
// §4.1). Body + all frontmatter except Status/SupersededBy are immutable
// once accepted (spec §5.2); this type is a read-only snapshot of a file.
type ADR struct {
	ID       string // "ADR-0007"
	Num      int    // 7, for numeric sort (plan §3)
	Title    string
	Category string
	Date     string // ISO-8601 "2026-07-01"; spec §4.1: date created, frozen
	Status   Status

	Source       string // "" when absent (sourceTracking type "none")
	Supersedes   string // "" unless this ADR supersedes another
	SupersededBy string // "" unless a later ADR superseded this one

	// Sections maps MADR body heading -> trimmed content, and SectionOrder
	// preserves the order headings appeared in the file. Exposed for M2's
	// full body-shape validation.
	Sections     map[string]string
	SectionOrder []string

	Path string // source file path, for error messages and diagnostics
}

// IsRuleBearing reports whether this ADR carries a non-empty "## Rule"
// section (plan §2.12). Only rule-bearing ADRs project into constitution.md;
// the parser guarantees a present Rule section is non-empty, so presence in
// Sections implies rule-bearing.
func (a *ADR) IsRuleBearing() bool {
	return a.Rule() != ""
}

// Rule returns the trimmed content of the ADR's "## Rule" section, or "" when
// the ADR is a catalog-only record with no Rule section.
func (a *ADR) Rule() string {
	return strings.TrimSpace(a.Sections[RuleSection])
}
