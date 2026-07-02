package render

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// header is static (no template data), so it's a plain constant rather
// than part of the text/template body below. Ends with exactly one
// newline; renderTemplate controls all further blank-line spacing in Go.
const header = `<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". To change a rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution
`

// adrTmplText renders one active ADR's projected entry: title as a rule
// heading, the Decision Outcome body, then a metadata line (plan §4).
// Blank-line spacing *between* entries/categories is assembled in Go
// (renderTemplate) rather than fought over in template whitespace-trim
// syntax, so it stays easy to reason about and byte-exact.
const adrTmplText = `### {{.Title}}

{{.DecisionOutcome}}

{{.MetaLine}}
`

var adrTmpl = template.Must(template.New("adr").Parse(adrTmplText))

type tmplADR struct {
	Title           string
	DecisionOutcome string
	MetaLine        string
}

// renderTemplate executes the projection template over pre-sorted
// sections built by Group and assembles the final byte-exact output.
func renderTemplate(sections []CategorySection) ([]byte, error) {
	catChunks := make([]string, 0, len(sections))
	for _, s := range sections {
		adrChunks := make([]string, 0, len(s.ADRs))
		for _, a := range s.ADRs {
			data := tmplADR{
				Title:           a.Title,
				DecisionOutcome: a.Sections[adr.DecisionOutcomeSection],
				MetaLine:        metaLine(a),
			}
			var buf bytes.Buffer
			if err := adrTmpl.Execute(&buf, data); err != nil {
				return nil, fmt.Errorf("render constitution.md: ADR %s: %w", a.ID, err)
			}
			adrChunks = append(adrChunks, strings.TrimRight(buf.String(), "\n"))
		}
		catChunks = append(catChunks, "## "+s.Name+"\n\n"+strings.Join(adrChunks, "\n\n"))
	}

	out := header
	if body := strings.Join(catChunks, "\n\n"); body != "" {
		out += "\n" + body + "\n"
	}
	return []byte(out), nil
}

// metaLine formats an active ADR's metadata line: "ADR-0007 · 2026-07-01
// · source FS-0042", omitting the source segment when absent
// (implementation-plan.md §4).
func metaLine(a adr.ADR) string {
	line := a.ID + " · " + a.Date
	if a.Source != "" {
		line += " · source " + a.Source
	}
	return line
}
