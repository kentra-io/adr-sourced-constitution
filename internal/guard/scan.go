package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// scanDir parses every "*.md" file directly under dir, mirroring
// adr.ParseDir's file selection (non-recursive, sorted by filename) but
// WITHOUT ParseDir's fail-fast duplicate-id check: guard's whole job is to
// detect a duplicate id and report it as a typed id_collision violation
// rather than crash, so it needs the full set even when ids collide.
//
// A malformed file (one that fails adr.Parse) still fails scanDir outright
// — that is a "guard could not run" condition (exit 2), not a violation
// kind the plan §2.7 enum has room for. An absent adr/ directory is not an
// error: it parses to zero ADRs (a fresh or not-yet-adopted project is
// vacuously clean).
func scanDir(dir string) ([]adr.ADR, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
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

	adrs := make([]adr.ADR, 0, len(names))
	for _, name := range names {
		a, err := adr.Parse(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("guard: %w", err)
		}
		adrs = append(adrs, *a)
	}
	return adrs, nil
}

// rootRelFile renders an ADR's file identity as the fixed
// "constitution/adr/<filename>" form (spec §4.3: the ADR directory's
// location relative to the project root never varies), forward-slash,
// independent of how dir/adrs.Path were constructed on this OS — matching
// the form git-mode violations use so all Violation.File values in one
// Result are uniformly repo-root-relative.
func rootRelFile(name string) string {
	return "constitution/adr/" + name
}
