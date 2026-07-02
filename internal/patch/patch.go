// Package patch performs line-targeted textual edits on raw ADR bytes: it
// changes a single frontmatter field's value (and, for supersede, inserts
// one derived line) while leaving *every other byte* — body, other
// frontmatter, indentation, line endings, a leading BOM — byte-identical.
//
// This is the §5.3 immutability guarantee made mechanical: the only legal
// change to an accepted ADR is its status line (plus the derived
// superseded-by back-link), so the editor that performs it must be
// physically incapable of touching anything else. No YAML round-trip is
// used for writes — no YAML library guarantees byte-for-byte preservation
// of untouched content (plan §3, "Status mutation"). YAML parsing is for
// validation only, in package adr.
//
// Because the edits are byte-level and hand-rolled, this package is a fuzz
// target (patch_fuzz_test.go): for arbitrary valid ADR bytes, the output
// must differ from the input only on the intended line(s) and must
// re-parse to the same model with only the status fields changed.
package patch

import (
	"errors"
	"strings"
)

// Errors returned when the raw bytes don't contain the structure an edit
// needs. Callers only reach these on malformed input — the mutating verbs
// operate on ADRs that already parsed — but they keep the fuzz contract
// honest (a graceful error, never a panic).
var (
	// ErrNoFrontmatter means the input has no "---"-delimited frontmatter
	// block to edit.
	ErrNoFrontmatter = errors.New("patch: no frontmatter block found")
	// ErrFieldNotFound means the targeted frontmatter field is absent.
	ErrFieldNotFound = errors.New("patch: frontmatter field not found")
)

// bom is the UTF-8 byte-order mark; preserved verbatim as a prefix so a
// BOM'd file round-trips byte-for-byte through an edit.
const bom = "\ufeff"

// line is one physical line split into its content and its terminator, so
// reconstruction (content+term, joined) reproduces the input exactly —
// including whether the final line had a trailing newline and whether the
// terminator was "\n" or "\r\n".
type line struct {
	text string // content, without the line terminator
	term string // "\n", "\r\n", or "" for a final line with no terminator
}

// SetStatus replaces the value of the frontmatter `status:` field, leaving
// every other byte untouched. It is the whole of `deprecate` and the
// old-ADR half of `supersede`'s status flip.
func SetStatus(data []byte, status string) ([]byte, error) {
	return editField(data, "status", status, "")
}

// Supersede sets `status: superseded` and inserts a `superseded-by:
// <supersededBy>` line immediately after the status line, matching the
// status line's indentation and terminator. This is the only edit that
// adds a line; it adds exactly one.
func Supersede(data []byte, supersededBy string) ([]byte, error) {
	return editField(data, "status", "superseded", "superseded-by: "+supersededBy)
}

// SetID replaces the value of the frontmatter `id:` field. This is the one
// permitted id edit — the `renumber` escape hatch (plan §2.6) — and the
// reason it lives here rather than being forbidden outright is that it must
// preserve the rest of the file exactly like any other status transition.
func SetID(data []byte, id string) ([]byte, error) {
	return editField(data, "id", id, "")
}

// editField finds the first top-level `<field>:` line inside the
// frontmatter block, replaces its value with newValue (preserving the key,
// the whitespace around the colon, and the terminator), and — when
// insertAfter is non-empty — inserts it as a new line right after, using
// the same leading indentation and terminator. Everything outside these
// one or two lines is byte-identical to the input.
func editField(data []byte, field, newValue, insertAfter string) ([]byte, error) {
	prefix := ""
	s := string(data)
	if strings.HasPrefix(s, bom) {
		prefix, s = bom, s[len(bom):]
	}

	lines := splitKeepEnds(s)

	closeIdx, ok := frontmatterBounds(lines)
	if !ok {
		return nil, ErrNoFrontmatter
	}

	// Fields live strictly between the opening delimiter (line 0) and the
	// closing one at closeIdx.
	for i := 1; i < closeIdx; i++ {
		indent, key, sep, _, ok := splitField(lines[i].text)
		if !ok || indent != "" || key != field {
			continue
		}
		// Rebuild only the value portion; key + separator + terminator stay.
		lines[i].text = indent + field + sep + newValue

		if insertAfter != "" {
			ins := line{text: indent + insertAfter, term: lines[i].term}
			lines = append(lines, line{})
			copy(lines[i+2:], lines[i+1:])
			lines[i+1] = ins
		}
		return []byte(prefix + join(lines)), nil
	}
	return nil, ErrFieldNotFound
}

// frontmatterBounds returns the index of the closing "---" delimiter line,
// given that the opening one is always line 0. A field line lives strictly
// between line 0 and closeAt. ok is false when there is no well-formed
// frontmatter block (no opening or no closing delimiter).
func frontmatterBounds(lines []line) (closeAt int, ok bool) {
	if len(lines) == 0 || lines[0].text != "---" {
		return 0, false
	}
	for i := 1; i < len(lines); i++ {
		if lines[i].text == "---" {
			return i, true
		}
	}
	return 0, false
}

// splitField parses "  key: value" into its parts. sep is the exact text
// between the key and the value (the colon plus following spaces), so it
// can be reproduced verbatim. ok is false when the line has no "key:"
// shape.
func splitField(text string) (indent, key, sep, value string, ok bool) {
	i := 0
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	indent = text[:i]
	rest := text[i:]
	colon := strings.IndexByte(rest, ':')
	if colon <= 0 {
		return "", "", "", "", false
	}
	key = rest[:colon]
	if strings.ContainsAny(key, " \t") {
		return "", "", "", "", false
	}
	afterColon := rest[colon+1:]
	j := 0
	for j < len(afterColon) && (afterColon[j] == ' ' || afterColon[j] == '\t') {
		j++
	}
	sep = rest[colon : colon+1+j]
	value = afterColon[j:]
	return indent, key, sep, value, true
}

// splitKeepEnds splits s into lines that retain their terminators, such
// that concatenating every text+term reproduces s exactly. A trailing "\n"
// does not yield a spurious empty final line.
func splitKeepEnds(s string) []line {
	var lines []line
	i := 0
	for i < len(s) {
		nl := strings.IndexByte(s[i:], '\n')
		if nl < 0 {
			lines = append(lines, line{text: s[i:], term: ""})
			break
		}
		end := i + nl
		content, term := s[i:end], "\n"
		if strings.HasSuffix(content, "\r") {
			content, term = content[:len(content)-1], "\r\n"
		}
		lines = append(lines, line{text: content, term: term})
		i = end + 1
	}
	return lines
}

// join reconstructs the file bytes from the line slice.
func join(lines []line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.text)
		b.WriteString(l.term)
	}
	return b.String()
}
