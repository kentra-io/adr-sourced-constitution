package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/manifest"
)

// gitIn runs a git command against dir with a fixed, hermetic commit
// identity (this package's tests never rely on any ambient ~/.gitconfig).
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=guard-test", "GIT_AUTHOR_EMAIL=guard-test@example.com",
		"GIT_COMMITTER_NAME=guard-test", "GIT_COMMITTER_EMAIL=guard-test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// writeADR composes and writes one well-formed accepted ADR, returning its
// parsed model (Path included) for the caller to fold into a manifest.
func writeADR(t *testing.T, adrDir, id, title, decisionText string) adr.ADR {
	t.Helper()
	content := adr.Compose(adr.NewADR{
		ID: id, Title: title, Date: "2026-07-01",
		Body: "## Context and Problem Statement\n\nx\n\n## Considered Options\n\n- a\n\n## Decision Outcome\n\n" + decisionText + "\n",
	})
	path := filepath.Join(adrDir, adr.Filename(id, title))
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := adr.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	return *a
}

// newCleanRepo builds a scratch git repo with one committed, accepted ADR
// and a matching manifest — the baseline every integration test in this
// file starts from and mutates.
func newCleanRepo(t *testing.T) (root, adrDir string) {
	t.Helper()
	requireGit(t)

	root = t.TempDir()
	adrDir = filepath.Join(root, "constitution", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "init", "-q")

	a := writeADR(t, adrDir, "ADR-0001", "First rule", "y")
	if err := manifest.Write(adrDir, []adr.ADR{a}); err != nil {
		t.Fatal(err)
	}

	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "init")
	return root, adrDir
}

func TestRunCleanRepoIsClean(t *testing.T) {
	root, _ := newCleanRepo(t)

	res, err := Run(Options{Root: root})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !res.Summary.Clean || res.Summary.Violations != 0 {
		t.Errorf("Run() = %+v, want clean", res)
	}
	if res.Mode != "git" || res.Base != "HEAD" {
		t.Errorf("Run() Mode/Base = %q/%q, want git/HEAD", res.Mode, res.Base)
	}
}

func TestRunDetectsBodyEditInGitMode(t *testing.T) {
	root, adrDir := newCleanRepo(t)
	path := filepath.Join(adrDir, "ADR-0001-first-rule.md")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := []byte(strings.Replace(string(data), "\ny\n", "\nz\n", 1))
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Run(Options{Root: root})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Summary.Clean {
		t.Fatal("Run() = clean, want body_changed + manifest_mismatch violations")
	}
	kinds := map[Kind]bool{}
	for _, v := range res.Violations {
		kinds[v.Kind] = true
	}
	if !kinds[KindBodyChanged] || !kinds[KindManifestMismatch] {
		t.Errorf("Run().Violations = %+v, want both body_changed and manifest_mismatch", res.Violations)
	}
}

// TestRunHistoryRewriteScenario is the DoD's honest "out-of-band edit with
// git history rewritten" case: the edit gets folded into HEAD (e.g. via
// commit --amend, simulating a rewritten history an agent or attacker
// controls), so default git-mode (base=HEAD, diffed against a working tree
// that now equals HEAD) sees NO difference at all — only the manifest,
// which nothing re-derived after the edit, still disagrees.
func TestRunHistoryRewriteScenario(t *testing.T) {
	root, adrDir := newCleanRepo(t)
	path := filepath.Join(adrDir, "ADR-0001-first-rule.md")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := []byte(strings.Replace(string(data), "\ny\n", "\ntampered\n", 1))
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "tamper")
	gitIn(t, root, "commit", "--amend", "-q", "-m", "tamper (history rewritten)")

	res, err := Run(Options{Root: root})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Summary.Clean {
		t.Fatal("Run() = clean, want the manifest to still catch the tamper")
	}
	if len(res.Violations) != 1 || res.Violations[0].Kind != KindManifestMismatch {
		t.Errorf("Run().Violations = %+v, want exactly one manifest_mismatch (git mode sees nothing: base==HEAD==working tree)", res.Violations)
	}
}

