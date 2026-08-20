package render

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// header is static (no template data), so it's a plain constant rather
// than part of the text/template body below. Ends with exactly one
// newline; renderTemplate controls all further blank-line spacing in Go.
const header = `<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". Only the rules (## Rules entries) of active ADRs project here; to
  change a rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution
`

// preamble is the D2 goal statement (proposal v0.2): every rendered
// constitution states what the document is for, so any agent reading it
// knows what belongs in it.
const preamble = `The source of truth for this project's standing technical decisions — how
recurring problems are solved (architecture, mapping, testing, process) — so
that requirements can stay functional and need not re-explain implementation
choices.
`

// decisionLogPointer tells the reader where the full decision log
// lives. It renders in BOTH forms of the projection: the placeholder
// has always carried it, and a populated constitution.md needs it
// just as much — only rule-bearing ADRs project, so every record-only
// ADR is otherwise invisible to a reader of this file (issue #24).
const decisionLogPointer = "Decision log: constitution/adr/."

// placeholderLine is the sole body line when no active ADR carries a
// rule: the constitution is empty of standing rules, and the reader is
// pointed at the decision log.
const placeholderLine = "No standing rules yet. " + decisionLogPointer

// entryTmplText renders one projected rule entry: its slug as the rule
// heading, the rule text verbatim, then the carrying ADR's metadata line.
// Blank-line spacing *between* entries/categories is assembled in Go
// (renderTemplate) rather than fought over in template whitespace-trim
// syntax, so it stays byte-exact.
const entryTmplText = `### {{.Slug}}

{{.Text}}

{{.MetaLine}}
`

var entryTmpl = template.Must(template.New("entry").Parse(entryTmplText))

type tmplEntry struct {
	Slug     string
	Text     string
	MetaLine string
}

// renderTemplate executes the projection template over pre-sorted
// sections built by Group and assembles the final byte-exact output.
func renderTemplate(sections []CategorySection) ([]byte, error) {
	catChunks := make([]string, 0, len(sections))
	for _, s := range sections {
		entryChunks := make([]string, 0, len(s.Entries))
		for _, e := range s.Entries {
			data := tmplEntry{
				Slug:     e.Rule.Slug,
				Text:     e.Rule.Text,
				MetaLine: metaLine(e),
			}
			var buf bytes.Buffer
			if err := entryTmpl.Execute(&buf, data); err != nil {
				return nil, fmt.Errorf("render constitution.md: rule %s/%s/%s: %w",
					e.ADR.ID, e.Rule.Category, e.Rule.Slug, err)
			}
			entryChunks = append(entryChunks, strings.TrimRight(buf.String(), "\n"))
		}
		catChunks = append(catChunks, "## "+s.Name+"\n\n"+strings.Join(entryChunks, "\n\n"))
	}

	body := strings.Join(catChunks, "\n\n")
	if body == "" {
		// No projected rule from any active ADR: the placeholder line
		// already carries the pointer.
		body = placeholderLine
	} else {
		// Populated: the pointer goes directly after the preamble,
		// exactly where the placeholder form puts it, so both renders
		// share one structure.
		body = decisionLogPointer + "\n\n" + body
	}
	return []byte(header + "\n" + preamble + "\n" + body + "\n"), nil
}

// metaLine formats a projected entry's metadata line from its carrying
// ADR: "ADR-0007 · 2026-07-01 · source FS-0042", omitting the source
// segment when absent (implementation-plan.md §4).
func metaLine(e Entry) string {
	line := e.ADR.ID + " · " + e.ADR.Date
	if e.ADR.Source != "" {
		line += " · source " + e.ADR.Source
	}
	return line
}
