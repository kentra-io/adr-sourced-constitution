package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// --- shared command-level test helpers (also used by init_test.go) ---

// runCLI drives the real CLI in-process against the test's working
// directory (t.Chdir'd to a temp repo).
func runCLI(t *testing.T, args ...string) error {
	t.Helper()
	return run(context.Background(), append([]string{"constitution"}, args...))
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const minimalBody = "## Context and Problem Statement\n\nx\n\n## Considered Options\n\n- a\n\n## Decision Outcome\n\ny\n"

// setupRepo builds a minimal constitution repo in a temp dir and chdirs
// into it: constitution.yml with the given consent policy + categories,
// and a body.md carrying the minimal MADR sections.
func setupRepo(t *testing.T, consentPolicy string, categories ...string) {
	t.Helper()
	t.Chdir(t.TempDir())
	var b strings.Builder
	b.WriteString("schemaVersion: 1\nconsent:\n  policy: " + consentPolicy + "\nsourceTracking:\n  type: none\ncategories:\n")
	for _, c := range categories {
		b.WriteString("  - " + c + "\n")
	}
	mustWriteFile(t, "constitution.yml", b.String())
	mustWriteFile(t, "body.md", minimalBody)
}

// --- composeRulesSection ---

func TestComposeRulesSection(t *testing.T) {
	base := []byte(minimalBody)

	t.Run("no flags returns body unchanged", func(t *testing.T) {
		out, err := composeRulesSection(base, nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != string(base) {
			t.Errorf("body changed with no flags:\n%s", out)
		}
	})

	t.Run("groups consecutive flags by category", func(t *testing.T) {
		out, err := composeRulesSection(base, []string{
			"testing/unit-first: Unit tests come first.",
			"testing/no-mocks: Prefer fakes over mocks.",
			"architecture/hexagonal: Ports and adapters.",
		})
		if err != nil {
			t.Fatal(err)
		}
		want := "## Rules\n\n### testing\n\n#### unit-first\nUnit tests come first.\n\n#### no-mocks\nPrefer fakes over mocks.\n\n### architecture\n\n#### hexagonal\nPorts and adapters.\n"
		if !strings.HasSuffix(string(out), want) {
			t.Errorf("composed section = ...%q, want suffix %q", tail(string(out), len(want)+40), want)
		}
		if err := adr.ValidateBody(out, "test"); err != nil {
			t.Errorf("composed body does not validate: %v", err)
		}
	})

	t.Run("regroups a non-consecutive category, keeping in-category flag order", func(t *testing.T) {
		out, err := composeRulesSection(base, []string{
			"a/x: one.",
			"b/y: two.",
			"a/z: three.",
		})
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if strings.Count(s, "### a") != 1 {
			t.Fatalf("category a re-opened:\n%s", s)
		}
		// a keeps first-appearance order (before b), and x before z within a.
		ia, ib := strings.Index(s, "### a"), strings.Index(s, "### b")
		ix, iz := strings.Index(s, "#### x"), strings.Index(s, "#### z")
		if ia >= ib || ia >= ix || ix >= iz || iz >= ib {
			t.Errorf("ordering wrong (a=%d b=%d x=%d z=%d):\n%s", ia, ib, ix, iz, s)
		}
		if err := adr.ValidateBody(out, "test"); err != nil {
			t.Errorf("regrouped body does not validate (grammar rejects re-opened categories): %v", err)
		}
	})

	t.Run("bad formats", func(t *testing.T) {
		for _, bad := range []string{
			"no colon here",
			"missing-slug: text",
			"a/b/c: too many parts",
			"a/b:   ",
			"",
		} {
			if _, err := composeRulesSection(base, []string{bad}); err == nil {
				t.Errorf("composeRulesSection(%q) = nil error, want the format rejection", bad)
			} else if !strings.Contains(err.Error(), `must be "<category>/<slug>: <text>"`) {
				t.Errorf("composeRulesSection(%q) error = %q", bad, err)
			}
		}
	})
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// --- adr new: flag interplay, vocabulary, preflight ---

// TestNewRejectsRuleFlagWithBodyRules proves --rule and a body-file that
// carries its own ## Rules section are mutually exclusive.
func TestNewRejectsRuleFlagWithBodyRules(t *testing.T) {
	setupRepo(t, "off", "testing")
	mustWriteFile(t, "body-rules.md", minimalBody+"\n## Rules\n\n### testing\n\n#### x\n\ntext\n")

	err := runCLI(t, "adr", "new", "--title", "T", "--body-file", "body-rules.md",
		"--rule", "testing/y: other text")
	if err == nil {
		t.Fatal("adr new(--rule + body Rules) = nil, want error")
	}
	if !strings.Contains(err.Error(), "provide the rules exactly once") {
		t.Errorf("error = %q", err)
	}
	if _, statErr := os.Stat(filepath.Join("constitution", "adr")); !os.IsNotExist(statErr) {
		t.Error("adr dir was created despite the refusal")
	}
}

// TestNewUnknownCategoryRefused proves a rule category outside the
// vocabulary refuses (with the --new-category hint) and writes nothing.
func TestNewUnknownCategoryRefused(t *testing.T) {
	setupRepo(t, "off", "architecture")

	err := runCLI(t, "adr", "new", "--title", "T", "--body-file", "body.md",
		"--rule", "testing/unit-first: Unit tests first.")
	if err == nil {
		t.Fatal("adr new(unknown category) = nil, want error")
	}
	if !strings.Contains(err.Error(), `rule category "testing" is not in the configured vocabulary`) ||
		!strings.Contains(err.Error(), "pass --new-category testing") {
		t.Errorf("error = %q", err)
	}
	if _, statErr := os.Stat(filepath.Join("constitution", "adr")); !os.IsNotExist(statErr) {
		t.Error("adr dir was created despite the refusal")
	}
}

// TestNewNewCategoryGrowsVocabulary proves --new-category admits the rule,
// appends the category to constitution.yml, and the rule projects.
func TestNewNewCategoryGrowsVocabulary(t *testing.T) {
	setupRepo(t, "off", "architecture")

	err := runCLI(t, "adr", "new", "--title", "T", "--body-file", "body.md",
		"--rule", "testing/unit-first: Unit tests first.", "--new-category", "testing")
	if err != nil {
		t.Fatalf("adr new(--new-category) = %v, want nil", err)
	}
	cfg := mustReadFile(t, "constitution.yml")
	if !strings.Contains(cfg, "- testing") {
		t.Errorf("constitution.yml missing the appended category:\n%s", cfg)
	}
	adrFile := mustReadFile(t, filepath.Join("constitution", "adr", "ADR-0001-t.md"))
	if !strings.Contains(adrFile, "#### unit-first") {
		t.Errorf("ADR missing the composed rule:\n%s", adrFile)
	}
	con := mustReadFile(t, filepath.Join("constitution", "constitution.md"))
	if !strings.Contains(con, "Unit tests first.") {
		t.Errorf("constitution.md missing the projected rule:\n%s", con)
	}
}

// TestNewUnusedNewCategoryRefused proves a --new-category no rule uses is
// an error (vocabulary stays tight; typos surface).
func TestNewUnusedNewCategoryRefused(t *testing.T) {
	setupRepo(t, "off", "architecture")

	err := runCLI(t, "adr", "new", "--title", "T", "--body-file", "body.md",
		"--rule", "architecture/x: text.", "--new-category", "security")
	if err == nil {
		t.Fatal("adr new(unused --new-category) = nil, want error")
	}
	if !strings.Contains(err.Error(), `--new-category "security" is not used by any rule`) {
		t.Errorf("error = %q", err)
	}
	if _, statErr := os.Stat(filepath.Join("constitution", "adr")); !os.IsNotExist(statErr) {
		t.Error("adr dir was created despite the refusal")
	}
}

// TestNewPreflightBeforeConsent proves the fold preflight rejects a
// dangling retirement ref BEFORE the consent gate: under the strict policy
// (which would itself refuse in this non-TTY test), the surfaced error is
// the preflight's, and nothing is written.
func TestNewPreflightBeforeConsent(t *testing.T) {
	setupRepo(t, "strict", "testing")

	err := runCLI(t, "adr", "new", "--title", "T", "--body-file", "body.md",
		"--supersedes-rule", "ADR-0001/testing/ghost")
	if err == nil {
		t.Fatal("adr new(dangling ref) = nil, want error")
	}
	if !strings.Contains(err.Error(), "does not resolve to any rule in the log") {
		t.Errorf("error = %q, want the dangling-ref rejection", err)
	}
	if strings.Contains(err.Error(), "consent") {
		t.Errorf("error = %q: consent gate ran before the preflight", err)
	}
	if _, statErr := os.Stat(filepath.Join("constitution", "adr")); !os.IsNotExist(statErr) {
		t.Error("adr dir was created despite the refusal")
	}
}

// TestNewConsentAfterPreflight is the control for the ordering proof: with
// a VALID input under strict consent (non-TTY, no --approve), the refusal
// is the consent gate's, and still nothing is written — including the
// vocabulary growth a --new-category authorized (deferred persistence:
// refuse ⇒ constitution.yml untouched).
func TestNewConsentAfterPreflight(t *testing.T) {
	setupRepo(t, "strict", "testing")
	cfgBefore := mustReadFile(t, "constitution.yml")

	err := runCLI(t, "adr", "new", "--title", "T", "--body-file", "body.md",
		"--rule", "security/x: text.", "--new-category", "security")
	if err == nil {
		t.Fatal("adr new(strict, no --approve) = nil, want the consent refusal")
	}
	if !strings.Contains(err.Error(), "consent") {
		t.Errorf("error = %q, want the consent refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join("constitution", "adr")); !os.IsNotExist(statErr) {
		t.Error("adr dir was created despite the refusal")
	}
	if cfgAfter := mustReadFile(t, "constitution.yml"); cfgAfter != cfgBefore {
		t.Errorf("constitution.yml changed despite the consent refusal:\n%s", cfgAfter)
	}
}

// TestNewMalformedRefFlag proves a malformed --supersedes-rule value fails
// naming the flag, before composition.
func TestNewMalformedRefFlag(t *testing.T) {
	setupRepo(t, "off", "testing")

	err := runCLI(t, "adr", "new", "--title", "T", "--body-file", "body.md",
		"--supersedes-rule", "not-a-ref")
	if err == nil {
		t.Fatal("adr new(malformed ref) = nil, want error")
	}
	if !strings.Contains(err.Error(), "--supersedes-rule") ||
		!strings.Contains(err.Error(), "ADR-NNNN/<category>/<slug>") {
		t.Errorf("error = %q", err)
	}
}

// TestSupersedeRetiresRule drives the symmetric supersede surface: the
// superseding ADR retires the old rule and files a replacement; the
// projection swaps them.
func TestSupersedeRetiresRule(t *testing.T) {
	setupRepo(t, "off", "testing")

	if err := runCLI(t, "adr", "new", "--title", "Old way", "--body-file", "body.md",
		"--rule", "testing/approach: The old approach."); err != nil {
		t.Fatalf("seeding ADR-0001: %v", err)
	}
	if err := runCLI(t, "supersede", "ADR-0001", "--title", "New way", "--body-file", "body.md",
		"--rule", "testing/approach: The new approach.",
		"--supersedes-rule", "ADR-0001/testing/approach"); err != nil {
		t.Fatalf("supersede: %v", err)
	}

	con := mustReadFile(t, filepath.Join("constitution", "constitution.md"))
	if strings.Contains(con, "The old approach.") {
		t.Errorf("constitution.md still projects the retired rule:\n%s", con)
	}
	if !strings.Contains(con, "The new approach.") {
		t.Errorf("constitution.md missing the replacement rule:\n%s", con)
	}
}

// TestSupersedeReRetireNoDoubleRetire pins the preflight's status-flip
// simulation: superseding a retirer while re-retiring the same ref must NOT
// false-error as double-retire — the flip removes the old ADR from the
// accepted-retirer set before the fold runs. Without the simulation (e.g. a
// preflight that just appends to the existing log), this scenario fails.
func TestSupersedeReRetireNoDoubleRetire(t *testing.T) {
	setupRepo(t, "off", "testing")

	if err := runCLI(t, "adr", "new", "--title", "First", "--body-file", "body.md",
		"--rule", "testing/x: The first rule."); err != nil {
		t.Fatalf("seeding ADR-0001: %v", err)
	}
	if err := runCLI(t, "adr", "new", "--title", "Second", "--body-file", "body.md",
		"--rule", "testing/x: The second rule.",
		"--supersedes-rule", "ADR-0001/testing/x"); err != nil {
		t.Fatalf("seeding retirer ADR-0002: %v", err)
	}
	if err := runCLI(t, "supersede", "ADR-0002", "--title", "Third", "--body-file", "body.md",
		"--rule", "testing/x: The third rule.",
		"--supersedes-rule", "ADR-0001/testing/x"); err != nil {
		t.Fatalf("supersede(re-retire) = %v, want nil — flip simulation missing?", err)
	}

	con := mustReadFile(t, filepath.Join("constitution", "constitution.md"))
	if strings.Contains(con, "The first rule.") {
		t.Errorf("constitution.md projects the re-retired rule:\n%s", con)
	}
	if strings.Contains(con, "The second rule.") {
		t.Errorf("constitution.md projects the superseded ADR's rule:\n%s", con)
	}
	if !strings.Contains(con, "The third rule.") {
		t.Errorf("constitution.md missing the superseding ADR's rule:\n%s", con)
	}
}

// TestSupersedePreflightDanglingRef proves supersede runs the same fold
// preflight: a dangling ref refuses with both files untouched.
func TestSupersedePreflightDanglingRef(t *testing.T) {
	setupRepo(t, "off", "testing")

	if err := runCLI(t, "adr", "new", "--title", "Old way", "--body-file", "body.md",
		"--rule", "testing/approach: The old approach."); err != nil {
		t.Fatalf("seeding ADR-0001: %v", err)
	}
	before := mustReadFile(t, filepath.Join("constitution", "adr", "ADR-0001-old-way.md"))

	err := runCLI(t, "supersede", "ADR-0001", "--title", "New way", "--body-file", "body.md",
		"--supersedes-rule", "ADR-0001/testing/ghost")
	if err == nil {
		t.Fatal("supersede(dangling ref) = nil, want error")
	}
	if !strings.Contains(err.Error(), "does not resolve to any rule in the log") {
		t.Errorf("error = %q", err)
	}
	after := mustReadFile(t, filepath.Join("constitution", "adr", "ADR-0001-old-way.md"))
	if before != after {
		t.Error("ADR-0001 was modified despite the preflight refusal")
	}
	if _, statErr := os.Stat(filepath.Join("constitution", "adr", "ADR-0002-new-way.md")); !os.IsNotExist(statErr) {
		t.Error("ADR-0002 was written despite the preflight refusal")
	}
}
