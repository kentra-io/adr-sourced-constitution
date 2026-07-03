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

// CategorySection is one rendered category: its heading and the active
// ADRs in it, sorted numerically by id.
type CategorySection struct {
	Name string
	ADRs []adr.ADR
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

// RuleBearing filters to the ADRs that carry a "## Rule" section — the
// curated read model (plan §2.12): only rule-bearing active ADRs project into
// constitution.md; catalog-only records stay in the log alone.
func RuleBearing(adrs []adr.ADR) []adr.ADR {
	out := make([]adr.ADR, 0, len(adrs))
	for i := range adrs {
		if adrs[i].IsRuleBearing() {
			out = append(out, adrs[i])
		}
	}
	return out
}

// ValidateCategories hard-errors on any ADR (active or not — the log is a
// validated input in its entirety, per implementation-plan.md §2.5) whose
// category isn't in the project's configured vocabulary.
func ValidateCategories(cfg *config.Config, adrs []adr.ADR) error {
	valid := make(map[string]bool, len(cfg.Categories))
	for _, c := range cfg.Categories {
		valid[c] = true
	}
	for _, a := range adrs {
		if !valid[a.Category] {
			return adr.NewValidationError(a.Path, 0, "category", fmt.Sprintf(
				"not in the configured category vocabulary %s (got %q)",
				formatVocabulary(cfg.Categories), a.Category,
			))
		}
	}
	return nil
}

// Group buckets the active set by category, in cfg.Categories order
// (plan §3), each category's ADRs sorted ascending by numeric id.
// Categories with zero active ADRs are omitted from the output.
func Group(cfg *config.Config, active []adr.ADR) []CategorySection {
	byCategory := make(map[string][]adr.ADR, len(cfg.Categories))
	for _, a := range active {
		byCategory[a.Category] = append(byCategory[a.Category], a)
	}

	sections := make([]CategorySection, 0, len(cfg.Categories))
	for _, cat := range cfg.Categories {
		inCat := byCategory[cat]
		if len(inCat) == 0 {
			continue
		}
		sort.Slice(inCat, func(i, j int) bool { return inCat[i].Num < inCat[j].Num })
		sections = append(sections, CategorySection{Name: cat, ADRs: inCat})
	}
	return sections
}

// Render produces the byte-exact constitution.md projection (spec §6):
// validate categories -> active set -> group -> render template.
func Render(cfg *config.Config, adrs []adr.ADR) ([]byte, error) {
	if err := ValidateCategories(cfg, adrs); err != nil {
		return nil, err
	}
	sections := Group(cfg, RuleBearing(ActiveSet(adrs)))
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
