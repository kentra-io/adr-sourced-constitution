package guard

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// idCollisions detects two-or-more files sharing an id (plan §2.6/§2.7):
// "a NEW file whose id collides with an existing one" is exactly the case
// this catches, and it works whether the colliding pair is one new file vs
// one old file, or two files both new in this change — it looks at the
// full current id set, not a diff. Reported as one id_collision violation
// per colliding id, citing every file that uses it (plan: "report ... with
// both files, NOT as a crash").
func idCollisions(adrs []adr.ADR) []Violation {
	byID := make(map[string][]string, len(adrs))
	for i := range adrs {
		byID[adrs[i].ID] = append(byID[adrs[i].ID], rootRelFile(filepath.Base(adrs[i].Path)))
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var violations []Violation
	for _, id := range ids {
		files := byID[id]
		if len(files) < 2 {
			continue
		}
		sort.Strings(files)
		violations = append(violations, Violation{
			Kind: KindIDCollision, ID: id, File: files[0], Files: files,
			Message: fmt.Sprintf("%s: id used by %d files: %s", id, len(files), strings.Join(files, ", ")),
		})
	}
	return violations
}
