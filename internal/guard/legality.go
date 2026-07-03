package guard

import (
	"fmt"
	"reflect"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// frozenFields lists the frontmatter fields that must never differ between
// the base-ref and current versions of an existing ADR (spec §5.2/§5.3:
// "body + other frontmatter frozen"). id is deliberately absent: a same-path
// id change cannot reach compareLegal in the first place (ParseBytes
// cross-checks filename<->id, so a changed id with an unchanged filename
// fails to parse and surfaces as a "guard could not run" error instead —
// see scanDir/checkGit); an id change WITH a filename change is a rename,
// handled entirely by the file_renamed path in checkGit and never reaches
// compareLegal at all.
var frozenFields = []struct {
	name string
	get  func(*adr.ADR) string
}{
	{"title", func(a *adr.ADR) string { return a.Title }},
	{"category", func(a *adr.ADR) string { return a.Category }},
	{"date", func(a *adr.ADR) string { return a.Date }},
	{"source", func(a *adr.ADR) string { return a.Source }},
	{"supersedes", func(a *adr.ADR) string { return a.Supersedes }},
}

// compareLegal structurally compares the base-ref (old) and current (cur)
// parsed versions of one modified ADR file and returns a violation for
// every difference the guard (spec §5.3) does not permit: any body change,
// any frozen-frontmatter field change, or a status/superseded-by change
// that isn't a legal accepted->superseded|deprecated transition (spec
// §5.2, plan §2.7). id is asserted equal by the caller's contract (see
// frozenFields doc); it is not re-checked here.
func compareLegal(id, file string, old, cur *adr.ADR) []Violation {
	var vs []Violation

	if !reflect.DeepEqual(old.Sections, cur.Sections) || !reflect.DeepEqual(old.SectionOrder, cur.SectionOrder) {
		vs = append(vs, Violation{
			Kind: KindBodyChanged, ID: id, File: file,
			Message: fmt.Sprintf("%s: body changed; only status (and its derived superseded-by) may change on an existing ADR", id),
		})
	}

	for _, f := range frozenFields {
		ov, cv := f.get(old), f.get(cur)
		if ov != cv {
			vs = append(vs, Violation{
				Kind: KindFrozenFieldChanged, ID: id, File: file, Field: f.name,
				Message: fmt.Sprintf("%s: frozen field %q changed from %q to %q", id, f.name, ov, cv),
			})
		}
	}

	if old.Status != cur.Status || old.SupersededBy != cur.SupersededBy {
		if !legalStatusTransition(old, cur) {
			vs = append(vs, Violation{
				Kind: KindFrozenFieldChanged, ID: id, File: file, Field: "status",
				Message: fmt.Sprintf(
					"%s: illegal status transition %s -> %s (superseded-by %q -> %q); the only legal change is accepted -> superseded|deprecated, and never back",
					id, old.Status, cur.Status, old.SupersededBy, cur.SupersededBy,
				),
			})
		}
	}

	return vs
}

// legalStatusTransition is the field-scoped legality rule of spec §5.2/§5.3
// made mechanical: the sole permitted change is accepted -> superseded (with
// a non-empty superseded-by) or accepted -> deprecated (with superseded-by
// left empty, spec §5.2 "no replacement"). Every other transition —
// including the identity transitions superseded->superseded or
// deprecated->deprecated with a DIFFERENT superseded-by, and any
// resurrection back to accepted — is illegal. Called only when something
// about status/superseded-by actually differs.
func legalStatusTransition(old, cur *adr.ADR) bool {
	if old.Status != adr.StatusAccepted {
		return false // no further mutation is ever legal once an ADR left "accepted"
	}
	switch cur.Status {
	case adr.StatusSuperseded:
		return cur.SupersededBy != ""
	case adr.StatusDeprecated:
		return cur.SupersededBy == ""
	default:
		return false
	}
}
