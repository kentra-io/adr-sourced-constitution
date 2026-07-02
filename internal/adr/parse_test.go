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

// TestParseMalformed is the M1 DoD malformed-ADR table: each case
// asserts the *exact* error message contract (file, line where
// determinable, field) — errors are UX here, per implementation-plan.md
// §8 ("agents parse them").
func TestParseMalformed(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		wantErr  string // exact Error() string, with {file} substituted for the temp path
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
			// Verified once against go.yaml.in/yaml/v3 v3.0.4's actual
			// error text; the message CONTRACT is ours (see yamlerr.go),
			// only the wrapped %s is library-sourced.
			wantErr: `{file}:2: frontmatter is not valid YAML: yaml: line 1: did not find expected ',' or ']'`,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, tt.filename, tt.content)

			_, err := Parse(path)
			if err == nil {
				t.Fatalf("Parse() error = nil, want error matching %q", tt.wantErr)
			}

			want := strings.ReplaceAll(tt.wantErr, "{file}", path)
			if err.Error() != want {
				t.Errorf("Parse() error = %q, want %q", err.Error(), want)
			}
		})
	}
}
