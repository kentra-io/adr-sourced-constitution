package adr

import (
	"fmt"
	"strings"
)

// ParseError is the precise, structured error every ADR validation failure
// returns: file, line (where determinable), field (frontmatter field or
// body section name; empty for whole-file structural errors), and an
// actionable message. Errors are UX here — agents parse them (see
// implementation-plan.md §8) — so the message contract is asserted exactly
// in tests, not just checked for a substring.
type ParseError struct {
	File  string
	Line  int // 0 when not determinable
	Field string
	Msg   string
}

func (e *ParseError) Error() string {
	var b strings.Builder
	b.WriteString(e.File)
	if e.Line > 0 {
		fmt.Fprintf(&b, ":%d", e.Line)
	}
	b.WriteString(": ")
	if e.Field != "" {
		fmt.Fprintf(&b, "field %q: ", e.Field)
	}
	b.WriteString(e.Msg)
	return b.String()
}

// NewValidationError builds a ParseError from outside this package (e.g.
// internal/render's category-vocabulary cross-check, which needs
// constitution.yml and so cannot live in the parser itself) using the same
// file/line/field contract as parse errors.
func NewValidationError(file string, line int, field, msg string) *ParseError {
	return &ParseError{File: file, Line: line, Field: field, Msg: msg}
}
