package adr

import (
	"fmt"
	"strings"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

// rawMeta is the literal frontmatter schema (spec §4.1) for yaml
// unmarshaling; string-typed throughout so semantic validation (id
// format, date format, status enum) happens explicitly below with
// precise errors, rather than relying on yaml decode-time type errors.
type rawMeta struct {
	ID              string   `yaml:"id"`
	Title           string   `yaml:"title"`
	Date            string   `yaml:"date"`
	Status          string   `yaml:"status"`
	Source          string   `yaml:"source"`
	Supersedes      string   `yaml:"supersedes"`
	SupersededBy    string   `yaml:"superseded-by"`
	SupersedesRules []string `yaml:"supersedes-rules"`
	RemovesRules    []string `yaml:"removes-rules"`
}

// meta is a validated rawMeta: every field has passed schema validation.
type meta struct {
	ID, Title, Date                  string
	Status                           Status
	Source, Supersedes, SupersededBy string
	SupersedesRules, RemovesRules    []RuleRef
}

// requiredFrontmatterFields lists the mandatory frontmatter fields (spec
// §4.1): id, title, date, status. `source` is conditionally required by
// sourceTracking config (plan §2.8), which is a cross-cutting concern the
// write path (M2's `adr new`) owns — not validated here.
// `supersedes`/`superseded-by`/`supersedes-rules`/`removes-rules` are
// optional derived/authored fields.
var requiredFrontmatterFields = []struct {
	name string
	get  func(rawMeta) string
}{
	{"id", func(m rawMeta) string { return m.ID }},
	{"title", func(m rawMeta) string { return m.Title }},
	{"date", func(m rawMeta) string { return m.Date }},
	{"status", func(m rawMeta) string { return m.Status }},
}

// parseMeta unmarshals and validates an ADR's frontmatter block. It
// returns the first validation failure as a precise *ParseError.
func parseMeta(fm []byte, file string) (*meta, error) {
	var raw rawMeta
	if err := yaml.Unmarshal(fm, &raw); err != nil {
		return nil, &ParseError{
			File: file,
			Line: yamlErrorLine(err),
			Msg:  fmt.Sprintf("frontmatter is not valid YAML: %s", err),
		}
	}

	for _, f := range requiredFrontmatterFields {
		if strings.TrimSpace(f.get(raw)) == "" {
			return nil, &ParseError{File: file, Field: f.name, Msg: "is required"}
		}
	}

	if _, ok := parseID(raw.ID); !ok {
		return nil, &ParseError{
			File: file, Field: "id", Line: fieldLine(fm, "id"),
			Msg: fmt.Sprintf("must match \"ADR-NNNN\" (got %q)", raw.ID),
		}
	}

	if _, err := time.Parse("2006-01-02", raw.Date); err != nil {
		return nil, &ParseError{
			File: file, Field: "date", Line: fieldLine(fm, "date"),
			Msg: fmt.Sprintf("must be an ISO-8601 date YYYY-MM-DD (got %q)", raw.Date),
		}
	}

	status := Status(raw.Status)
	switch status {
	case StatusAccepted, StatusSuperseded, StatusDeprecated:
	default:
		return nil, &ParseError{
			File: file, Field: "status", Line: fieldLine(fm, "status"),
			Msg: fmt.Sprintf("must be one of %q, %q, %q (got %q)",
				StatusAccepted, StatusSuperseded, StatusDeprecated, raw.Status),
		}
	}

	supRefs, err := parseRefList(raw.SupersedesRules, "supersedes-rules", fm, file)
	if err != nil {
		return nil, err
	}
	remRefs, err := parseRefList(raw.RemovesRules, "removes-rules", fm, file)
	if err != nil {
		return nil, err
	}

	return &meta{
		ID: raw.ID, Title: raw.Title, Date: raw.Date,
		Status: status, Source: raw.Source, Supersedes: raw.Supersedes,
		SupersededBy:    raw.SupersededBy,
		SupersedesRules: supRefs, RemovesRules: remRefs,
	}, nil
}

// parseRefList validates every entry of a frontmatter rule-ref list
// (supersedes-rules / removes-rules), returning a precise *ParseError
// naming the field and the malformed entry.
func parseRefList(entries []string, field string, fm []byte, file string) ([]RuleRef, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	refs := make([]RuleRef, 0, len(entries))
	for _, e := range entries {
		r, err := ParseRuleRef(strings.TrimSpace(e))
		if err != nil {
			return nil, &ParseError{File: file, Field: field, Line: fieldLine(fm, field), Msg: err.Error()}
		}
		refs = append(refs, r)
	}
	return refs, nil
}
