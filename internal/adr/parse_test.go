package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestParseWellFormed is a control case: a valid ADR parses into the
// expected model, with no error.
func TestParseWellFormed(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "ADR-0001-use-gofmt.md", `---
id: ADR-0001
title: Format all Go code with gofmt
date: 2026-07-01
status: accepted
source: FS-0042
---

## Context and Problem Statement

Inconsistent formatting produces noisy diffs.

## Considered Options

- No enforced formatter
- gofmt

## Decision Outcome

All Go code must be formatted with gofmt.

## Consequences

None significant.
`)

	got, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	if got.ID != "ADR-0001" || got.Num != 1 {
		t.Errorf("ID/Num = %q/%d, want ADR-0001/1", got.ID, got.Num)
	}
	if got.Title != "Format all Go code with gofmt" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Status != StatusAccepted {
		t.Errorf("Status = %q", got.Status)
	}
	if got.Source != "FS-0042" {
		t.Errorf("Source = %q", got.Source)
	}
	want := "All Go code must be formatted with gofmt."
	if got.Sections[DecisionOutcomeSection] != want {
		t.Errorf("Sections[Decision Outcome] = %q, want %q", got.Sections[DecisionOutcomeSection], want)
	}
}

// TestParseRuleBearing proves the rule-bearing model (proposal D5/A1): a
// "## Rules" section is optional, and its presence with valid entries makes
// the ADR rule-bearing while its absence makes the ADR a record-only entry.
func TestParseRuleBearing(t *testing.T) {
	dir := t.TempDir()

	ruleBody := `---
id: ADR-0001
title: A standing rule
date: 2026-07-01
status: accepted
---

## Context and Problem Statement

Why.

## Considered Options

- A

## Decision Outcome

The full decision, at length.

## Rules

### architecture

#### do-the-thing

Do the thing; do not do the other thing.
`
	rulePath := writeFile(t, dir, "ADR-0001-a-standing-rule.md", ruleBody)
	a, err := Parse(rulePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !a.IsRuleBearing() {
		t.Error("ADR with a ## Rules section should be rule-bearing")
	}
	want := Rule{Category: "architecture", Slug: "do-the-thing", Text: "Do the thing; do not do the other thing."}
	if len(a.Rules) != 1 || a.Rules[0] != want {
		t.Errorf("Rules = %+v, want [%+v]", a.Rules, want)
	}

	recordBody := `---
id: ADR-0002
title: A catalog record
date: 2026-07-01
status: accepted
---

## Context and Problem Statement

Why.

## Considered Options

- A

## Decision Outcome

A point-in-time record with no standing rule.
`
	recordPath := writeFile(t, dir, "ADR-0002-a-catalog-record.md", recordBody)
	rec, err := Parse(recordPath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if rec.IsRuleBearing() {
		t.Error("ADR with no ## Rules section must be a record-only entry")
	}
	if len(rec.Rules) != 0 {
		t.Errorf("Rules = %+v, want empty", rec.Rules)
	}
}

// TestParseMalformed is the M1 DoD malformed-ADR table: each case
// asserts the *exact* error message contract (file, line where
// determinable, field) — errors are UX here, per implementation-plan.md
// §8 ("agents parse them").
func TestParseMalformed(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		// wantErr is the exact Error() string ({file} is substituted for
		// the temp path). If wantErrPrefix is set instead, the error is
		// asserted as prefix + substring — used only where part of the
		// message is sourced from the yaml library, whose exact wording a
		// patch bump may change; our own wrapper text stays exact.
		wantErr         string
		wantErrPrefix   string
		wantErrContains string
	}{
		{
			name:     "missing frontmatter",
			filename: "ADR-0001-no-frontmatter.md",
			content:  "id: ADR-0001\ntitle: No delimiter\n",
			wantErr:  `{file}:1: file must start with a "---" frontmatter delimiter line`,
		},
		{
			name:     "unterminated frontmatter",
			filename: "ADR-0001-unterminated.md",
			content:  "---\nid: ADR-0001\ntitle: No closing delimiter\n",
			wantErr:  `{file}:1: frontmatter is not terminated: no closing "---" delimiter line found`,
		},
		{
			name:     "bad yaml",
			filename: "ADR-0002-bad-yaml.md",
			content: `---
id: ADR-0002
title: [unterminated flow sequence
date: 2026-07-01
status: accepted
---

## Decision Outcome

Body.
`,
			// Our wrapper prefix (file:line + "frontmatter is not valid
			// YAML:") is the exact contract; the trailing detail is
			// library-sourced, so only a stable substring is pinned.
			wantErrPrefix:   `{file}:2: frontmatter is not valid YAML: `,
			wantErrContains: `yaml:`,
		},
		{
			name:     "missing required field",
			filename: "ADR-0003-missing-title.md",
			content: `---
id: ADR-0003
date: 2026-07-01
status: accepted
---

## Decision Outcome

Body.
`,
			wantErr: `{file}: field "title": is required`,
		},
		{
			name:     "bad id format",
			filename: "ADR-0004-bad-id.md",
			content: `---
id: ADR-7
title: Bad id format
date: 2026-07-01
status: accepted
---

## Decision Outcome

Body.
`,
			wantErr: `{file}:2: field "id": must match "ADR-NNNN" (got "ADR-7")`,
		},
		{
			name:     "unknown status",
			filename: "ADR-0005-unknown-status.md",
			content: `---
id: ADR-0005
title: Unknown status
date: 2026-07-01
status: proposed
---

## Decision Outcome

Body.
`,
			wantErr: `{file}:5: field "status": must be one of "accepted", "superseded", "deprecated" (got "proposed")`,
		},
		{
			name:     "missing Decision Outcome",
			filename: "ADR-0006-missing-decision-outcome.md",
			content: `---
id: ADR-0006
title: Missing decision outcome section
date: 2026-07-01
status: accepted
---

## Context and Problem Statement

Why.

## Considered Options

Options.
`,
			wantErr: `{file}: field "Decision Outcome": required section "## Decision Outcome" is missing`,
		},
		{
			name:     "bad date format",
			filename: "ADR-0007-bad-date.md",
			content: `---
id: ADR-0007
title: Bad date format
date: 07/01/2026
status: accepted
---

## Decision Outcome

Body.
`,
			wantErr: `{file}:4: field "date": must be an ISO-8601 date YYYY-MM-DD (got "07/01/2026")`,
		},
		{
			name:     "id does not match filename",
			filename: "ADR-0008-mismatched-id.md",
			content: `---
id: ADR-0009
title: Frontmatter id does not match filename
date: 2026-07-01
status: accepted
---

## Decision Outcome

Body.
`,
			wantErr: `{file}:2: field "id": frontmatter id "ADR-0009" does not match filename-derived id "ADR-0008"`,
		},
		{
			name:     "empty Rules section",
			filename: "ADR-0009-empty-rules.md",
			content: `---
id: ADR-0009
title: Empty rules section
date: 2026-07-01
status: accepted
---

## Decision Outcome

Body.

## Rules

` + "   " + `
`,
			wantErr: `{file}: field "Rules": the "## Rules" section is present but empty; give it "### <category>" / "#### <slug>" rule entries or remove it (a record-only ADR has no Rules section)`,
		},
		{
			name:     "duplicate Rules section",
			filename: "ADR-0009-duplicate-rules.md",
			content: `---
id: ADR-0009
title: Duplicate rules section
date: 2026-07-01
status: accepted
---

## Decision Outcome

Body.

## Rules

### testing

#### first-rule

First rule.

## Rules

### testing

#### second-rule

Second rule.
`,
			wantErr: `{file}: field "Rules": the "## Rules" section appears more than once; a body may carry at most one`,
		},
		{
			name:     "heading line in a rule text",
			filename: "ADR-0009-heading-in-rule.md",
			content: `---
id: ADR-0009
title: Heading in rule text
date: 2026-07-01
status: accepted
---

## Decision Outcome

Body.

## Rules

### testing

#### real-rule

real rule
# Big Heading
`,
			wantErr: `{file}: field "Rules": rule text is plain prose and must not contain Markdown heading lines; found: # Big Heading`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, tt.filename, tt.content)

			_, err := Parse(path)
			if err == nil {
				t.Fatalf("Parse() error = nil, want error matching %q%q", tt.wantErr, tt.wantErrPrefix)
			}

			if tt.wantErrPrefix != "" {
				prefix := strings.ReplaceAll(tt.wantErrPrefix, "{file}", path)
				if !strings.HasPrefix(err.Error(), prefix) {
					t.Errorf("Parse() error = %q, want prefix %q", err.Error(), prefix)
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("Parse() error = %q, want it to contain %q", err.Error(), tt.wantErrContains)
				}
				return
			}

			want := strings.ReplaceAll(tt.wantErr, "{file}", path)
			if err.Error() != want {
				t.Errorf("Parse() error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestParseBytesRulesModel(t *testing.T) {
	src := []byte(`---
id: ADR-0004
title: Testing discipline
date: 2026-07-21
status: accepted
supersedes-rules: [ADR-0002/testing/old-tiers]
removes-rules: [ADR-0002/testing/no-mutation]
---

## Context and Problem Statement

Why.

## Considered Options

* one

## Decision Outcome

Chosen.

## Rules

### testing

#### three-tier-tests
Three tiers.

#### no-mocking-unowned
Never mock types you don't own.

### architecture

#### hex-core
Hexagonal.
`)
	a, err := ParseBytes(src, "ADR-0004-testing-discipline.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.IsRuleBearing() || len(a.Rules) != 3 {
		t.Fatalf("Rules = %+v", a.Rules)
	}
	if a.Rules[2] != (Rule{Category: "architecture", Slug: "hex-core", Text: "Hexagonal."}) {
		t.Fatalf("rule order/content wrong: %+v", a.Rules[2])
	}
	if len(a.SupersedesRules) != 1 || a.SupersedesRules[0].String() != "ADR-0002/testing/old-tiers" {
		t.Fatalf("SupersedesRules = %+v", a.SupersedesRules)
	}
	if len(a.RemovesRules) != 1 || a.RemovesRules[0].String() != "ADR-0002/testing/no-mutation" {
		t.Fatalf("RemovesRules = %+v", a.RemovesRules)
	}
}

func TestParseBytesBadRuleRefInFrontmatter(t *testing.T) {
	src := []byte("---\nid: ADR-0002\ntitle: T\ndate: 2026-07-21\nstatus: accepted\nsupersedes-rules: [not-a-ref]\n---\n\n## Context and Problem Statement\n\nx\n\n## Considered Options\n\n* a\n\n## Decision Outcome\n\ny\n")
	_, err := ParseBytes(src, "ADR-0002-t.md")
	if err == nil || !strings.Contains(err.Error(), `field "supersedes-rules"`) {
		t.Fatalf("err = %v, want supersedes-rules field error", err)
	}
}

// category: is no longer part of the schema — a file WITHOUT it parses,
// and a file WITH it still parses (unknown fields are ignored; the
// in-house pre-v0.2 logs rely on this).
func TestParseBytesNoCategoryField(t *testing.T) {
	src := []byte("---\nid: ADR-0001\ntitle: T\ndate: 2026-07-21\nstatus: accepted\n---\n\n## Context and Problem Statement\n\nx\n\n## Considered Options\n\n* a\n\n## Decision Outcome\n\ny\n")
	a, err := ParseBytes(src, "ADR-0001-t.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.IsRuleBearing() {
		t.Fatal("record-only ADR must not be rule-bearing")
	}
}

// TestParseCRLF proves parsing is line-ending-independent: a CRLF-authored
// ADR yields the same model as its LF twin, with no \r bytes surviving
// into any section content (which would otherwise leak into the rendered
// constitution.md and make the projection author-line-ending-dependent).
func TestParseCRLF(t *testing.T) {
	lf := `---
id: ADR-0001
title: CRLF handling
date: 2026-07-01
status: accepted
---

## Context and Problem Statement

Why.

## Considered Options

- Option

## Decision Outcome

First line.
Second line.
`
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")
	dir := t.TempDir()
	lfPath := writeFile(t, dir, "ADR-0001-crlf-handling.md", lf)

	fromLF, err := Parse(lfPath)
	if err != nil {
		t.Fatalf("Parse(LF) error = %v", err)
	}
	fromCRLF, err := ParseBytes([]byte(crlf), lfPath)
	if err != nil {
		t.Fatalf("ParseBytes(CRLF) error = %v", err)
	}

	for name, content := range fromCRLF.Sections {
		if strings.Contains(content, "\r") {
			t.Errorf("section %q from CRLF input contains a \\r byte: %q", name, content)
		}
		if content != fromLF.Sections[name] {
			t.Errorf("section %q differs between CRLF and LF input:\nCRLF: %q\nLF:   %q",
				name, content, fromLF.Sections[name])
		}
	}
	if fromCRLF.ID != fromLF.ID || fromCRLF.Title != fromLF.Title || fromCRLF.Status != fromLF.Status {
		t.Errorf("CRLF and LF inputs parsed to different metadata: %+v vs %+v", fromCRLF, fromLF)
	}
}

// TestParseBOM proves a leading UTF-8 byte-order mark is stripped before
// parsing rather than breaking the frontmatter delimiter match.
func TestParseBOM(t *testing.T) {
	content := "\xef\xbb\xbf" + `---
id: ADR-0001
title: BOM handling
date: 2026-07-01
status: accepted
---

## Decision Outcome

Outcome.
`
	dir := t.TempDir()
	path := writeFile(t, dir, "ADR-0001-bom-handling.md", content)

	got, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil (BOM should be stripped)", err)
	}
	if got.ID != "ADR-0001" || got.Sections[DecisionOutcomeSection] != "Outcome." {
		t.Errorf("BOM'd ADR parsed incorrectly: ID=%q, Decision Outcome=%q",
			got.ID, got.Sections[DecisionOutcomeSection])
	}
}
