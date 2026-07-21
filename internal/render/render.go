// Package render implements the deterministic constitution.md projection
// (spec §6): active-set resolution, grouping by category (in config
// order) and sorting (by numeric id), and rendering via text/template.
//
// Determinism is a hard requirement (implementation-plan.md §3): no map
// iteration may leak into output order. Grouping below always walks
// cfg.Categories (a slice, order-preserving) rather than ranging over a
// map, and every per-category ADR slice is explicitly sorted.
package render

import (
	"fmt"
	"sort"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

// CategorySection is one rendered category: its heading and the rule
// entries in it, in ascending ADR-number order (then file order within an
// ADR).
type CategorySection struct {
	Name    string
	Entries []Entry
}

// Entry is one projected rule: the rule itself plus the ADR that carries
// it (for the metadata line).
type Entry struct {
	Rule adr.Rule
	ADR  adr.ADR
}

// ActiveSet returns the ADRs with status "accepted" — spec §5/§6: the
// active set; superseded/deprecated ADRs are dropped (their metadata still
// parses, they're just excluded here).
func ActiveSet(adrs []adr.ADR) []adr.ADR {
	active := make([]adr.ADR, 0, len(adrs))
	for _, a := range adrs {
		if a.Status == adr.StatusAccepted {
			active = append(active, a)
		}
	}
	return active
}

// ValidateCategories hard-errors on any rule of any ADR (active or not —
// the log is a validated input in its entirety, per implementation-plan.md
// §2.5) whose category isn't in the project's configured vocabulary.
func ValidateCategories(cfg *config.Config, adrs []adr.ADR) error {
	valid := make(map[string]bool, len(cfg.Categories))
	for _, c := range cfg.Categories {
		valid[c] = true
	}
	for _, a := range adrs {
		for _, r := range a.Rules {
			if !valid[r.Category] {
				return adr.NewValidationError(a.Path, 0, "category", fmt.Sprintf(
					"rule %s/%s: not in the configured category vocabulary %s (got %q)",
					r.Category, r.Slug, formatVocabulary(cfg.Categories), r.Category,
				))
			}
		}
	}
	return nil
}

// Group buckets the active set's rules by their category, in
// cfg.Categories order (plan §3). ADRs are sorted ascending by numeric id
// first, and an ADR's rules keep their file order, so each category's
// entries are ordered by (ADR number, position in file). Categories with
// zero entries are omitted from the output. ADRs with no rules contribute
// nothing — record-only ADRs stay in the log alone.
func Group(cfg *config.Config, active []adr.ADR) []CategorySection {
	sorted := append([]adr.ADR(nil), active...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Num < sorted[j].Num })

	byCategory := make(map[string][]Entry, len(cfg.Categories))
	for _, a := range sorted {
		for _, r := range a.Rules {
			byCategory[r.Category] = append(byCategory[r.Category], Entry{Rule: r, ADR: a})
		}
	}

	sections := make([]CategorySection, 0, len(cfg.Categories))
	for _, cat := range cfg.Categories {
		inCat := byCategory[cat]
		if len(inCat) == 0 {
			continue
		}
		sections = append(sections, CategorySection{Name: cat, Entries: inCat})
	}
	return sections
}

// Render produces the byte-exact constitution.md projection (spec §6):
// validate categories -> active set -> group -> render template.
func Render(cfg *config.Config, adrs []adr.ADR) ([]byte, error) {
	if err := ValidateCategories(cfg, adrs); err != nil {
		return nil, err
	}
	sections := Group(cfg, ActiveSet(adrs))
	return renderTemplate(sections)
}

func formatVocabulary(categories []string) string {
	out := "["
	for i, c := range categories {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out + "]"
}
