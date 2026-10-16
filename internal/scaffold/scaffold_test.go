package scaffold

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

// newTestRoot returns a scratch repo root with constitution/ present (so
// .state can be written) and a config selecting one skills tree.
func newTestRoot(t *testing.T) (root string, cfg *config.Config) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "constitution"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg = &config.Config{
		SchemaVersion: config.SchemaVersion,
		Skills:        config.Skills{Trees: []string{config.SkillTreeClaude}},
		Categories:    []string{"architecture"},
	}
	return root, cfg
}

func initOpts(root string, cfg *config.Config) Options {
	return Options{Root: root, Cfg: cfg, Mode: ModeInit, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
}

func warnOpts(root string, cfg *config.Config) (Options, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return Options{Root: root, Cfg: cfg, Mode: ModeWarn, Stdout: &stdout, Stderr: &stderr}, &stdout, &stderr
}

func claudePath(root string) string { return filepath.Join(root, "CLAUDE.md") }
func agentsPath(root string) string { return filepath.Join(root, "AGENTS.md") }
func skillPath(root string) string {
	return filepath.Join(root, ".claude", "skills", "adr-draft", "SKILL.md")
}
func statePathT(root string) string { return filepath.Join(root, "constitution", ".state") }

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// legacyFile returns content as an older CLI left it: the block appended
// at EOF by ApplyBlock.
func legacyFile(t *testing.T, content, interior string) string {
	t.Helper()
	out, err := ApplyBlock([]byte(content), interior)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestRefresh_InitWritesAndIsIdempotent(t *testing.T) {
	root, cfg := newTestRoot(t)

	if err := Refresh(initOpts(root, cfg)); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	if _, err := os.Stat(skillPath(root)); err != nil {
		t.Fatalf("skill not fanned out: %v", err)
	}
	if _, err := os.Stat(statePathT(root)); err != nil {
		t.Fatalf(".state not written: %v", err)
	}
	for _, p := range []string{claudePath(root), agentsPath(root)} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s must not be written (err = %v)", p, err)
		}
	}

	// Snapshot, refresh again, assert byte-identical (idempotent no-op).
	before := readAll(t, root)
	if err := Refresh(initOpts(root, cfg)); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	after := readAll(t, root)
	for name, b := range before {
		if !bytes.Equal(after[name], b) {
			t.Fatalf("%s changed on idempotent re-run", name)
		}
	}
}

