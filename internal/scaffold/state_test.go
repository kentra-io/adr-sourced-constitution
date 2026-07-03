package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestState_RoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "constitution"), 0o755); err != nil {
		t.Fatal(err)
	}

	st := newState()
	st.set("CLAUDE.md", "aaa")
	st.set(".claude/skills/adr-draft/SKILL.md", "bbb")
	if err := st.Save(root); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := LoadState(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if h, ok := got.get("CLAUDE.md"); !ok || h != "aaa" {
		t.Fatalf("CLAUDE.md hash = %q, %v", h, ok)
	}
	if h, ok := got.get(".claude/skills/adr-draft/SKILL.md"); !ok || h != "bbb" {
		t.Fatalf("skill hash = %q, %v", h, ok)
	}
}

func TestState_DeterministicOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "constitution"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Insert in non-sorted order; on-disk bytes must be sorted by path and
	// therefore stable across runs (the idempotency requirement).
	st := newState()
	for _, p := range []string{"zeta", "alpha", "mid", "beta"} {
		st.set(p, "h-"+p)
	}
	if err := st.Save(root); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(statePath(root))
	if err != nil {
		t.Fatal(err)
	}

	st2 := newState()
	for _, p := range []string{"beta", "zeta", "alpha", "mid"} {
		st2.set(p, "h-"+p)
	}
	if err := st2.Save(root); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(statePath(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("state serialization is not order-stable:\n%s\n---\n%s", first, second)
	}
}

func TestState_MissingIsEmpty(t *testing.T) {
	st, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatalf("missing .state should load as empty, got %v", err)
	}
	if !st.empty() {
		t.Fatal("expected empty state")
	}
}

func TestState_BadSchemaVersionRefused(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "constitution")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".state"), []byte("schemaVersion: 99\nmanaged: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(root); err == nil {
		t.Fatal("expected an error for an unsupported schemaVersion")
	}
}
