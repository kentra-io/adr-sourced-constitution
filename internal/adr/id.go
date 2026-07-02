package adr

import (
	"regexp"
	"strconv"
)

// idPattern matches the "ADR-NNNN" id format (spec §4.1, §4.3: monotonic
// zero-padded ids, at least 4 digits so the log can grow past 9999
// without a format change).
var idPattern = regexp.MustCompile(`^ADR-([0-9]{4,})$`)

// parseID validates and extracts the numeric part of an "ADR-NNNN" id,
// used both for format validation and for the numeric sort key (plan §3:
// "ADRs by numeric id").
func parseID(id string) (num int, ok bool) {
	m := idPattern.FindStringSubmatch(id)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// filenamePattern matches the "ADR-NNNN-slug.md" filename convention
// (spec §4.1 deviation (iii)). Capture group 1 is the id portion, reused
// to cross-check against the frontmatter `id` field.
var filenamePattern = regexp.MustCompile(`^(ADR-[0-9]{4,})-.+\.md$`)

// filenameID extracts the id encoded in an ADR filename, or ok=false if
// the filename doesn't match the required convention.
func filenameID(name string) (id string, ok bool) {
	m := filenamePattern.FindStringSubmatch(name)
	if m == nil {
		return "", false
	}
	return m[1], true
}
