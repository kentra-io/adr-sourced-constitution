package guard

import (
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

func TestIDCollisions(t *testing.T) {
	tests := []struct {
		name  string
		adrs  []adr.ADR
		want  int // number of violations
		files []string
	}{
		{
			name: "no collision",
			adrs: []adr.ADR{
				{ID: "ADR-0001", Path: "constitution/adr/ADR-0001-a.md"},
				{ID: "ADR-0002", Path: "constitution/adr/ADR-0002-b.md"},
			},
			want: 0,
		},
		{
			name: "one pair collides",
			adrs: []adr.ADR{
				{ID: "ADR-0001", Path: "constitution/adr/ADR-0001-a.md"},
				{ID: "ADR-0001", Path: "constitution/adr/ADR-0001-b.md"},
			},
			want:  1,
			files: []string{"constitution/adr/ADR-0001-a.md", "constitution/adr/ADR-0001-b.md"},
		},
		{
			name: "three-way collision reported as one violation citing all three",
			adrs: []adr.ADR{
				{ID: "ADR-0001", Path: "constitution/adr/ADR-0001-a.md"},
				{ID: "ADR-0001", Path: "constitution/adr/ADR-0001-b.md"},
				{ID: "ADR-0001", Path: "constitution/adr/ADR-0001-c.md"},
			},
			want:  1,
			files: []string{"constitution/adr/ADR-0001-a.md", "constitution/adr/ADR-0001-b.md", "constitution/adr/ADR-0001-c.md"},
		},
		{
			name: "two independent collisions yield two violations",
			adrs: []adr.ADR{
				{ID: "ADR-0001", Path: "constitution/adr/ADR-0001-a.md"},
				{ID: "ADR-0001", Path: "constitution/adr/ADR-0001-b.md"},
				{ID: "ADR-0002", Path: "constitution/adr/ADR-0002-a.md"},
				{ID: "ADR-0002", Path: "constitution/adr/ADR-0002-b.md"},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idCollisions(tt.adrs)
			if len(got) != tt.want {
				t.Fatalf("idCollisions() = %+v, want %d violation(s)", got, tt.want)
			}
			for _, v := range got {
				if v.Kind != KindIDCollision {
					t.Errorf("violation.Kind = %q, want %q", v.Kind, KindIDCollision)
				}
				if len(v.Files) < 2 {
					t.Errorf("violation.Files = %v, want at least 2 entries", v.Files)
				}
			}
			if tt.files != nil {
				if len(got) != 1 {
					t.Fatalf("expected exactly one violation to check Files against, got %d", len(got))
				}
				if len(got[0].Files) != len(tt.files) {
					t.Errorf("violation.Files = %v, want %v", got[0].Files, tt.files)
				}
			}
		})
	}
}
