package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseNameStatus(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []diffEntry
	}{
		{name: "empty", out: "", want: nil},
		{
			name: "added",
			out:  "A\tconstitution/adr/ADR-0002-new.md\n",
			want: []diffEntry{{status: 'A', path: "constitution/adr/ADR-0002-new.md"}},
		},
		{
			name: "modified",
			out:  "M\tconstitution/adr/ADR-0001-a.md\n",
			want: []diffEntry{{status: 'M', path: "constitution/adr/ADR-0001-a.md"}},
		},
		{
			name: "deleted",
			out:  "D\tconstitution/adr/ADR-0001-a.md\n",
			want: []diffEntry{{status: 'D', path: "constitution/adr/ADR-0001-a.md"}},
		},
		{
			name: "renamed with similarity score",
			out:  "R087\tconstitution/adr/ADR-0001-a.md\tconstitution/adr/ADR-0001-b.md\n",
			want: []diffEntry{{status: 'R', oldPath: "constitution/adr/ADR-0001-a.md", path: "constitution/adr/ADR-0001-b.md"}},
		},
		{
			name: "multiple lines, no trailing newline",
			out:  "A\tconstitution/adr/ADR-0002-new.md\nD\tconstitution/adr/ADR-0001-a.md",
			want: []diffEntry{
				{status: 'A', path: "constitution/adr/ADR-0002-new.md"},
				{status: 'D', path: "constitution/adr/ADR-0001-a.md"},
			},
		},
		{
			name: "blank lines tolerated",
			out:  "A\tconstitution/adr/ADR-0002-new.md\n\n",
			want: []diffEntry{{status: 'A', path: "constitution/adr/ADR-0002-new.md"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNameStatus(tt.out)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseNameStatus(%q) = %+v, want %+v", tt.out, got, tt.want)
			}
		})
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
