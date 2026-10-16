// Package scaffold implements `constitution init`'s side of the build: it
// fans the embedded Layer-2 skills out into a repo's agent-skill trees,
// drift-protected via constitution/.state (implementation-plan.md §2.2, §6),
// and removes the managed pointer blocks that CLI versions before 0.4.0
// wrote into agent-instruction files (CLAUDE.md, AGENTS.md).
//
// The managed-block editor here is deliberately byte-precise: it locates a
// block by an exact marker pair and cuts only that block (plus what
// ApplyBlock added around it) — the doctoc/terraform-docs/ansible
// blockinfile pattern (plan §2.2). The marker text is byte-stable across CLI
// versions (a documented ansible-blockinfile failure mode), which is what
// lets 0.4.0 find and remove blocks an older CLI wrote.
package scaffold

import (
	"fmt"
	"strings"
)

// BlockBegin and BlockEnd are the managed-block markers (plan §2.2). They
// are byte-exact and MUST NOT change across CLI versions: locating a block
// depends on finding this exact pair. A strategy change migrates via the
// `v1` token, not by editing the surrounding text.
const (
	BlockBegin = "<!-- BEGIN adr-sourced-constitution v1 (managed — do not edit by hand; `constitution init` updates it) -->"
	BlockEnd   = "<!-- END adr-sourced-constitution v1 -->"
)

// MarkerError reports a malformed marker pair in a managed target: a BEGIN
// with no END, an END with no BEGIN, a reversed pair, or more than one
// block. It is a distinct type so callers can recognise a structurally
// ambiguous file and leave it untouched — guessing which bytes are the
// managed region would be exactly the wrong thing to do.
type MarkerError struct {
	// Path is the file the malformed markers were found in (set by the
	// caller that read the file; empty when LocateBlock is used directly).
	Path string
	msg  string
}

func (e *MarkerError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("managed block in %s: %s", e.Path, e.msg)
	}
	return "managed block: " + e.msg
}

// LocateBlock finds the managed block in content. When neither marker is
// present it returns found=false with a nil error (an unmanaged file is a
// legitimate append target). A malformed pair returns a *MarkerError.
//
// On success, content[begin:end] is the exact byte span of the whole block
// (both marker lines and the interior between them), and interior is the
// text between the markers with surrounding newlines trimmed (CR and LF), so
// a CRLF-authored block yields the same interior as its LF twin.
func LocateBlock(content []byte) (found bool, begin, end int, interior string, err error) {
	s := string(content)
	b := strings.Index(s, BlockBegin)
	e := strings.Index(s, BlockEnd)

	switch {
	case b < 0 && e < 0:
		return false, 0, 0, "", nil
	case b < 0:
		return false, 0, 0, "", &MarkerError{msg: "END marker present without a matching BEGIN marker"}
	case e < 0:
		return false, 0, 0, "", &MarkerError{msg: "BEGIN marker present without a matching END marker"}
	case e < b:
		return false, 0, 0, "", &MarkerError{msg: "END marker appears before BEGIN marker"}
	}

	// A second BEGIN after the END means two managed blocks — ambiguous;
	// refuse rather than silently editing only the first.
	if strings.Contains(s[e+len(BlockEnd):], BlockBegin) {
		return false, 0, 0, "", &MarkerError{msg: "more than one managed block found"}
	}

	interiorRaw := s[b+len(BlockBegin) : e]
	interior = strings.Trim(interiorRaw, "\r\n")
	end = e + len(BlockEnd)
	return true, b, end, interior, nil
}

// RenderBlock renders a managed block for the given interior, WITHOUT a
// trailing newline (surrounding-content newlines are the caller's concern in
// ApplyBlock). The interior sits on its own line(s) between the markers.
func RenderBlock(interior string) string {
	return BlockBegin + "\n" + interior + "\n" + BlockEnd
}

// ApplyBlock returns content with the managed block set to interior. If a
// block is present its span is replaced in place (everything before BEGIN
// and after END is preserved byte-for-byte); otherwise a fresh block is
// appended at EOF, separated from any existing content by a blank line, with
// the file left newline-terminated. A malformed marker pair is a
// *MarkerError.
//
// No CLI path writes blocks since 0.4.0; ApplyBlock remains the reference
// for the exact shape older CLIs appended, which RemoveBlock undoes.
func ApplyBlock(content []byte, interior string) ([]byte, error) {
	found, begin, end, _, err := LocateBlock(content)
	if err != nil {
		return nil, err
	}
	block := RenderBlock(interior)

	if found {
		out := make([]byte, 0, begin+len(block)+(len(content)-end))
		out = append(out, content[:begin]...)
		out = append(out, block...)
		out = append(out, content[end:]...)
		return out, nil
	}

	if len(content) == 0 {
		return []byte(block + "\n"), nil
	}
	out := make([]byte, 0, len(content)+len(block)+2)
	out = append(out, content...)
	if out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, '\n') // blank line between existing content and the block
	out = append(out, block...)
	out = append(out, '\n') // keep the file newline-terminated
	return out, nil
}

// RemoveBlock returns content with the managed block cut out. removed is
// false (and content is returned unchanged) when no block is present; a
// malformed marker pair is a *MarkerError.
//
// A block at EOF — where ApplyBlock appends one — is cut together with
// exactly what ApplyBlock added: the blank-line separator before it and the
// line ending after it. Exact byte restoration is not possible (ApplyBlock
// may have added a newline to an unterminated file, and always wrote LF), so
// the result is the pre-append content, newline-terminated. A block found
// mid-file (moved by hand) is cut together with the one line ending that
// follows it (CRLF or LF); the surrounding bytes are untouched.
func RemoveBlock(content []byte) (out []byte, removed bool, err error) {
	found, begin, end, _, err := LocateBlock(content)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return content, false, nil
	}

	before, after := content[:begin], content[end:]
	after = trimLeadingEOL(after)

	if len(after) == 0 {
		// EOF block: drop the blank-line separator ApplyBlock added, i.e. the
		// last line ending of `before` when the line before it is also ended.
		if trimmed := trimTrailingEOL(before); len(trimmed) < len(before) && endsWithEOL(trimmed) {
			before = trimmed
		}
	}

	out = make([]byte, 0, len(before)+len(after))
	out = append(out, before...)
	out = append(out, after...)
	return out, true, nil
}

// trimLeadingEOL drops one leading line ending (CRLF or LF) from b.
func trimLeadingEOL(b []byte) []byte {
	switch {
	case len(b) >= 2 && b[0] == '\r' && b[1] == '\n':
		return b[2:]
	case len(b) >= 1 && b[0] == '\n':
		return b[1:]
	}
	return b
}

// trimTrailingEOL drops one trailing line ending (CRLF or LF) from b.
func trimTrailingEOL(b []byte) []byte {
	n := len(b)
	switch {
	case n >= 2 && b[n-2] == '\r' && b[n-1] == '\n':
		return b[:n-2]
	case n >= 1 && b[n-1] == '\n':
		return b[:n-1]
	}
	return b
}

// endsWithEOL reports whether b ends with a line ending.
func endsWithEOL(b []byte) bool {
	return len(b) > 0 && b[len(b)-1] == '\n'
}
