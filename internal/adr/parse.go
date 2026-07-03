package adr

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// requiredForRegen is the subset of MandatorySections the M1 read path
// enforces. Full MADR body-shape enforcement (all of MandatorySections)
// is M2's job on the write path (`adr new`); M1 only needs to locate
// Decision Outcome, since it's the sole section the projection renders
// (spec §6 step 4; implementation-plan.md instructs the parser to expose
// section extraction + a validate function M2 can reuse, which
// validateSections in body.go provides).
var requiredForRegen = []string{DecisionOutcomeSection}

// utf8BOM is the UTF-8 byte-order mark some editors prepend; normalize
// strips it so a BOM'd ADR file still parses.
var utf8BOM = []byte{0xef, 0xbb, 0xbf}

// normalize canonicalizes raw ADR content before parsing: strips a
// leading UTF-8 BOM and converts CRLF line endings to LF. Everything
// downstream (frontmatter split, section extraction, and ultimately the
// rendered constitution.md) therefore sees LF-only content — the
// projection's bytes must not depend on the line endings an ADR author's
// editor happened to write.
func normalize(data []byte) []byte {
	data = bytes.TrimPrefix(data, utf8BOM)
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// Parse reads and validates one ADR file, returning the file/line/field
// *ParseError from the first validation failure encountered.
func Parse(path string) (*ADR, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseBytes(data, path)
}

// ParseBytes validates raw ADR content against the schema. path is used
// for error messages and the filename<->id cross-check only — no file is
// read, so callers can parse content from any source (M3's guard parses
// git blobs, not working-tree files).
func ParseBytes(data []byte, path string) (*ADR, error) {
	a, fm, err := parseBytesCore(data, path)
	if err != nil {
		return nil, err
	}

	base := filepath.Base(path)
	fnID, ok := filenameID(base)
	if !ok {
		return nil, &ParseError{
			File: path,
			Msg:  fmt.Sprintf("filename %q does not match the required \"ADR-NNNN-slug.md\" pattern", base),
		}
	}
	if fnID != a.ID {
		return nil, &ParseError{
			File: path, Field: "id", Line: fieldLine(fm, "id"),
			Msg: fmt.Sprintf("frontmatter id %q does not match filename-derived id %q", a.ID, fnID),
		}
	}
	return a, nil
}

// ParseBytesUnnamed validates ADR content that exists only in memory and
// has no meaningful filename: the filename<->id cross-check is skipped,
// every other check applies. Its caller class is internal/patch's
// post-edit verification, which must re-validate patched bytes before
// anything is written; on-disk content must always go through
// ParseBytes/Parse. pathForErrors only labels error messages.
func ParseBytesUnnamed(data []byte, pathForErrors string) (*ADR, error) {
	a, _, err := parseBytesCore(data, pathForErrors)
	return a, err
}

// parseBytesCore is the shared parse pipeline: normalize, frontmatter
// split, schema validation, section extraction. It also returns the raw
// frontmatter block so ParseBytes can report a precise line number for its
// filename<->id cross-check.
func parseBytesCore(data []byte, path string) (*ADR, []byte, error) {
	data = normalize(data)

	fm, body, err := SplitFrontmatter(data)
	if err != nil {
		msg := "file must start with a \"---\" frontmatter delimiter line"
		if errors.Is(err, errNoClosingDelimiter) {
			msg = "frontmatter is not terminated: no closing \"---\" delimiter line found"
		}
		return nil, nil, &ParseError{File: path, Line: 1, Msg: msg}
	}

	m, err := parseMeta(fm, path)
	if err != nil {
		return nil, nil, err
	}

	sections, order := ExtractSections(body)
	if err := validateSections(sections, requiredForRegen, path); err != nil {
		return nil, nil, err
	}
	if err := validateRuleSection(sections, path); err != nil {
		return nil, nil, err
	}

	num, _ := parseID(m.ID) // format already validated in parseMeta

	return &ADR{
		ID:           m.ID,
		Num:          num,
		Title:        m.Title,
		Category:     m.Category,
		Date:         m.Date,
		Status:       m.Status,
		Source:       m.Source,
		Supersedes:   m.Supersedes,
		SupersededBy: m.SupersededBy,
		Sections:     sections,
		SectionOrder: order,
		Path:         path,
	}, fm, nil
}

// ParseDir parses every "*.md" file directly under dir (constitution/adr/,
// non-recursive — spec §4.3), sorted by filename for deterministic error
// ordering. It fails fast on the first malformed file, returning that
// file's precise *ParseError. Because it sees the full set, it also owns
// the cross-file invariant a single-file Parse cannot check: ids must be
// unique across the log (two files sharing an id would both render, in
// filename order — a nondeterministic-in-principle projection).
func ParseDir(dir string) ([]ADR, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	adrs := make([]ADR, 0, len(names))
	seen := make(map[string]string, len(names)) // id -> path of first file using it
	for _, name := range names {
		path := filepath.Join(dir, name)
		a, err := Parse(path)
		if err != nil {
			return nil, err
		}
		if first, dup := seen[a.ID]; dup {
			return nil, &ParseError{
				File: path, Field: "id",
				Msg: fmt.Sprintf("duplicate id %q (already used by %s)", a.ID, first),
			}
		}
		seen[a.ID] = path
		adrs = append(adrs, *a)
	}
	return adrs, nil
}
