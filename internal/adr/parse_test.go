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
category: code-style
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
	if got.Category != "code-style" {
		t.Errorf("Category = %q", got.Category)
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

// TestParseRuleBearing proves the rule-bearing model (plan §2.12): a "## Rule"
// section is optional, and its presence with content makes the ADR
// rule-bearing while its absence makes the ADR a catalog-only record.
func TestParseRuleBearing(t *testing.T) {
	dir := t.TempDir()

	ruleBody := `---
id: ADR-0001
title: A standing rule
category: architecture
date: 2026-07-01
status: accepted
---

## Context and Problem Statement

Why.

## Considered Options

- A

## Decision Outcome

The full decision, at length.

## Rule

Do the thing; do not do the other thing.
`
	rulePath := writeFile(t, dir, "ADR-0001-a-standing-rule.md", ruleBody)
	a, err := Parse(rulePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !a.IsRuleBearing() {
		t.Error("ADR with a ## Rule section should be rule-bearing")
	}
	if got := a.Rule(); got != "Do the thing; do not do the other thing." {
		t.Errorf("Rule() = %q", got)
	}

	recordBody := `---
id: ADR-0002
title: A catalog record
category: architecture
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
		t.Error("ADR with no ## Rule section must be a catalog-only record")
	}
	if rec.Rule() != "" {
		t.Errorf("Rule() = %q, want empty", rec.Rule())
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
category: architecture
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
category: architecture
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
category: architecture
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
category: architecture
date: 2026-07-01
status: proposed
---

## Decision Outcome

Body.
`,
			wantErr: `{file}:6: field "status": must be one of "accepted", "superseded", "deprecated" (got "proposed")`,
		},
		{
			name:     "missing Decision Outcome",
			filename: "ADR-0006-missing-decision-outcome.md",
			content: `---
id: ADR-0006
title: Missing decision outcome section
category: architecture
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
category: architecture
date: 07/01/2026
status: accepted
---

## Decision Outcome

Body.
`,
			wantErr: `{file}:5: field "date": must be an ISO-8601 date YYYY-MM-DD (got "07/01/2026")`,
		},
		{
			name:     "id does not match filename",
			filename: "ADR-0008-mismatched-id.md",
			content: `---
id: ADR-0009
title: Frontmatter id does not match filename
category: architecture
date: 2026-07-01
status: accepted
---

## Decision Outcome

Body.
`,
			wantErr: `{file}:2: field "id": frontmatter id "ADR-0009" does not match filename-derived id "ADR-0008"`,
		},
		{
			name:     "empty Rule section",
			filename: "ADR-0009-empty-rule.md",
			content: `---
id: ADR-0009
title: Empty rule section
category: architecture
date: 2026-07-01
status: accepted
---

## Decision Outcome

Body.

## Rule

` + "   " + `
`,
			wantErr: `{file}: field "Rule": the "## Rule" section is present but empty; give it a normative statement or remove it (a record-only ADR has no Rule section)`,
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

// TestParseCRLF proves parsing is line-ending-independent: a CRLF-authored
// ADR yields the same model as its LF twin, with no \r bytes surviving
// into any section content (which would otherwise leak into the rendered
// constitution.md and make the projection author-line-ending-dependent).
func TestParseCRLF(t *testing.T) {
	lf := `---
id: ADR-0001
title: CRLF handling
category: architecture
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
category: architecture
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
