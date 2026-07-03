package guard

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runGit executes the system git binary with args, cwd=dir, and returns
// trimmed stdout. Plan §2.7 is explicit: shell out to the system git
// binary, not go-git, not hand-parsed unified diffs.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

// gitAvailable reports whether dir is inside a usable git working tree
// with a working git binary on PATH. Both failure modes (binary absent,
// not a repo) collapse to the same boolean by design — see the package
// doc's base-resolution policy, which treats them identically.
func gitAvailable(dir string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	out, err := runGit(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// gitTopLevel returns the absolute path to the git repository root
// containing dir.
func gitTopLevel(dir string) (string, error) {
	out, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("guard: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// requireRepoRootIsGitTop enforces the v1 scope limit documented in the
// package doc: the constitution project root must equal the git
// repository's top level, so every path git reports (always root-relative,
// forward-slash) is directly usable as a guard Violation.File / manifest
// key without any rebasing. Symlinks are resolved before comparing so a
// scratch dir under a symlinked tmp (e.g. macOS /tmp -> /private/tmp)
// compares correctly.
func requireRepoRootIsGitTop(root string) error {
	top, err := gitTopLevel(root)
	if err != nil {
		return err
	}
	rootReal := resolvePath(root)
	topReal := resolvePath(top)
	if rootReal != topReal {
		return fmt.Errorf(
			"guard: git mode requires the constitution project root (%s) to be the git repository root (found %s); nested layouts are out of scope for v1 — pass --no-git",
			rootReal, topReal,
		)
	}
	return nil
}

// resolvePath returns an absolute, symlink-resolved form of p for robust
// path comparison, falling back progressively (symlink resolution, then
// plain Abs, then p itself) so a transient stat failure never turns into a
// spurious mismatch error.
func resolvePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// mergeBase computes `git merge-base <target> HEAD` (plan §2.7: computed
// locally, never trusted from pull_request.base.sha).
func mergeBase(dir, target string) (string, error) {
	out, err := runGit(dir, "merge-base", target, "HEAD")
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(out)
	if sha == "" {
		return "", fmt.Errorf("git merge-base %s HEAD returned no output", target)
	}
	return sha, nil
}

// showFile returns the content of the file at gitPath (repo-root-relative,
// forward-slash) as it existed at ref, via `git show <ref>:<path>`. Callers
// only pass paths git itself just reported as present at ref (from a diff
// entry with status D/M/R), so a missing-at-ref failure here indicates a
// genuine problem, not an expected case to special-case.
func showFile(dir, ref, gitPath string) ([]byte, error) {
	cmd := exec.Command("git", "show", ref+":"+gitPath)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("guard: git show %s:%s: %s", ref, gitPath, msg)
	}
	return stdout.Bytes(), nil
}

// diffEntry is one line of `git diff --name-status` output.
type diffEntry struct {
	status  byte   // 'A', 'M', 'D', 'R', 'C', ... (first byte of the status code)
	path    string // new/current path (repo-root-relative, forward-slash)
	oldPath string // set only for R/C: the base-ref path
}

// diffNameStatus runs `git diff --name-status -z -M <base> -- <pathspec>` and
// parses the NUL-delimited output. pathspec must already be
// repo-root-relative and forward-slash (the caller — checkGit — is
// responsible for that; see its doc comment on why that's always safe once
// requireRepoRootIsGitTop has passed).
//
// -z is load-bearing, not a nicety: WITHOUT it git C-quotes any path with a
// non-ASCII or control byte (e.g. a rename to ADR-0001-première-règle.md
// arrives as the literal string "constitution/adr/ADR-0001-premi\303\250re-…\.md"
// — surrounding double-quotes included) and separates the old/new rename
// paths with a tab. That defeats a byte-level ".md" extension check (the
// quoted path ends in `.md"`, not `.md`) and makes tab-in-filename ambiguous.
// With -z, paths are emitted verbatim as raw bytes, NUL-terminated, never
// quoted, so the parser and the downstream extension filter see the true
// path (spec §5.3 must not be evadable by a filename choice).
func diffNameStatus(dir, base, pathspec string) ([]diffEntry, error) {
	out, err := runGit(dir, "diff", "--name-status", "-z", "-M", base, "--", pathspec)
	if err != nil {
		return nil, fmt.Errorf("guard: git diff against %s: %w", base, err)
	}
	return parseNameStatus(out)
}

// parseNameStatus parses `git diff --name-status -z` output: a flat stream of
// NUL-terminated tokens. Each record is a status token (a single letter
// optionally followed by a similarity score, e.g. "M", "D", "A", "R089")
// followed by one path token, except rename/copy (R/C) records, which carry
// two path tokens: old then new. A truncated record (a status token with its
// path token(s) missing) is a parse failure, not something to skip silently —
// guard fails closed (exit 2) rather than risk overlooking a mutation.
func parseNameStatus(out string) ([]diffEntry, error) {
	tokens := strings.Split(out, "\x00")
	var entries []diffEntry
	for i := 0; i < len(tokens); {
		status := tokens[i]
		if status == "" {
			// The stream is NUL-terminated, so Split yields a trailing "" (and
			// nothing else empty in well-formed output); skip it.
			i++
			continue
		}
		code := status[0]
		switch code {
		case 'R', 'C':
			// git never emits an empty path token, so a "" where a path is
			// expected (including the stream's trailing "" reached early)
			// means the record was truncated.
			if i+2 >= len(tokens) || tokens[i+1] == "" || tokens[i+2] == "" {
				return nil, fmt.Errorf("guard: malformed git diff -z output: rename/copy record %q missing its path token(s)", status)
			}
			entries = append(entries, diffEntry{status: code, oldPath: tokens[i+1], path: tokens[i+2]})
			i += 3
		default:
			if i+1 >= len(tokens) || tokens[i+1] == "" {
				return nil, fmt.Errorf("guard: malformed git diff -z output: record %q missing its path token", status)
			}
			entries = append(entries, diffEntry{status: code, path: tokens[i+1]})
			i += 2
		}
	}
	return entries, nil
}

// readWorkingTree reads the current on-disk content of a repo-root-relative,
// forward-slash git path, rooted at repoRoot.
func readWorkingTree(repoRoot, gitPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(gitPath)))
}
