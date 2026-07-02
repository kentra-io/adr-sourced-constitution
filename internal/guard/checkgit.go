package guard

import (
	"fmt"
	"path/filepath"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// checkGit runs the git-mode structured comparison (spec §5.3, plan §2.7):
// diff base..working-tree over the ADR directory, then for each changed
// path apply the legality rule appropriate to how it changed.
//
// The pathspec is built as a repo-root-relative, forward-slash string
// purely by string concatenation ("constitution/adr"), never through
// filepath.Join/OS separators — this is safe (and, on Windows, required)
// because Run's caller (resolveGitMode via requireRepoRootIsGitTop) has
// already established that repoRoot IS the git top level, so
// "constitution/adr" relative to repoRoot is also relative to the git
// root, which is exactly the form git diff/git show expect and the exact
// form git itself reports paths in.
func checkGit(repoRoot, base string) ([]Violation, error) {
	pathspec := "constitution/adr"

	entries, err := diffNameStatus(repoRoot, base, pathspec)
	if err != nil {
		return nil, err
	}

	var violations []Violation
	for _, e := range entries {
		if filepath.Ext(e.path) != ".md" || filepath.Base(e.path) == "" {
			continue
		}
		v, err := checkGitEntry(repoRoot, base, e)
		if err != nil {
			return nil, err
		}
		violations = append(violations, v...)
	}
	return violations, nil
}

func checkGitEntry(repoRoot, base string, e diffEntry) ([]Violation, error) {
	switch e.status {
	case 'A':
		// New files are the point of an append-only log (spec §5.1) — no
		// per-file check here. A new file's id colliding with an existing
		// one is caught by the always-on idCollisions scan, which sees the
		// full current id set, not just this diff.
		return nil, nil

	case 'D':
		oldBlob, err := showFile(repoRoot, base, e.path)
		if err != nil {
			return nil, err
		}
		oldADR, err := adr.ParseBytes(oldBlob, e.path)
		if err != nil {
			return nil, fmt.Errorf("guard: %s:%s: %w", base, e.path, err)
		}
		return []Violation{{
			Kind: KindFileDeleted, ID: oldADR.ID, File: e.path,
			Message: fmt.Sprintf("%s: deleted (present at %s, absent from the working tree); the ADR log is append-only", oldADR.ID, base),
		}}, nil

	case 'M':
		oldADR, curADR, err := parseBothSides(repoRoot, base, e.path, e.path)
		if err != nil {
			return nil, err
		}
		return compareLegal(oldADR.ID, e.path, oldADR, curADR), nil

	case 'R':
		oldBlob, err := showFile(repoRoot, base, e.oldPath)
		if err != nil {
			return nil, err
		}
		oldADR, err := adr.ParseBytes(oldBlob, e.oldPath)
		if err != nil {
			return nil, fmt.Errorf("guard: %s:%s: %w", base, e.oldPath, err)
		}
		return []Violation{{
			Kind: KindFileRenamed, ID: oldADR.ID, File: e.path, OldFile: e.oldPath,
			Message: fmt.Sprintf("%s: renamed from %s to %s; an accepted ADR's filename is frozen along with its content", oldADR.ID, e.oldPath, e.path),
		}}, nil

	default:
		// Copies (C) and type changes (T) are not expected for a Markdown
		// log and have no dedicated Kind in the plan §2.7 enum; treat any
		// other status git reports as a frozen-content violation rather
		// than silently ignoring it.
		oldADR, curADR, err := parseBothSides(repoRoot, base, e.path, e.path)
		if err != nil {
			return nil, err
		}
		return compareLegal(oldADR.ID, e.path, oldADR, curADR), nil
	}
}

// parseBothSides parses the base-ref blob at oldPath and the current
// working-tree file at curPath (both repo-root-relative, forward-slash),
// as they existed at the two ends of a comparison.
func parseBothSides(repoRoot, base, oldPath, curPath string) (old, cur *adr.ADR, err error) {
	oldBlob, err := showFile(repoRoot, base, oldPath)
	if err != nil {
		return nil, nil, err
	}
	old, err = adr.ParseBytes(oldBlob, oldPath)
	if err != nil {
		return nil, nil, fmt.Errorf("guard: %s:%s: %w", base, oldPath, err)
	}

	curBytes, err := readWorkingTree(repoRoot, curPath)
	if err != nil {
		return nil, nil, fmt.Errorf("guard: reading %s: %w", curPath, err)
	}
	cur, err = adr.ParseBytes(curBytes, filepath.Join(repoRoot, filepath.FromSlash(curPath)))
	if err != nil {
		return nil, nil, fmt.Errorf("guard: %w", err)
	}
	return old, cur, nil
}
