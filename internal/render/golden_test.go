package render_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
	"github.com/kentra-io/adr-sourced-constitution/internal/render"
)

var update = flag.Bool("update", false, "update golden files (testdata/golden/*/constitution.md)")

// renderFixture loads a testdata/golden/<name> fixture (constitution.yml +
// constitution/adr/*.md) and renders it, failing the test on any error.
func renderFixture(t *testing.T, name string) []byte {
	t.Helper()
	dir := filepath.Join("testdata", "golden", name)

	cfg, err := config.Load(filepath.Join(dir, "constitution.yml"))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	adrs, err := adr.ParseDir(filepath.Join(dir, "constitution", "adr"))
	if err != nil {
		t.Fatalf("adr.ParseDir: %v", err)
	}

	out, err := render.Render(cfg, adrs)
	if err != nil {
		t.Fatalf("render.Render: %v", err)
	}
	return out
}

// TestGolden is the M1/M5.5 DoD golden suite, asserting byte-exact
// constitution.md projections:
//   - fixture1: a mixed rule/record log — ~12 ADRs across 6 categories,
//     some active rule-bearing (project), some active record-only (stay in
//     the log only), a three-link supersede chain (ADR-0001 -> ADR-0005 ->
//     ADR-0009) and a deprecated entry (ADR-0004). Two categories (process,
//     data) have only record-only active ADRs and are omitted entirely
//     (plan §2.12 category omission).
//   - empty: no active ADR is rule-bearing (one record-only accepted, one
//     deprecated rule) — the placeholder form (plan §2.12).
func TestGolden(t *testing.T) {
	for _, name := range []string{"fixture1", "empty"} {
		t.Run(name, func(t *testing.T) {
			got := renderFixture(t, name)

			golden := filepath.Join("testdata", "golden", name, "constitution", "constitution.md")
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden: %v (run go test -update to create it)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("rendered output does not match golden %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
			}
		})
	}
}

// TestGoldenFixture1Determinism is the M1 DoD "byte-identical across 100
// runs" determinism proof: renders the same fixture 100 times in-process
// and asserts every run produces identical bytes. Cross-OS byte-equality
// is proven by TestGoldenFixture1 itself running on the full CI OS
// matrix (a checked-in golden target is inherently a cross-OS assertion).
func TestGoldenFixture1Determinism(t *testing.T) {
	dir := filepath.Join("testdata", "golden", "fixture1")

	cfg, err := config.Load(filepath.Join(dir, "constitution.yml"))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	adrs, err := adr.ParseDir(filepath.Join(dir, "constitution", "adr"))
	if err != nil {
		t.Fatalf("adr.ParseDir: %v", err)
	}

	first, err := render.Render(cfg, adrs)
	if err != nil {
		t.Fatalf("render.Render: %v", err)
	}
	for i := 0; i < 100; i++ {
		got, err := render.Render(cfg, adrs)
		if err != nil {
			t.Fatalf("render.Render (run %d): %v", i, err)
		}
		if !bytes.Equal(got, first) {
			t.Fatalf("render.Render is nondeterministic: run %d differs from run 0", i)
		}
	}
}
