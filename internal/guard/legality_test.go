package guard

import (
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// TestLegalStatusTransition is the legality matrix plan §8 calls for:
// every status-pair x superseded-by combination the guard must classify as
// legal or illegal (spec §5.2: "the sole allowed change is accepted ->
// superseded | deprecated"; §5.3: "no resurrection").
func TestLegalStatusTransition(t *testing.T) {
	tests := []struct {
		name            string
		oldStatus       adr.Status
		oldSupersededBy string
		curStatus       adr.Status
		curSupersededBy string
		wantLegal       bool
	}{
		// accepted -> * : the only origin status any transition may leave from.
		{"accepted->superseded with backlink is legal", adr.StatusAccepted, "", adr.StatusSuperseded, "ADR-0002", true},
		{"accepted->superseded without backlink is illegal", adr.StatusAccepted, "", adr.StatusSuperseded, "", false},
		{"accepted->deprecated with no backlink is legal", adr.StatusAccepted, "", adr.StatusDeprecated, "", true},
		{"accepted->deprecated WITH a backlink is illegal", adr.StatusAccepted, "", adr.StatusDeprecated, "ADR-0002", false},
		{"accepted->accepted with a forged backlink is illegal", adr.StatusAccepted, "", adr.StatusAccepted, "ADR-0002", false},

		// superseded -> * : never legal, including staying superseded with a
		// different backlink (re-pointing) or reverting to accepted.
		{"superseded->accepted (resurrection) is illegal", adr.StatusSuperseded, "ADR-0002", adr.StatusAccepted, "", false},
		{"superseded->deprecated is illegal", adr.StatusSuperseded, "ADR-0002", adr.StatusDeprecated, "", false},
		{"superseded re-pointed to a different backlink is illegal", adr.StatusSuperseded, "ADR-0002", adr.StatusSuperseded, "ADR-0003", false},

		// deprecated -> * : never legal, including reverting to accepted or
		// superseded, or acquiring a backlink after the fact.
		{"deprecated->accepted (resurrection) is illegal", adr.StatusDeprecated, "", adr.StatusAccepted, "", false},
		{"deprecated->superseded is illegal", adr.StatusDeprecated, "", adr.StatusSuperseded, "ADR-0002", false},
		{"deprecated acquiring a backlink is illegal", adr.StatusDeprecated, "", adr.StatusDeprecated, "ADR-0002", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := &adr.ADR{Status: tt.oldStatus, SupersededBy: tt.oldSupersededBy}
			cur := &adr.ADR{Status: tt.curStatus, SupersededBy: tt.curSupersededBy}
			if got := legalStatusTransition(old, cur); got != tt.wantLegal {
				t.Errorf("legalStatusTransition(%s/%q -> %s/%q) = %v, want %v",
					tt.oldStatus, tt.oldSupersededBy, tt.curStatus, tt.curSupersededBy, got, tt.wantLegal)
			}
		})
	}
}

// baseADR returns a well-formed, self-consistent ADR the compareLegal
// tests mutate one field at a time from.
func baseADR() *adr.ADR {
	return &adr.ADR{
		ID: "ADR-0001", Title: "Title", Category: "architecture", Date: "2026-07-01",
		Status: adr.StatusAccepted, Source: "bootstrap",
		Sections:     map[string]string{"Decision Outcome": "y"},
		SectionOrder: []string{"Decision Outcome"},
		Path:         "constitution/adr/ADR-0001-title.md",
	}
}

