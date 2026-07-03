package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// TestParseNameStatus exercises the -z (NUL-delimited) parser. The tokens
// git emits are separated AND terminated by \x00: a status token followed by
// one path token, or for R/C two path tokens (old then new). Paths are raw
// bytes — never C-quoted — which is the whole point of -z: the non-ASCII case
// below is the exact C1 regression (a quoted path ending in `.md"` used to
// evade the extension filter).
func TestParseNameStatus(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []diffEntry
	}{
		{name: "empty", out: "", want: nil},
		{
			name: "added",
			out:  "A\x00constitution/adr/ADR-0002-new.md\x00",
			want: []diffEntry{{status: 'A', path: "constitution/adr/ADR-0002-new.md"}},
		},
		{
			name: "modified",
			out:  "M\x00constitution/adr/ADR-0001-a.md\x00",
			want: []diffEntry{{status: 'M', path: "constitution/adr/ADR-0001-a.md"}},
		},
		{
			name: "deleted",
			out:  "D\x00constitution/adr/ADR-0001-a.md\x00",
			want: []diffEntry{{status: 'D', path: "constitution/adr/ADR-0001-a.md"}},
		},
		{
			name: "renamed with similarity score",
			out:  "R087\x00constitution/adr/ADR-0001-a.md\x00constitution/adr/ADR-0001-b.md\x00",
			want: []diffEntry{{status: 'R', oldPath: "constitution/adr/ADR-0001-a.md", path: "constitution/adr/ADR-0001-b.md"}},
		},
		{
			name: "rename to a non-ASCII path is preserved verbatim (C1 regression)",
			out:  "R089\x00constitution/adr/ADR-0001-first-rule.md\x00constitution/adr/ADR-0001-première-règle.md\x00",
			want: []diffEntry{{status: 'R', oldPath: "constitution/adr/ADR-0001-first-rule.md", path: "constitution/adr/ADR-0001-première-règle.md"}},
		},
		{
			name: "multiple records including a rename",
			out:  "A\x00constitution/adr/ADR-0002-new.md\x00R100\x00constitution/adr/ADR-0001-a.md\x00constitution/adr/ADR-0001-b.md\x00D\x00constitution/adr/ADR-0003-c.md\x00",
			want: []diffEntry{
				{status: 'A', path: "constitution/adr/ADR-0002-new.md"},
				{status: 'R', oldPath: "constitution/adr/ADR-0001-a.md", path: "constitution/adr/ADR-0001-b.md"},
				{status: 'D', path: "constitution/adr/ADR-0003-c.md"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNameStatus(tt.out)
			if err != nil {
				t.Fatalf("parseNameStatus(%q) error = %v", tt.out, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseNameStatus(%q) = %+v, want %+v", tt.out, got, tt.want)
			}
		})
	}
}

// TestParseNameStatusTruncated: a record whose path token(s) are missing is a
// parse failure, not something to skip silently — guard fails closed (exit 2)
// rather than risk overlooking a mutation whose path it couldn't read.
func TestParseNameStatusTruncated(t *testing.T) {
	for _, out := range []string{
		"M\x00", // modify with no path
		"R089\x00constitution/adr/ADR-0001-a.md\x00", // rename missing new path
	} {
		if _, err := parseNameStatus(out); err == nil {
			t.Errorf("parseNameStatus(%q) = nil error, want a malformed-output error", out)
		}
	}
}

// requireGit skips the test if the system git binary is unavailable —
// every CI leg has one, but this keeps the unit suite honest about the
// dependency rather than failing opaquely.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found on PATH")
	}
}

func TestRequireRepoRootIsGitTop(t *testing.T) {
	requireGit(t)

	root := t.TempDir()
	if _, err := runGit(root, "init", "-q"); err != nil {
		t.Fatal(err)
	}

	t.Run("root matches git top level", func(t *testing.T) {
		if err := requireRepoRootIsGitTop(root); err != nil {
			t.Errorf("requireRepoRootIsGitTop() = %v, want nil", err)
		}
	})

	t.Run("a subdirectory of the repo is rejected", func(t *testing.T) {
		sub := filepath.Join(root, "nested")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := requireRepoRootIsGitTop(sub); err == nil {
			t.Error("requireRepoRootIsGitTop(nested) = nil, want an error (nested layouts are out of scope for v1)")
		}
	})
}