func TestRunLegalSupersedeIsClean(t *testing.T) {
	root, adrDir := newCleanRepo(t)

	newADR := writeADR(t, adrDir, "ADR-0002", "Refined rule", "y2")
	old, err := adr.Parse(filepath.Join(adrDir, "ADR-0001-first-rule.md"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(old.Path)
	if err != nil {
		t.Fatal(err)
	}
	patched := []byte(strings.Replace(string(raw), "status: accepted\n", "status: superseded\nsuperseded-by: ADR-0002\n", 1))
	if err := os.WriteFile(old.Path, patched, 0o644); err != nil {
		t.Fatal(err)
	}
	supersededOld, err := adr.Parse(old.Path)
	if err != nil {
		t.Fatal(err)
	}

	if err := manifest.Write(adrDir, []adr.ADR{*supersededOld, newADR}); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "supersede")

	res, err := Run(Options{Root: root})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !res.Summary.Clean {
		t.Errorf("Run() = %+v, want clean (a legal supersede transition)", res)
	}
}

func TestRunNoGitManifestOnly(t *testing.T) {
	root, _ := newCleanRepo(t)

	res, err := Run(Options{Root: root, NoGit: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Mode != "manifest-only" {
		t.Errorf("Run().Mode = %q, want manifest-only", res.Mode)
	}
	if !res.Summary.Clean {
		t.Errorf("Run() = %+v, want clean", res)
	}
}

func TestRunMergeBaseMode(t *testing.T) {
	root, adrDir := newCleanRepo(t)
	gitIn(t, root, "branch", "target-branch", "HEAD")

	// A new ADR added on top of target-branch: fine, append-only.
	added := writeADR(t, adrDir, "ADR-0002", "Second rule", "y2")
	existing, err := adr.Parse(filepath.Join(adrDir, "ADR-0001-first-rule.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Write(adrDir, []adr.ADR{*existing, added}); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "add second")

	res, err := Run(Options{Root: root, MergeBase: "target-branch"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !res.Summary.Clean {
		t.Errorf("Run() = %+v, want clean (a pure addition against the merge-base)", res)
	}
	if res.Mode != "git" || res.Base == "HEAD" || res.Base == "" {
		t.Errorf("Run().Base = %q, want the resolved merge-base sha, not the literal ref/HEAD", res.Base)
	}
}

func TestRunExplicitGitModeOnNonRepoErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "constitution", "adr"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(Options{Root: root, Base: "HEAD"}); err == nil {
		t.Error("Run() with explicit --base on a non-repo = nil error, want an error (exit 2 territory)")
	}
}

func TestRunBareModeFallsBackWhenNotARepo(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "constitution", "adr"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := Run(Options{Root: root})
	if err != nil {
		t.Fatalf("Run() error = %v, want a silent fallback to manifest-only", err)
	}
	if res.Mode != "manifest-only" {
		t.Errorf("Run().Mode = %q, want manifest-only", res.Mode)
	}
}

// writeRuleADR composes and writes one accepted ADR carrying a single rule
// under the given category, for draft-mode vocabulary tests.
func writeRuleADR(t *testing.T, adrDir, id, title, category, slug string) {
	t.Helper()
	content := adr.Compose(adr.NewADR{
		ID: id, Title: title, Date: "2026-07-01",
		Body: "## Context and Problem Statement\n\nx\n\n## Considered Options\n\n- a\n\n## Decision Outcome\n\ny\n\n" +
			"## Rules\n\n### " + category + "\n\n#### " + slug + "\nRule text.\n",
	})
	path := filepath.Join(adrDir, adr.Filename(id, title))
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.Parse(path); err != nil {
		t.Fatal(err)
	}
}

// Draft phase reports clean on mutations that would scream in sealed mode:
// a reworded body and a stale manifest are legal working-set states before
// seal (v0.2 proposal §3).
func TestRunDraftIgnoresFrozenEditAndManifest(t *testing.T) {
	root, adrDir := newCleanRepo(t)

	// Sealed-illegal on both axes: body rewritten in place, manifest stale.
	writeADR(t, adrDir, "ADR-0001", "First rule", "REWRITTEN out-of-band")

	sealed, err := Run(Options{Root: root})
	if err != nil {
		t.Fatalf("sealed Run() error = %v", err)
	}
	if sealed.Summary.Clean {
		t.Fatal("sealed Run() = clean; want violations (the draft test needs a mutation sealed mode catches)")
	}

	draft, err := Run(Options{Root: root, Phase: "draft", Categories: []string{"architecture"}})
	if err != nil {
		t.Fatalf("draft Run() error = %v", err)
	}
	if !draft.Summary.Clean {
		t.Errorf("draft Run() = %+v, want clean (git legality + manifest checks are sealed-phase semantics)", draft.Violations)
	}
	if draft.Mode != "draft" {
		t.Errorf("Mode = %q, want %q", draft.Mode, "draft")
	}
}

func TestRunDraftUnknownCategory(t *testing.T) {
	root := t.TempDir()
	adrDir := filepath.Join(root, "constitution", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuleADR(t, adrDir, "ADR-0001", "In vocab", "architecture", "in-vocab")
	writeRuleADR(t, adrDir, "ADR-0002", "Out of vocab", "tooling", "pin-versions")

	res, err := Run(Options{Root: root, Phase: "draft", Categories: []string{"architecture", "testing"}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Summary.Violations != 1 {
		t.Fatalf("got %d violation(s), want exactly 1: %+v", res.Summary.Violations, res.Violations)
	}
	v := res.Violations[0]
	if v.Kind != KindUnknownCategory || v.ID != "ADR-0002" {
		t.Errorf("violation = %+v, want unknown_category on ADR-0002", v)
	}
	wantMsg := `rule tooling/pin-versions uses category "tooling", which is not in the configured vocabulary [architecture testing]`
	if v.Message != wantMsg {
		t.Errorf("Message = %q, want %q", v.Message, wantMsg)
	}
}

func TestRunDraftIDCollisionStillReported(t *testing.T) {
	root := t.TempDir()
	adrDir := filepath.Join(root, "constitution", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeADR(t, adrDir, "ADR-0001", "First", "y")
	writeADR(t, adrDir, "ADR-0001", "First again", "y")

	res, err := Run(Options{Root: root, Phase: "draft", Categories: []string{"architecture"}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Summary.Violations != 1 || res.Violations[0].Kind != KindIDCollision {
		t.Errorf("got %+v, want exactly one id_collision", res.Violations)
	}
}

// An explicit git base in draft is a hard error, not a silent no-op: the
// caller asked for a check that has no semantics before seal.
func TestRunDraftExplicitGitBaseErrors(t *testing.T) {
	root, _ := newCleanRepo(t)

	for _, opts := range []Options{
		{Root: root, Phase: "draft", Base: "HEAD"},
		{Root: root, Phase: "draft", MergeBase: "main"},
	} {
		_, err := Run(opts)
		if err == nil {
			t.Errorf("Run(%+v) error = nil, want draft-phase git-mode refusal", opts)
			continue
		}
		if !strings.Contains(err.Error(), "do not apply before") {
			t.Errorf("error = %q, want the draft-phase refusal message", err)
		}
	}
}
