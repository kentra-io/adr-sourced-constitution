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
// cfg.Categories order (plan §3), skipping every rule whose ref is in
// `retired` (the fold's mask — proposal A6; nil means no retirements).
// ADRs are sorted ascending by numeric id first, and an ADR's rules keep
// their file order, so each category's entries are ordered by (ADR number,
// position in file). Categories with zero entries are omitted from the
// output. ADRs with no rules contribute nothing — record-only ADRs stay in
// the log alone.
func Group(cfg *config.Config, active []adr.ADR, retired map[string]string) []CategorySection {
	sorted := append([]adr.ADR(nil), active...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Num < sorted[j].Num })

	byCategory := make(map[string][]Entry, len(cfg.Categories))
	for _, a := range sorted {
		for _, r := range a.Rules {
			ref := adr.RuleRef{ADRID: a.ID, Category: r.Category, Slug: r.Slug}
			if _, masked := retired[ref.String()]; masked {
				continue
			}
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

// Render produces the byte-exact constitution.md projection plus fold
// warnings (currently: rule resurrections — proposal A7). Steps: validate
// categories -> index every rule in the log -> validate retirement refs ->
// mask rules retired by currently-accepted ADRs -> group -> render.
func Render(cfg *config.Config, adrs []adr.ADR) ([]byte, []string, error) {
	if err := ValidateCategories(cfg, adrs); err != nil {
		return nil, nil, err
	}
	retired, warnings, err := foldRetirements(adrs)
	if err != nil {
		return nil, nil, err
	}
	sections := Group(cfg, ActiveSet(adrs), retired)
	out, err := renderTemplate(sections)
	if err != nil {
		return nil, nil, err
	}
	return out, warnings, nil
}

// foldRetirements validates every retirement ref in the log and returns the
// set of refs retired by currently-accepted ADRs (proposal A6/A7) as a
// ref-string -> retiring-ADR-id map, plus one warning per rule a
// no-longer-accepted retirer resurrects.
//
// Errors (each a *adr.ParseError naming the retirer's file and the
// originating frontmatter list): a ref listed more than once by one ADR
// (within a list or across the two lists), a ref that resolves to no rule
// in the log (any status — frozen history is addressable), a forward/self
// ref (an ADR may only retire rules of an earlier ADR), and two accepted
// ADRs retiring the same ref.
//
// A7 semantics: retirements count only from currently-accepted ADRs. A
// no-longer-accepted retirer's refs are recorded as resurrection
// candidates instead; after the pass, a warning is emitted for each
// candidate whose ref no accepted ADR retires — those rules actually
// become visible again. Warning order follows log order (deterministic).
func foldRetirements(adrs []adr.ADR) (map[string]string, []string, error) {
	// Pass 1: index every rule of every ADR, any status — retirement refs
	// address frozen history, not just the active set.
	index := make(map[string]*adr.ADR)
	for i := range adrs {
		a := &adrs[i]
		for _, r := range a.Rules {
			index[adr.RuleRef{ADRID: a.ID, Category: r.Category, Slug: r.Slug}.String()] = a
		}
	}

	type origin struct {
		ref   adr.RuleRef
		field string // the frontmatter list the ref came from, for errors
	}
	type candidate struct{ retirerID, ref string }

	retired := make(map[string]string) // ref -> retiring ADR id (accepted retirers only)
	var candidates []candidate         // resurrections, pending the visibility filter

	// Pass 2: validate every ref of every ADR, in log order.
	for i := range adrs {
		a := &adrs[i]
		refs := make([]origin, 0, len(a.SupersedesRules)+len(a.RemovesRules))
		for _, r := range a.SupersedesRules {
			refs = append(refs, origin{r, "supersedes-rules"})
		}
		for _, r := range a.RemovesRules {
			refs = append(refs, origin{r, "removes-rules"})
		}

		listed := make(map[string]bool, len(refs))
		for _, o := range refs {
			key := o.ref.String()
			// Precedence: the duplicate check runs before resolution, so a
			// duplicated dangling ref reports "listed more than once", not
			// the dangling error.
			if listed[key] {
				return nil, nil, adr.NewValidationError(a.Path, 0, o.field, fmt.Sprintf(
					"retirement ref %q: listed more than once by %s", key, a.ID))
			}
			listed[key] = true

			target, ok := index[key]
			if !ok {
				return nil, nil, adr.NewValidationError(a.Path, 0, o.field, fmt.Sprintf(
					"retirement ref %q does not resolve to any rule in the log", key))
			}
			if target.Num >= a.Num {
				return nil, nil, adr.NewValidationError(a.Path, 0, o.field, fmt.Sprintf(
					"retirement ref %q: an ADR may only retire rules of an earlier ADR", key))
			}

			if a.Status != adr.StatusAccepted {
				// A7: this retirer's retirements no longer apply. Only a rule
				// that would otherwise project (accepted target) can resurrect.
				if target.Status == adr.StatusAccepted {
					candidates = append(candidates, candidate{a.ID, key})
				}
				continue
			}

			if prev, dup := retired[key]; dup {
				return nil, nil, adr.NewValidationError(a.Path, 0, o.field, fmt.Sprintf(
					"retirement ref %q: already retired by %s", key, prev))
			}
			retired[key] = a.ID
		}
	}

	// Visibility filter: only warn about resurrections that actually show —
	// a ref some accepted ADR also retires stays masked, nothing resurfaces.
	var warnings []string
	for _, c := range candidates {
		if _, stillMasked := retired[c.ref]; stillMasked {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"%s is no longer accepted; its retirement of %s no longer applies — the rule projects again (resurrected). Re-retire it if that is unintended.",
			c.retirerID, c.ref))
	}
	return retired, warnings, nil
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
