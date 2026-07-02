package adr

import (
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

// Parse reads and validates one ADR file, returning the file/line/field
// *ParseError from the first validation failure encountered.
func Parse(path string) (*ADR, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fm, body, err := SplitFrontmatter(data)
	if err != nil {
		msg := "file must start with a \"---\" frontmatter delimiter line"
		if errors.Is(err, errNoClosingDelimiter) {
			msg = "frontmatter is not terminated: no closing \"---\" delimiter line found"
		}
		return nil, &ParseError{File: path, Line: 1, Msg: msg}
	}

	m, err := parseMeta(fm, path)
	if err != nil {
		return nil, err
	}

	sections, order := ExtractSections(body)
	if err := validateSections(sections, requiredForRegen, path); err != nil {
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
	if fnID != m.ID {
		return nil, &ParseError{
			File: path, Field: "id", Line: fieldLine(fm, "id"),
			Msg: fmt.Sprintf("frontmatter id %q does not match filename-derived id %q", m.ID, fnID),
		}
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
	}, nil
}

// ParseDir parses every "*.md" file directly under dir (constitution/adr/,
// non-recursive — spec §4.3), sorted by filename for deterministic error
// ordering. It fails fast on the first malformed file, returning that
// file's precise *ParseError.
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
	for _, name := range names {
		a, err := Parse(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		adrs = append(adrs, *a)
	}
	return adrs, nil
}
