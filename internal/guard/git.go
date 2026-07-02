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

// diffNameStatus runs `git diff --name-status -M <base> -- <pathspec>` and
// parses the output. pathspec must already be repo-root-relative and
// forward-slash (the caller — checkGit — is responsible for that; see its
// doc comment on why that's always safe once requireRepoRootIsGitTop has
// passed).
func diffNameStatus(dir, base, pathspec string) ([]diffEntry, error) {
	out, err := runGit(dir, "diff", "--name-status", "-M", base, "--", pathspec)
	if err != nil {
		return nil, fmt.Errorf("guard: git diff against %s: %w", base, err)
	}
	return parseNameStatus(out), nil
}

func parseNameStatus(out string) []diffEntry {
	var entries []diffEntry
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] == "" {
			continue
		}
		code := fields[0][0]
		switch code {
		case 'R', 'C':
			if len(fields) < 3 {
				continue
			}
			entries = append(entries, diffEntry{status: code, oldPath: fields[1], path: fields[2]})
		default:
			entries = append(entries, diffEntry{status: code, path: fields[1]})
		}
	}
	return entries
}

// readWorkingTree reads the current on-disk content of a repo-root-relative,
// forward-slash git path, rooted at repoRoot.
func readWorkingTree(repoRoot, gitPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(gitPath)))
}
