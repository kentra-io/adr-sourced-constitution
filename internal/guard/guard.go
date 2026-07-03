// Package guard implements `constitution guard` (spec §5.3/§5.4,
// implementation-plan.md §2.7): the immutability check that catches any
// out-of-band mutation of an accepted ADR.
//
// Three independent checks run, in this order, and their violations are
// pooled into one Result:
//
//  1. id-uniqueness — always on, regardless of mode: two files sharing an
//     id is a violation (kind id_collision) whether or not either changed.
//  2. git mode (default, unless --no-git or the directory isn't a usable
//     git repository) — a structured comparison of the base ref's version
//     of each changed file against its current version: only status and
//     superseded-by may differ, and only in the direction spec §5.2
//     permits. See legality.go.
//  3. manifest cross-check — always on, regardless of mode: the recorded
//     hash in constitution/adr/.manifest.sha256 for each ADR must match
//     that ADR's current frozen-content hash (internal/manifest.Hash).
//     This is the only check that survives a rewritten git history, since
//     it compares disk against a file, not commit against commit. See
//     manifest.go. It is advisory (plan §2.7): a committer who edits both
//     the ADR and the manifest in the same commit defeats it — no
//     tamper-evidence claim is made against a malicious actor; see
//     docs/manifest-canonicalization.md.
//
// # Git-mode base resolution
//
// Bare `guard` (no --base/--merge-base/--no-git): if the constitution
// project root is a usable git repository, git mode runs with base=HEAD
// (catches an agent's uncommitted in-progress edits, spec §5.4 Phase 1
// "surfaces; human honors"); if it is NOT a git repository (or the git
// binary is unavailable), guard degrades to manifest-only — this is the
// "no-git fallback" plan §2.7 describes, and the text/JSON output labels the
// mode "manifest-only" so the degrade is visible in the result itself. If
// the caller explicitly asked for git mode (--base or
// --merge-base given) and no usable repository exists, that is instead a
// hard error (exit 2): the caller asked for a check this environment
// cannot perform. --no-git always skips git mode outright, no detection
// needed.
//
// # Scope limit
//
// Git mode requires the constitution project root (the directory holding
// constitution.yml) to equal the git repository's top level. A nested
// layout (constitution/ inside a subdirectory of a larger git repo) is
// out of scope for v1 and produces a clear error rather than silently
// computing paths relative to the wrong root.
//
// # What guard does NOT check
//
// Referential integrity of the supersedes/superseded-by links (a
// superseded-by pointing at an ADR id that does not exist, a supersedes
// backlink with no matching forward link) is deliberately NOT guard's
// concern in v1: it is the write path's invariant, established and held by
// the mutating commands (`supersede`/`deprecate`) at the moment they write
// the link, not re-verified here. guard's remit is out-of-band *mutation* of
// already-accepted content, not link well-formedness.
package guard

import (
	"fmt"
	"path/filepath"
	"sort"
)

// Kind enumerates the exact violation types plan §2.7 pins. Every
// violation cites one of these; there is no catch-all.
type Kind string

// The exact violation kinds plan §2.7 pins; there is no catch-all.
const (
	KindFrozenFieldChanged Kind = "frozen_field_changed"
	KindBodyChanged        Kind = "body_changed"
	KindFileDeleted        Kind = "file_deleted"
	KindFileRenamed        Kind = "file_renamed"
	KindManifestMismatch   Kind = "manifest_mismatch"
	KindIDCollision        Kind = "id_collision"
)

// allKinds is the frozen enum plan §2.7 pins, as data: every Kind guard may
// ever emit, in schema order. It is the single source the JSON-schema
// fixture's `kind` enum and the all-kinds golden payload are held in lockstep
// with (see json_test.go's TestKindEnumLockstep) — adding a 7th Kind without
// updating the schema and the golden sample fails that test, so the machine
// contract cannot drift out from under a consumer.
var allKinds = []Kind{
	KindFrozenFieldChanged,
	KindBodyChanged,
	KindFileDeleted,
	KindFileRenamed,
	KindManifestMismatch,
	KindIDCollision,
}

// Violation is one detected illegal mutation, citing the ADR id and the
// file(s) involved (plan §2.7: "each citing the ADR id"). Fields unused by
// a given Kind are omitted from JSON output (Field/OldFile/Files).
type Violation struct {
	Kind    Kind     `json:"kind"`
	ID      string   `json:"id"`
	File    string   `json:"file"`
	OldFile string   `json:"oldFile,omitempty"` // file_renamed: the base-ref path
	Files   []string `json:"files,omitempty"`   // id_collision: every file sharing the id
	Field   string   `json:"field,omitempty"`   // frozen_field_changed: the frontmatter field name
	Message string   `json:"message"`
}