func TestRefresh_NoManagedTargetsSkipsState(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "constitution"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{SchemaVersion: config.SchemaVersion, Categories: []string{"architecture"}}

	if err := Refresh(Options{Root: root, Cfg: cfg, Mode: ModeInit}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, err := os.Stat(statePathT(root)); !os.IsNotExist(err) {
		t.Fatal("a repo that manages nothing must not get a .state file")
	}
}

// driftSkill rewrites the fanned-out skill to simulate a user hand-edit.
func driftSkill(t *testing.T, root string) {
	t.Helper()
	writeFile(t, skillPath(root), "HACKED\n")
}

func TestRefresh_WarnLeavesDrift(t *testing.T) {
	root, cfg := newTestRoot(t)
	if err := Refresh(initOpts(root, cfg)); err != nil {
		t.Fatal(err)
	}
	driftSkill(t, root)

	o, _, stderr := warnOpts(root, cfg)
	if err := Refresh(o); err != nil {
		t.Fatalf("warn mode must never fail on drift: %v", err)
	}
	if got := readFile(t, skillPath(root)); got != "HACKED\n" {
		t.Fatal("warn mode must leave the drifted file untouched")
	}
	if !strings.Contains(stderr.String(), "drifted") {
		t.Fatalf("expected a drift warning, got %q", stderr.String())
	}
}

func TestRefresh_InitRefusesDriftWithoutForce(t *testing.T) {
	root, cfg := newTestRoot(t)
	if err := Refresh(initOpts(root, cfg)); err != nil {
		t.Fatal(err)
	}
	driftSkill(t, root)

	if err := Refresh(initOpts(root, cfg)); err == nil {
		t.Fatal("init mode must refuse a drifted target without --force")
	}
	if got := readFile(t, skillPath(root)); got != "HACKED\n" {
		t.Fatal("a refused init must not have rewritten the file")
	}
}

func TestRefresh_InitForceOverwritesDrift(t *testing.T) {
	root, cfg := newTestRoot(t)
	if err := Refresh(initOpts(root, cfg)); err != nil {
		t.Fatal(err)
	}
	driftSkill(t, root)

	o := initOpts(root, cfg)
	o.Force = true
	if err := Refresh(o); err != nil {
		t.Fatalf("force refresh: %v", err)
	}
	if got := readFile(t, skillPath(root)); got == "HACKED\n" {
		t.Fatal("--force must overwrite the drifted file")
	}
}

func TestRefresh_InitConfirmOverwritesDrift(t *testing.T) {
	root, cfg := newTestRoot(t)
	if err := Refresh(initOpts(root, cfg)); err != nil {
		t.Fatal(err)
	}
	driftSkill(t, root)

	o := initOpts(root, cfg)
	o.Confirm = func(string) (bool, error) { return true, nil } // user says yes
	if err := Refresh(o); err != nil {
		t.Fatalf("confirmed refresh: %v", err)
	}
	if got := readFile(t, skillPath(root)); got == "HACKED\n" {
		t.Fatal("an accepted confirm must overwrite the drifted file")
	}
}

// --- legacy pointer-block removal (0.4.0) ---

func TestRefresh_RemovesLegacyBlocks(t *testing.T) {
	for _, mode := range []Mode{ModeInit, ModeWarn} {
		root, cfg := newTestRoot(t)
		writeFile(t, claudePath(root), legacyFile(t, "# Project\n\nKeep me.\n", legacyClaudeInterior))
		writeFile(t, agentsPath(root), legacyFile(t, "", legacyAgentsInterior))

		var stdout bytes.Buffer
		o := Options{Root: root, Cfg: cfg, Mode: mode, Stdout: &stdout, Stderr: &bytes.Buffer{}}
		if err := Refresh(o); err != nil {
			t.Fatalf("mode %d: refresh: %v", mode, err)
		}
		if got := readFile(t, claudePath(root)); got != "# Project\n\nKeep me.\n" {
			t.Fatalf("mode %d: CLAUDE.md = %q, want the pre-append content", mode, got)
		}
		if _, err := os.Stat(agentsPath(root)); !os.IsNotExist(err) {
			t.Fatalf("mode %d: AGENTS.md held only the block and must be deleted (err = %v)", mode, err)
		}
		out := stdout.String()
		if !strings.Contains(out, "removed the always-loaded constitution block from CLAUDE.md") ||
			!strings.Contains(out, "removed AGENTS.md") {
			t.Fatalf("mode %d: stdout = %q", mode, out)
		}
	}
}

func TestRefresh_PrunesLegacyStateEntries(t *testing.T) {
	root, cfg := newTestRoot(t)
	st := newState()
	st.set("CLAUDE.md", "deadbeef") // no CLAUDE.md on disk at all
	st.set("AGENTS.md", "deadbeef")
	if err := st.Save(root); err != nil {
		t.Fatal(err)
	}

	o, _, _ := warnOpts(root, cfg)
	if err := Refresh(o); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"CLAUDE.md", "AGENTS.md"} {
		if _, ok := got.get(rel); ok {
			t.Errorf(".state still records %s", rel)
		}
	}
}

func TestRefresh_RemovesBlockMatchingStoredHash(t *testing.T) {
	root, cfg := newTestRoot(t)
	const other = "@some/other-interior.md"
	writeFile(t, claudePath(root), legacyFile(t, "# P\n", other))
	st := newState()
	st.set("CLAUDE.md", hashContent([]byte(other)))
	if err := st.Save(root); err != nil {
		t.Fatal(err)
	}

	o, _, _ := warnOpts(root, cfg)
	if err := Refresh(o); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, claudePath(root)); got != "# P\n" {
		t.Fatalf("CLAUDE.md = %q, want the block removed", got)
	}
}

