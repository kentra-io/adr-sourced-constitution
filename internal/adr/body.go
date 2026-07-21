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

// validateAndParseRules extracts + validates the optional Rules section of
// an already-extracted body. Shared by the read path (parseBytesCore) and
// the write path (ValidateBody) so valid-on-write and valid-on-read can
// never drift. A "## Rules" section MAY be absent (the ADR is then a
// record-only entry); when present it must appear exactly once (order is
// the heading order ExtractSections returns — the sections map collapses a
// duplicate heading to one entry) and must satisfy the strict h3/h4 rules
// grammar (ParseRulesSection).
func validateAndParseRules(sections map[string]string, order []string, file string) ([]Rule, error) {
	if countHeadings(order, RulesSection) > 1 {
		return nil, &ParseError{File: file, Field: RulesSection,
			Msg: "the \"## Rules\" section appears more than once; a body may carry at most one"}
	}
	content, present := sections[RulesSection]
	if !present {
		return nil, nil
	}
	return ParseRulesSection(content, file)
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
