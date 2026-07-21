package adr

import (
	"fmt"
	"strings"
)

// RulesSection is the optional body heading that makes an ADR rule-bearing
// (proposal D5/A1). Its content is the h3/h4 rules grammar:
// "### <category>" subsections each holding "#### <slug>" rule entries.
const RulesSection = "Rules"

// Rule is one standing rule carried by an ADR: filed under a category from
// the configured vocabulary, addressable as ADR-NNNN/<Category>/<Slug>,
// with Text the exact prose the constitution renders.
type Rule struct {
	Category string
	Slug     string
	Text     string
}

// ParseRulesSection parses a "## Rules" section's content (as
// ExtractSections returns it: h3/h4 lines intact) into its rule entries,
// preserving file order. The grammar is strict (proposal A1): every rule
// lives under exactly one "### <category>" and one "#### <slug>"; untagged
// text anywhere, empty rule bodies, empty categories, re-opened (repeated)
// category headings, duplicate slugs within a category, non-kebab names,
// and deeper/stray heading lines are all *ParseError rejections — never
// silently swallowed.
func ParseRulesSection(content, file string) ([]Rule, error) {
	fail := func(msg string) ([]Rule, error) {
		return nil, &ParseError{File: file, Field: RulesSection, Msg: msg}
	}
	if strings.TrimSpace(content) == "" {
		return fail(`the "## Rules" section is present but empty; give it "### <category>" / "#### <slug>" rule entries or remove it (a record-only ADR has no Rules section)`)
	}

	var rules []Rule
	category := ""
	slug := ""
	entriesInCategory := 0
	var buf []string
	seen := map[string]bool{}           // "category/slug"
	seenCategories := map[string]bool{} // category name

	flushRule := func() error {
		if slug == "" {
			return nil
		}
		text := strings.TrimSpace(strings.Join(buf, "\n"))
		if text == "" {
			return &ParseError{File: file, Field: RulesSection,
				Msg: fmt.Sprintf("rule %q has no text; every \"#### <slug>\" entry carries the rule's prose", category+"/"+slug)}
		}
		rules = append(rules, Rule{Category: category, Slug: slug, Text: text})
		return nil
	}

	// closeCategory runs whenever the current category is about to end
	// (a new "### " heading starts, or we hit EOF): a category that
	// collected zero rule entries is rejected, whether it was the first,
	// an intermediate, or the trailing one.
	closeCategory := func() error {
		if category != "" && entriesInCategory == 0 {
			return &ParseError{File: file, Field: RulesSection,
				Msg: fmt.Sprintf("category %q has no rule entries", category)}
		}
		return nil
	}

	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "#### "):
			if err := flushRule(); err != nil {
				return nil, err
			}
			s := strings.TrimSpace(strings.TrimPrefix(line, "#### "))
			if category == "" {
				return fail(fmt.Sprintf("rule entry %q must appear under a \"### <category>\" heading", s))
			}
			if !isKebab(s) {
				return fail(fmt.Sprintf("rule slug %q must be kebab-case", s))
			}
			if seen[category+"/"+s] {
				return fail(fmt.Sprintf("duplicate rule slug %q in category %q", s, category))
			}
			seen[category+"/"+s] = true
			slug = s
			entriesInCategory++
			buf = nil
		case strings.HasPrefix(line, "### "):
			if err := flushRule(); err != nil {
				return nil, err
			}
			if err := closeCategory(); err != nil {
				return nil, err
			}
			c := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			if !isKebab(c) {
				return fail(fmt.Sprintf("category %q must be kebab-case", c))
			}
			if seenCategories[c] {
				return fail(fmt.Sprintf("category %q appears more than once; group all of a category's rules under one heading", c))
			}
			seenCategories[c] = true
			category = c
			entriesInCategory = 0
			slug = ""
		case strings.HasPrefix(strings.TrimLeft(line, " \t"), "#"):
			return fail("rule text is plain prose and must not contain Markdown heading lines; found: " + strings.TrimSpace(line))
		default:
			if strings.TrimSpace(line) == "" {
				if slug != "" {
					buf = append(buf, line)
				}
				continue
			}
			if slug == "" {
				if category == "" {
					return fail(fmt.Sprintf("text %q must appear under a \"### <category>\" heading", strings.TrimSpace(line)))
				}
				return fail(fmt.Sprintf("text %q must appear under a \"#### <slug>\" rule heading", strings.TrimSpace(line)))
			}
			buf = append(buf, line)
		}
	}
	if err := flushRule(); err != nil {
		return nil, err
	}
	if err := closeCategory(); err != nil {
		return nil, err
	}
	return rules, nil
}
