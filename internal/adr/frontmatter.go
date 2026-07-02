package adr

import (
	"errors"
	"strings"
)

// errNoOpeningDelimiter and errNoClosingDelimiter are sentinels; Parse
// translates them into file-specific *ParseError values. Kept unexported
// so SplitFrontmatter's contract stays a plain (frontmatter, body, error)
// split, independent of any file path.
var (
	errNoOpeningDelimiter = errors.New("missing opening frontmatter delimiter")
	errNoClosingDelimiter = errors.New("missing closing frontmatter delimiter")
)

// frontmatterStartLine is the 1-based line number, within a well-formed
// ADR file, of the frontmatter block's first content line. The opening
// "---" delimiter is always line 1, so the block itself always starts at
// line 2 — SplitFrontmatter only ever succeeds when that holds.
const frontmatterStartLine = 2

// SplitFrontmatter splits raw ADR file content on the manual "---"
// delimiter convention (implementation-plan.md §3: no frontmatter library
// is used for parsing beyond this split — yaml.Unmarshal runs on the
// resulting block). The file must start with a line that is exactly
// "---" (a trailing "\r" is tolerated) and contain a second such line
// terminating the block; everything between is the frontmatter, and
// everything after is the Markdown body.
//
// This is deliberately a pure byte-in/byte-out function with no path
// argument, so it can double as the fuzz target (FuzzSplitFrontmatter):
// its only contract under fuzzing is "never panics", not any particular
// split result for malformed input.
func SplitFrontmatter(data []byte) (frontmatter, body []byte, err error) {
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return nil, nil, errNoOpeningDelimiter
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			fm := strings.Join(lines[1:i], "\n")
			b := strings.Join(lines[i+1:], "\n")
			return []byte(fm), []byte(b), nil
		}
	}
	return nil, nil, errNoClosingDelimiter
}

// fieldLine returns the absolute (whole-file) 1-based line number of the
// frontmatter field named `field` (a top-level "field: value" line), or 0
// if it can't be found. Used to make id/date/status/category validation
// errors precise per the file/line/field contract.
func fieldLine(fm []byte, field string) int {
	lines := strings.Split(string(fm), "\n")
	prefix := field + ":"
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return i + frontmatterStartLine
		}
	}
	return 0
}