// Summary is the machine-readable roll-up (plan §2.7: "violations array +
// summary").
type Summary struct {
	Checked    int  `json:"checked"`    // ADRs scanned on disk
	Violations int  `json:"violations"` // len(Violations)
	Clean      bool `json:"clean"`
}

// Result is exactly what `--format json` marshals (plan §2.7: "JSON-only
// on stdout, pipeable"). Mode/Base are excluded from JSON (json:"-") and
// exist only for the human `--format text` renderer.
type Result struct {
	Violations []Violation `json:"violations"`
	Summary    Summary     `json:"summary"`

	Mode string `json:"-"` // "git" or "manifest-only", for text output
	Base string `json:"-"` // resolved git base ref; "" in manifest-only mode
}

// Options configures one guard run (cmd/constitution/guard.go builds this
// from flags).
type Options struct {
	// Root is the constitution project root: the directory containing
	// constitution.yml and constitution/. Required.
	Root string
	// Base is an explicit git ref to diff against (--base). Empty means
	// HEAD, unless MergeBase is set.
	Base string
	// MergeBase, when non-empty, resolves the diff base as
	// `git merge-base <MergeBase> HEAD` (--merge-base, the CI mode) rather
	// than trusting a caller-supplied SHA (plan §2.7).
	MergeBase string
	// NoGit skips git mode outright: manifest + id-uniqueness only.
	NoGit bool
}

// Run performs one guard check and returns every violation found. A
// non-nil error means guard could not run at all (exit 2 per the plan §2.7
// exit contract: not a usable git repo when git mode was explicitly
// requested, a missing manifest, a malformed ADR file, or a git command
// that failed) — Result is always zero-valued in that case. A nil error
// with a non-empty Result.Violations means the check ran successfully and
// found problems (exit 1); a nil error with zero violations is clean
// (exit 0). Callers map these to the exit contract; Run itself never
// exits or logs.
func Run(opts Options) (Result, error) {
	adrDir := filepath.Join(opts.Root, "constitution", "adr")

	adrs, err := scanDir(adrDir)
	if err != nil {
		return Result{}, err
	}

	var violations []Violation
	violations = append(violations, idCollisions(adrs)...)

	mode, err := resolveGitMode(opts)
	if err != nil {
		return Result{}, err
	}
	if mode.active {
		gv, err := checkGit(opts.Root, mode.base)
		if err != nil {
			return Result{}, err
		}
		violations = append(violations, gv...)
	}

	mv, err := checkManifest(adrDir, adrs)
	if err != nil {
		return Result{}, err
	}
	violations = append(violations, mv...)

	sortViolations(violations)
	if violations == nil {
		// Keep the JSON payload's "violations" key an array, never null —
		// a machine consumer (plan §2.7: "pipeable") shouldn't need a nil
		// check to iterate a clean result.
		violations = []Violation{}
	}

	res := Result{
		Violations: violations,
		Summary: Summary{
			Checked:    len(adrs),
			Violations: len(violations),
			Clean:      len(violations) == 0,
		},
		Mode: "manifest-only",
	}
	if mode.active {
		res.Mode = "git"
		res.Base = mode.base
	}
	return res, nil
}

func sortViolations(vs []Violation) {
	sort.Slice(vs, func(i, j int) bool {
		a, b := vs[i], vs[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Field < b.Field
	})
}

// gitModeDecision is resolveGitMode's result: whether git-mode checks run
// at all, and if so, against which resolved ref.
type gitModeDecision struct {
	active bool
	base   string
}

// resolveGitMode implements the base-resolution + fallback/error policy
// documented in the package doc's "Git-mode base resolution" section.
func resolveGitMode(opts Options) (gitModeDecision, error) {
	if opts.NoGit {
		return gitModeDecision{}, nil
	}

	explicit := opts.Base != "" || opts.MergeBase != ""
	if !gitAvailable(opts.Root) {
		if explicit {
			return gitModeDecision{}, fmt.Errorf(
				"guard: git mode was requested (--base/--merge-base) but %s is not a usable git repository (or the git binary is unavailable); pass --no-git for manifest-only checking",
				opts.Root,
			)
		}
		return gitModeDecision{}, nil
	}

	if err := requireRepoRootIsGitTop(opts.Root); err != nil {
		if explicit {
			return gitModeDecision{}, err
		}
		return gitModeDecision{}, nil
	}

	base := "HEAD"
	switch {
	case opts.MergeBase != "":
		sha, err := mergeBase(opts.Root, opts.MergeBase)
		if err != nil {
			return gitModeDecision{}, fmt.Errorf("guard: computing merge-base against %q: %w", opts.MergeBase, err)
		}
		base = sha
	case opts.Base != "":
		base = opts.Base
	}
	return gitModeDecision{active: true, base: base}, nil
}