func TestRefresh_DriftedLegacyBlock(t *testing.T) {
	drifted := func(t *testing.T) (string, *config.Config, string) {
		root, cfg := newTestRoot(t)
		content := legacyFile(t, "# P\n", "@constitution/HACKED.md")
		writeFile(t, claudePath(root), content)
		return root, cfg, content
	}

	t.Run("warn mode leaves it", func(t *testing.T) {
		root, cfg, content := drifted(t)
		o, _, stderr := warnOpts(root, cfg)
		if err := Refresh(o); err != nil {
			t.Fatalf("drift must never fail: %v", err)
		}
		if readFile(t, claudePath(root)) != content {
			t.Fatal("drifted block must be left untouched")
		}
		if !strings.Contains(stderr.String(), "constitution init --force") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})
	t.Run("init without force warns, no error", func(t *testing.T) {
		root, cfg, content := drifted(t)
		var stderr bytes.Buffer
		o := initOpts(root, cfg)
		o.Stderr = &stderr
		if err := Refresh(o); err != nil {
			t.Fatalf("init must not fail on a drifted legacy block: %v", err)
		}
		if readFile(t, claudePath(root)) != content {
			t.Fatal("drifted block must be left untouched")
		}
		if !strings.Contains(stderr.String(), "edited by hand") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})
	t.Run("init confirm no leaves it", func(t *testing.T) {
		root, cfg, content := drifted(t)
		o := initOpts(root, cfg)
		o.Confirm = func(string) (bool, error) { return false, nil }
		if err := Refresh(o); err != nil {
			t.Fatal(err)
		}
		if readFile(t, claudePath(root)) != content {
			t.Fatal("declined confirm must leave the block")
		}
	})
	t.Run("init confirm yes removes it", func(t *testing.T) {
		root, cfg, _ := drifted(t)
		o := initOpts(root, cfg)
		o.Confirm = func(string) (bool, error) { return true, nil }
		if err := Refresh(o); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, claudePath(root)); got != "# P\n" {
			t.Fatalf("CLAUDE.md = %q", got)
		}
	})
	t.Run("init force removes it", func(t *testing.T) {
		root, cfg, _ := drifted(t)
		o := initOpts(root, cfg)
		o.Force = true
		if err := Refresh(o); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, claudePath(root)); got != "# P\n" {
			t.Fatalf("CLAUDE.md = %q", got)
		}
	})
}

func TestRefresh_MalformedLegacyMarkersLeftEvenUnderForce(t *testing.T) {
	root, cfg := newTestRoot(t)
	content := "# x\n" + BlockBegin + "\nno end\n"
	writeFile(t, claudePath(root), content)

	var stderr bytes.Buffer
	o := initOpts(root, cfg)
	o.Force = true
	o.Stderr = &stderr
	if err := Refresh(o); err != nil {
		t.Fatalf("malformed markers must not fail: %v", err)
	}
	if readFile(t, claudePath(root)) != content {
		t.Fatal("malformed markers must leave the file untouched")
	}
	if !strings.Contains(stderr.String(), "managed block in CLAUDE.md: BEGIN marker present without") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRefresh_LegacyBlockSymlinkSkipped(t *testing.T) {
	t.Run("link to a file outside the checked set warns", func(t *testing.T) {
		root, cfg := newTestRoot(t)
		target := filepath.Join(root, "docs.md")
		content := legacyFile(t, "# P\n", legacyClaudeInterior)
		writeFile(t, target, content)
		if err := os.Symlink("docs.md", claudePath(root)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		o, _, stderr := warnOpts(root, cfg)
		if err := Refresh(o); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(claudePath(root))
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("CLAUDE.md must stay a symlink (err = %v)", err)
		}
		if readFile(t, target) != content {
			t.Fatal("the symlink target must be left untouched")
		}
		if !strings.Contains(stderr.String(), "CLAUDE.md is a symlink") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})
	t.Run("CLAUDE.md -> AGENTS.md: AGENTS.md cleaned, link kept, no warning", func(t *testing.T) {
		root, cfg := newTestRoot(t)
		writeFile(t, agentsPath(root), legacyFile(t, "# Shared\n", legacyAgentsInterior))
		if err := os.Symlink("AGENTS.md", claudePath(root)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		o, _, stderr := warnOpts(root, cfg)
		if err := Refresh(o); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, agentsPath(root)); got != "# Shared\n" {
			t.Fatalf("AGENTS.md = %q", got)
		}
		info, err := os.Lstat(claudePath(root))
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("CLAUDE.md must stay a symlink (err = %v)", err)
		}
		if stderr.Len() != 0 {
			t.Fatalf("no warning expected, got %q", stderr.String())
		}
	})
}

func TestRefresh_LegacyBlockOnlyFileRemovedWarnsAboutDanglingLink(t *testing.T) {
	root, cfg := newTestRoot(t)
	writeFile(t, agentsPath(root), legacyFile(t, "", legacyAgentsInterior))
	if err := os.Symlink("AGENTS.md", claudePath(root)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	o, _, stderr := warnOpts(root, cfg)
	if err := Refresh(o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(agentsPath(root)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("AGENTS.md should be removed (err = %v)", err)
	}
	want := "constitution: CLAUDE.md is a symlink to AGENTS.md, which held only the always-loaded constitution block and was removed; delete the link or point it elsewhere"
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// readAll returns the bytes of every regular file under root, keyed by path
// relative to root.
func readAll(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