func TestCompareLegal(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*adr.ADR)
		wantKinds []Kind
		wantField string // checked against the first violation with this kind, when non-empty
	}{
		{
			name:      "no change is clean",
			mutate:    func(_ *adr.ADR) {},
			wantKinds: nil,
		},
		{
			name:      "body content changed",
			mutate:    func(a *adr.ADR) { a.Sections["Decision Outcome"] = "z" },
			wantKinds: []Kind{KindBodyChanged},
		},
		{
			name:      "a new section appended",
			mutate:    func(a *adr.ADR) { a.SectionOrder = append(a.SectionOrder, "Consequences") },
			wantKinds: []Kind{KindBodyChanged},
		},
		{
			name:      "title changed",
			mutate:    func(a *adr.ADR) { a.Title = "New Title" },
			wantKinds: []Kind{KindFrozenFieldChanged},
			wantField: "title",
		},
		{
			name:      "category changed",
			mutate:    func(a *adr.ADR) { a.Category = "process" },
			wantKinds: []Kind{KindFrozenFieldChanged},
			wantField: "category",
		},
		{
			name:      "date changed",
			mutate:    func(a *adr.ADR) { a.Date = "2026-07-02" },
			wantKinds: []Kind{KindFrozenFieldChanged},
			wantField: "date",
		},
		{
			name:      "source changed",
			mutate:    func(a *adr.ADR) { a.Source = "FS-0042" },
			wantKinds: []Kind{KindFrozenFieldChanged},
			wantField: "source",
		},
		{
			name:      "supersedes changed",
			mutate:    func(a *adr.ADR) { a.Supersedes = "ADR-0000" },
			wantKinds: []Kind{KindFrozenFieldChanged},
			wantField: "supersedes",
		},
		{
			name: "legal supersede transition is clean",
			mutate: func(a *adr.ADR) {
				a.Status = adr.StatusSuperseded
				a.SupersededBy = "ADR-0002"
			},
			wantKinds: nil,
		},
		{
			name:      "legal deprecate transition is clean",
			mutate:    func(a *adr.ADR) { a.Status = adr.StatusDeprecated },
			wantKinds: nil,
		},
		{
			name: "accepted acquiring a backlink without a status change is illegal",
			mutate: func(a *adr.ADR) {
				a.SupersededBy = "ADR-0002" // status stays accepted; only the backlink appears
			},
			wantKinds: []Kind{KindFrozenFieldChanged},
			wantField: "status",
		},
		{
			name: "body AND a frozen field changed together yields both violations",
			mutate: func(a *adr.ADR) {
				a.Sections["Decision Outcome"] = "z"
				a.Category = "process"
			},
			wantKinds: []Kind{KindBodyChanged, KindFrozenFieldChanged},
		},
		{
			// A LEGAL status transition does NOT license smuggling frozen-content
			// changes alongside it: the status change is allowed, but the frozen
			// field and body edits riding with it must still each fire. Proves
			// the legality allow-list is scoped to status/superseded-by only.
			name: "a legal supersede carrying a frozen-field edit and a body edit still fires both content violations",
			mutate: func(a *adr.ADR) {
				a.Status = adr.StatusSuperseded
				a.SupersededBy = "ADR-0002"          // legal transition -> no status violation
				a.Category = "process"               // frozen field -> frozen_field_changed
				a.Sections["Decision Outcome"] = "z" // body -> body_changed
			},
			wantKinds: []Kind{KindBodyChanged, KindFrozenFieldChanged},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := baseADR()
			cur := baseADR()
			tt.mutate(cur)

			got := compareLegal(old.ID, old.Path, old, cur)
			if len(got) != len(tt.wantKinds) {
				t.Fatalf("compareLegal() = %+v, want %d violation(s) of kind(s) %v", got, len(tt.wantKinds), tt.wantKinds)
			}
			for i, v := range got {
				if v.Kind != tt.wantKinds[i] {
					t.Errorf("violation[%d].Kind = %q, want %q", i, v.Kind, tt.wantKinds[i])
				}
				if v.ID != old.ID {
					t.Errorf("violation[%d].ID = %q, want %q", i, v.ID, old.ID)
				}
				if v.File != old.Path {
					t.Errorf("violation[%d].File = %q, want %q", i, v.File, old.Path)
				}
			}
			if tt.wantField != "" {
				found := false
				for _, v := range got {
					if v.Field == tt.wantField {
						found = true
					}
				}
				if !found {
					t.Errorf("compareLegal() = %+v, want a violation with Field %q", got, tt.wantField)
				}
			}
		})
	}
}

// TestCompareLegalResurrection covers the one legality case baseADR's
// "mutate cur only" table shape can't express: a base-ref (old) version
// that is already superseded, reverted back to accepted in the current
// version — a resurrection, illegal regardless of the destination status
// (spec §5.3: "no resurrection").
func TestCompareLegalResurrection(t *testing.T) {
	old := baseADR()
	old.Status = adr.StatusSuperseded
	old.SupersededBy = "ADR-0002"

	cur := baseADR()
	cur.Status = adr.StatusAccepted
	cur.SupersededBy = ""

	got := compareLegal(old.ID, old.Path, old, cur)
	if len(got) != 1 || got[0].Kind != KindFrozenFieldChanged || got[0].Field != "status" {
		t.Fatalf("compareLegal() = %+v, want exactly one frozen_field_changed on \"status\"", got)
	}
}
