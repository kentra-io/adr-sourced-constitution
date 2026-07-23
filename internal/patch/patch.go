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
	"reflect"
	"strings"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
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
	// ErrValueNotOnKeyLine means the field's value does not start on the
	// key's own line — yaml permits "status:\n accepted" (a multi-line
	// plain scalar) and "status: # comment\n accepted", where the real
	// value lives on a continuation line. A single-line editor cannot
	// rewrite such a value without corrupting the file (found by
	// FuzzSupersede), so it refuses instead; the file is left untouched.
	// Normalizing the field to one-line "key: value" form by hand is a
	// legal edit: it changes neither the parsed model nor the manifest
	// hash, and the status line is the mutable line.
	//
	// This is the FRIENDLY pre-edit heuristic for the common multi-line
	// forms; ErrUnsafeEdit below is the mechanical backstop that closes
	// the whole class.
	ErrValueNotOnKeyLine = errors.New("patch: field value does not start on the key line; normalize the field to single-line \"key: value\" form first")
	// ErrUnsafeEdit means post-edit verification failed: the patched bytes
	// either no longer parse, or re-parse to a model differing from the
	// original in more than the intended field change. This closes the
	// multi-line-value class MECHANICALLY, whatever yaml construct caused
	// it — found by review with a double-quoted scalar using a backslash
	// line continuation (`status: "accep\` + newline + `ted"` parses as
	// accepted, slips past the on-line-value heuristic, and a naive edit
	// orphans the continuation). Nothing is written when this is returned.
	ErrUnsafeEdit = errors.New("patch: refusing edit: patched content does not re-parse to the same ADR with only the intended change (the field value likely spans multiple lines via a yaml construct; normalize it to single-line \"key: value\" form first)")
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
	out, err := editField(data, "status", status, "")
	if err != nil {
		return nil, err
	}
	if err := verify(data, out, func(o, p *adr.ADR) bool {
		return p.Status == adr.Status(status) &&
			p.SupersededBy == o.SupersededBy &&
			p.ID == o.ID &&
			sameFrozen(o, p)
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// Supersede sets `status: superseded` and inserts a `superseded-by:
// <supersededBy>` line immediately after the status line, matching the
// status line's indentation and terminator. This is the only edit that
// adds a line; it adds exactly one.
func Supersede(data []byte, supersededBy string) ([]byte, error) {
	out, err := editField(data, "status", "superseded", "superseded-by: "+supersededBy)
	if err != nil {
		return nil, err
	}
	if err := verify(data, out, func(o, p *adr.ADR) bool {
		return p.Status == adr.StatusSuperseded &&
			p.SupersededBy == supersededBy &&
			p.ID == o.ID &&
			sameFrozen(o, p)
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// SetID replaces the value of the frontmatter `id:` field. This is the one
// permitted id edit — the `renumber` escape hatch (plan §2.6) — and the
// reason it lives here rather than being forbidden outright is that it must
// preserve the rest of the file exactly like any other status transition.
func SetID(data []byte, id string) ([]byte, error) {
	out, err := editField(data, "id", id, "")
	if err != nil {
		return nil, err
	}
	if err := verify(data, out, func(o, p *adr.ADR) bool {
		return p.ID == id &&
			p.Status == o.Status &&
			p.SupersededBy == o.SupersededBy &&
			sameFrozen(o, p)
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// verify is the single post-edit choke point every exported edit passes
// through BEFORE its result can reach a caller (and therefore before any
// verb writes anything): re-parse the patched bytes and require the model
// to equal the pre-patch model except exactly the intended change, per the
// op-specific `intended` predicate. A byte editor cannot see every yaml
// construct that lets a value span lines (block scalars, quoted
// continuations, tags, ...); this guard makes them all refuse identically
// instead of corrupting the log.
//
// When the ORIGINAL bytes are not a valid ADR, verification is skipped:
// there is no model to protect, and the mutating verbs never reach patch
// with an unparsed file (they Parse first). This keeps the package usable
// on minimal fixtures and keeps the fuzz contract ("never panic, any
// error is fine") unchanged.
func verify(orig, patched []byte, intended func(o, p *adr.ADR) bool) error {
	o, err := adr.ParseBytesUnnamed(orig, "patch-input")
	if err != nil {
		return nil
	}
	p, err := adr.ParseBytesUnnamed(patched, "patch-output")
	if err != nil {
		return ErrUnsafeEdit
	}
	if !intended(o, p) {
		return ErrUnsafeEdit
	}
	return nil
}

// sameFrozen reports whether the frozen (immutable, spec §5.2) parts of
// two parsed ADRs are identical: all frontmatter except status and
// superseded-by (id equality is asserted per-op, since SetID legitimately
// changes it), and the entire body.
func sameFrozen(o, p *adr.ADR) bool {
	return o.Title == p.Title &&
		o.Date == p.Date &&
		o.Source == p.Source &&
		o.Supersedes == p.Supersedes &&
		reflect.DeepEqual(o.SupersedesRules, p.SupersedesRules) &&
		reflect.DeepEqual(o.RemovesRules, p.RemovesRules) &&
		reflect.DeepEqual(o.Sections, p.Sections) &&
		reflect.DeepEqual(o.SectionOrder, p.SectionOrder)
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
		indent, key, sep, ok := splitField(lines[i].text)
		if !ok || indent != "" || key != field {
			continue
		}
		// The value is whatever follows indent+key+sep on this line. If it
		// is empty or a comment, the real value lives on a yaml
		// continuation line, which a single-line edit would corrupt —
		// refuse (see ErrValueNotOnKeyLine).
		value := lines[i].text[len(indent)+len(key)+len(sep):]
		if value == "" || strings.HasPrefix(value, "#") {
			return nil, ErrValueNotOnKeyLine
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
// frontmatter block (no opening or no closing delimiter). Delimiter
// recognition is delegated to adr.IsFrontmatterDelimiter — the single
// boundary rule shared with the parser, so the two scanners cannot drift.
func frontmatterBounds(lines []line) (closeAt int, ok bool) {
	if len(lines) == 0 || !adr.IsFrontmatterDelimiter(lines[0].text) {
		return 0, false
	}
	for i := 1; i < len(lines); i++ {
		if adr.IsFrontmatterDelimiter(lines[i].text) {
			return i, true
		}
	}
	return 0, false
}

// splitField parses "  key: value" into its parts, mirroring what the yaml
// parser accepts for a block-mapping entry (verified empirically against
// go.yaml.in/yaml/v3): whitespace is permitted BEFORE the colon ("status :
// accepted", "status\t: accepted" both parse as key "status"), so the key
// is right-trimmed before comparison — otherwise an ADR the parser accepts
// could never be superseded/deprecated. A key with interior whitespace
// ("sta tus:") parses in yaml as the literal key "sta tus", which is never
// one of our schema fields, so it is rejected here as a non-target line.
//
// sep is the exact text between the trimmed key and the value (any
// whitespace before the colon, the colon, and any following whitespace), so
// reconstruction (indent + key + sep + newValue) reproduces the original
// spacing verbatim; the old value is whatever followed sep and is simply
// replaced. ok is false when the line has no "key:" shape.
func splitField(text string) (indent, key, sep string, ok bool) {
	i := 0
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	indent = text[:i]
	rest := text[i:]
	colon := strings.IndexByte(rest, ':')
	if colon <= 0 {
		return "", "", "", false
	}
	key = strings.TrimRight(rest[:colon], " \t")
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", "", false
	}
	afterColon := rest[colon+1:]
	j := 0
	for j < len(afterColon) && (afterColon[j] == ' ' || afterColon[j] == '\t') {
		j++
	}
	sep = rest[len(key) : colon+1+j]
	return indent, key, sep, true
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

// Unsupersede reverses Supersede: status back to `accepted`, and the
// derived `superseded-by:` line removed. It is the heal half of draft-phase
// `adr rm` (v0.2 proposal §3): deleting a superseding ADR restores the
// decision it had replaced. Tolerant of an absent superseded-by line and of
// an already-accepted status, so a crash between rm's heal write and its
// file delete converges on re-run instead of refusing.
func Unsupersede(data []byte) ([]byte, error) {
	out, err := editField(data, "status", string(adr.StatusAccepted), "")
	if err != nil {
		return nil, err
	}
	out = removeField(out, "superseded-by")
	if err := verify(data, out, func(o, p *adr.ADR) bool {
		return p.Status == adr.StatusAccepted &&
			p.SupersededBy == "" &&
			p.ID == o.ID &&
			sameFrozen(o, p)
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// removeField deletes the first top-level `<field>:` line inside the
// frontmatter block, leaving every other byte untouched. A missing field is
// a no-op, not an error — Unsupersede's convergence depends on that.
func removeField(data []byte, field string) []byte {
	prefix := ""
	s := string(data)
	if strings.HasPrefix(s, bom) {
		prefix, s = bom, s[len(bom):]
	}
	lines := splitKeepEnds(s)
	closeIdx, ok := frontmatterBounds(lines)
	if !ok {
		return data
	}
	for i := 1; i < closeIdx; i++ {
		indent, key, _, ok := splitField(lines[i].text)
		if !ok || indent != "" || key != field {
			continue
		}
		return []byte(prefix + join(append(lines[:i], lines[i+1:]...)))
	}
	return data
}
