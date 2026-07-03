package adr

import "strings"

// ExtractSections splits an ADR's Markdown body into its "## Heading"
// sections (the MADR body convention, spec §4.1), returning a map from
// heading text to trimmed section content and the order headings appeared
// in. Content before the first "## " heading is discarded (MADR ADRs open
// directly with "## Context and Problem Statement"; there is no
// preamble section to preserve).
//
// This is intentionally permissive about *which* headings appear — full
// mandatory-section enforcement is validateSections, so M2's write path
// can reuse extraction with a different required set (implementation-plan
// says full body-shape enforcement on write is M2's job).
func ExtractSections(body []byte) (sections map[string]string, order []string) {
	sections = map[string]string{}
	lines := strings.Split(string(body), "\n")

	var current string
	var buf []string
	inSection := false

	flush := func() {
		if inSection {
			sections[current] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
	}

	for _, line := range lines {
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			current = strings.TrimSpace(heading)
			order = append(order, current)
			buf = nil
			inSection = true
			continue
		}
		if inSection {
			buf = append(buf, line)
		}
	}
	flush()

	return sections, order
}

// validateRuleSection enforces the rule-bearing contract (plan §2.12): a
// "## Rule" section MAY be absent (the ADR is then a catalog-only record),
// but if present it must appear exactly once, must not be empty or
// whitespace-only, and must be plain prose — no line may begin with a
// Markdown heading marker. A rule is 1–3 lines of prose; malformed rule
// input is rejected here, never silently swallowed (a duplicate heading would
// let the last one silently win the projection; a heading line would either
// inject an outline entry into constitution.md or be swallowed as a section
// delimiter). Presence with valid content is what makes an ADR project into
// constitution.md. order is the heading order ExtractSections returns; it is
// needed because the sections map collapses a duplicate heading to one entry.
func validateRuleSection(sections map[string]string, order []string, file string) error {
	if countHeadings(order, RuleSection) > 1 {
		return &ParseError{
			File:  file,
			Field: RuleSection,
			Msg:   "the \"## Rule\" section appears more than once; a body may carry at most one \"## Rule\" section",
		}
	}
	content, present := sections[RuleSection]
	if !present {
		return nil
	}
	if strings.TrimSpace(content) == "" {
		return &ParseError{
			File:  file,
			Field: RuleSection,
			Msg:   "the \"## Rule\" section is present but empty; give it a normative statement or remove it (a record-only ADR has no Rule section)",
		}
	}
	return ruleTextError(content, file, RuleSection)
}

// countHeadings reports how many times name appears in a heading-order slice.
func countHeadings(order []string, name string) int {
	n := 0
	for _, h := range order {
		if h == name {
			n++
		}
	}
	return n
}

// ValidateRuleText enforces the plain-prose Rule contract (plan §2.12) on a
// raw rule statement supplied outside a parsed body — the `--rule` flag. A
// Rule is 1–3 lines of plain prose; no line may begin with a Markdown heading
// marker ('#'). This is validated on the raw flag value because composing a
// "## Rule" section from heading-bearing text would otherwise split the text
// across sections (silently truncating the rule) before the section-based
// validators could see it. file locates the error (e.g. "--rule").
func ValidateRuleText(text, file string) error {
	return ruleTextError(text, file, "")
}

// ruleTextError returns a *ParseError for the first line of text that begins
// with a Markdown heading marker ('#', ignoring up to leading indentation),
// or nil when every line is plain prose. field is the section name for the
// error's location ("Rule" on a parsed body) or "" for a raw flag value.
func ruleTextError(text, file, field string) error {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			return &ParseError{
				File:  file,
				Field: field,
				Msg:   "rule text is plain prose and must not contain Markdown heading lines; found a line beginning with \"#\": " + strings.TrimSpace(line),
			}
		}
	}
	return nil
}

// validateSections checks that every section in `required` is present in
// `sections`, returning a precise *ParseError (file + field = the missing
// heading) for the first one missing. Exported indirectly via the
// required set M1 enforces (requiredForRegen, parse.go); M2 will call this
// with the full MandatorySections set on the write path.
func validateSections(sections map[string]string, required []string, file string) error {
	for _, name := range required {
		if _, ok := sections[name]; !ok {
			return &ParseError{
				File:  file,
				Field: name,
				Msg:   "required section \"## " + name + "\" is missing",
			}
		}
	}
	return nil
}
