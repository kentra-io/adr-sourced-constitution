package adr

import (
	"fmt"
	"strings"
)

// RuleRef addresses one rule of one ADR: "ADR-NNNN/<category>/<slug>"
// (proposal D5). Slugs are unique within their (ADR, category), so a full
// ref is globally unambiguous. Sealed bodies are frozen, making refs
// immutable by construction.
type RuleRef struct {
	ADRID    string // "ADR-0004"
	Category string
	Slug     string
}

func (r RuleRef) String() string {
	return r.ADRID + "/" + r.Category + "/" + r.Slug
}

// ParseRuleRef parses and validates a rule ref. The error text is UX —
// agents read it — so it names the failing part precisely.
func ParseRuleRef(s string) (RuleRef, error) {
	bad := func(detail string) (RuleRef, error) {
		msg := fmt.Sprintf("rule ref must be %q (got %q)", "ADR-NNNN/<category>/<slug>", s)
		if detail != "" {
			msg += ": " + detail
		}
		return RuleRef{}, fmt.Errorf("%s", msg)
	}
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return bad("")
	}
	id, cat, slug := parts[0], parts[1], parts[2]
	if _, ok := parseID(id); !ok {
		return bad(fmt.Sprintf("id %q must match %q", id, "ADR-NNNN"))
	}
	if !isKebab(cat) {
		return bad(fmt.Sprintf("category %q must be kebab-case", cat))
	}
	if !isKebab(slug) {
		return bad(fmt.Sprintf("slug %q must be kebab-case", slug))
	}
	return RuleRef{ADRID: id, Category: cat, Slug: slug}, nil
}

// isKebab reports whether s is non-empty lowercase kebab-case:
// [a-z0-9]+ groups joined by single hyphens.
func isKebab(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") ||
		strings.Contains(s, "--") {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}
