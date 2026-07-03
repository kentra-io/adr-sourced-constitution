package guard

import (
	"fmt"
	"path/filepath"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/manifest"
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
		// The manifest file itself legitimately changes on every mutating
		// command (it records the new ADR's hash), and is verified separately
		// by the always-on manifest cross-check — so a change to it is
		// deliberately not a git-mode concern. This is the ONLY entry the git
		// path skips.
		if filepath.Base(e.path) == manifest.FileName {
			continue
		}
		// Fail closed. Every other entry under the constitution/adr/ pathspec
		// must be an ADR ".md" file guard can classify. Anything else here is
		// something guard has no rule for; skipping it silently is exactly the
		// quotepath bypass this rework closes — a rename to a non-ASCII name
		// once arrived C-quoted (ending `.md"`), read as non-".md", and was
		// dropped, letting a tampered frozen body pass clean. With -z parsing
		// the path is now the true path, so a non-".md" entry genuinely is
		// unclassifiable: refuse to run (exit 2) rather than report clean. For
		// a rename, BOTH endpoints must be ADR files.
		if filepath.Ext(e.path) != ".md" {
			return nil, fmt.Errorf(
				"guard: cannot classify %q (git status %c) under constitution/adr/; expected an ADR .md file or %s — refusing to report clean",
				e.path, e.status, manifest.FileName)
		}
		if (e.status == 'R' || e.status == 'C') && filepath.Ext(e.oldPath) != ".md" {
			return nil, fmt.Errorf(
				"guard: cannot classify rename/copy source %q (git status %c) under constitution/adr/; expected an ADR .md file — refusing to report clean",
				e.oldPath, e.status)
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
		// A rename is itself a violation (an accepted ADR's filename is frozen
		// along with its content), but a rename can ALSO carry a content edit
		// hidden behind git's rename detection. Compare the old blob (at the
		// old path) against the current file (at the new path) and append any
		// content/frozen-field/status violations, so a rename-and-edit reports
		// what actually changed, not just file_renamed.
		oldADR, curADR, err := parseBothSides(repoRoot, base, e.oldPath, e.path)
		if err != nil {
			return nil, err
		}
		vs := []Violation{{
			Kind: KindFileRenamed, ID: oldADR.ID, File: e.path, OldFile: e.oldPath,
			Message: fmt.Sprintf("%s: renamed from %s to %s; an accepted ADR's filename is frozen along with its content", oldADR.ID, e.oldPath, e.path),
		}}
		return append(vs, compareLegal(oldADR.ID, e.path, oldADR, curADR)...), nil

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
