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
  regen". Only rule-bearing (## Rule) active ADRs project here; to change a
  rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution
`

// placeholderLine is the sole body line when no active ADR is rule-bearing
// (plan §2.12): the constitution is empty of standing rules, and the reader
// is pointed at the decision log.
const placeholderLine = "No standing rules yet. Decision log: constitution/adr/."

// adrTmplText renders one rule-bearing ADR's projected entry: title as a rule
// heading, the Rule section body verbatim, then a metadata line (plan §2.12,
// §4). The Decision Outcome no longer projects. Blank-line spacing *between*
// entries/categories is assembled in Go (renderTemplate) rather than fought
// over in template whitespace-trim syntax, so it stays byte-exact.
const adrTmplText = `### {{.Title}}

{{.Rule}}

{{.MetaLine}}
`

var adrTmpl = template.Must(template.New("adr").Parse(adrTmplText))

type tmplADR struct {
	Title    string
	Rule     string
	MetaLine string
}

// renderTemplate executes the projection template over pre-sorted
// sections built by Group and assembles the final byte-exact output.
func renderTemplate(sections []CategorySection) ([]byte, error) {
	catChunks := make([]string, 0, len(sections))
	for _, s := range sections {
		adrChunks := make([]string, 0, len(s.ADRs))
		for _, a := range s.ADRs {
			data := tmplADR{
				Title:    a.Title,
				Rule:     a.Rule(),
				MetaLine: metaLine(a),
			}
			var buf bytes.Buffer
			if err := adrTmpl.Execute(&buf, data); err != nil {
				return nil, fmt.Errorf("render constitution.md: ADR %s: %w", a.ID, err)
			}
			adrChunks = append(adrChunks, strings.TrimRight(buf.String(), "\n"))
		}
		catChunks = append(catChunks, "## "+s.Name+"\n\n"+strings.Join(adrChunks, "\n\n"))
	}

	body := strings.Join(catChunks, "\n\n")
	if body == "" {
		// No rule-bearing active ADR: render the placeholder (plan §2.12).
		body = placeholderLine
	}
	return []byte(header + "\n" + body + "\n"), nil
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
