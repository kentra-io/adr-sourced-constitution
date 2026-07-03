package scaffold

import (
	"errors"
	"strings"
	"testing"
)

func mkBlock(interior string) string {
	return BlockBegin + "\n" + interior + "\n" + BlockEnd
}

func TestLocateBlock_Found(t *testing.T) {
	content := []byte("# Header\n\n" + mkBlock("@constitution/constitution.md") + "\n\ntrailer\n")
	found, begin, end, interior, err := LocateBlock(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected block to be found")
	}
	if interior != "@constitution/constitution.md" {
		t.Fatalf("interior = %q", interior)
	}
	if string(content[begin:end]) != mkBlock("@constitution/constitution.md") {
		t.Fatalf("span = %q", content[begin:end])
	}
}

func TestLocateBlock_Absent(t *testing.T) {
	found, _, _, _, err := LocateBlock([]byte("# just a file\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected not found")
	}
}

func TestLocateBlock_MalformedPairs(t *testing.T) {
	cases := map[string]string{
		"begin only": "x\n" + BlockBegin + "\ninterior\n",
		"end only":   "interior\n" + BlockEnd + "\n",
		"reversed":   BlockEnd + "\ninterior\n" + BlockBegin + "\n",
		"two begins": mkBlock("a") + "\n" + mkBlock("b"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, _, _, err := LocateBlock([]byte(content))
			if err == nil {
				t.Fatal("expected a MarkerError")
			}
			var me *MarkerError
			if !errors.As(err, &me) {
				t.Fatalf("expected *MarkerError, got %T", err)
			}
		})
	}
}

func TestLocateBlock_CRLF(t *testing.T) {
	// A CRLF-authored file must locate cleanly and yield the same interior as
	// its LF twin (surrounding CR/LF trimmed).
	content := []byte("# Header\r\n\r\n" + BlockBegin + "\r\n@constitution/constitution.md\r\n" + BlockEnd + "\r\n")
	found, _, _, interior, err := LocateBlock(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found")
	}
	if interior != "@constitution/constitution.md" {
		t.Fatalf("interior = %q, want clean LF interior", interior)
	}
}

func TestApplyBlock_ReplaceInterior(t *testing.T) {
	orig := []byte("# Keep me\n\n" + mkBlock("OLD") + "\n\n# Keep me too\n")
	out, err := ApplyBlock(orig, "NEW")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, mkBlock("NEW")) {
		t.Fatalf("interior not replaced:\n%s", s)
	}
	if strings.Contains(s, "OLD") {
		t.Fatalf("old interior leaked:\n%s", s)
	}
	if !strings.HasPrefix(s, "# Keep me\n\n") || !strings.HasSuffix(s, "# Keep me too\n") {
		t.Fatalf("content outside the block not preserved:\n%s", s)
	}
}

func TestApplyBlock_AppendEOF(t *testing.T) {
	orig := []byte("# Project\n\nSome guidance.\n")
	out, err := ApplyBlock(orig, "@constitution/constitution.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !strings.HasPrefix(s, "# Project\n\nSome guidance.\n\n") {
		t.Fatalf("existing content not preserved with a blank-line separator:\n%q", s)
	}
	if !strings.HasSuffix(s, BlockEnd+"\n") {
		t.Fatalf("appended block should end newline-terminated:\n%q", s)
	}
}

func TestApplyBlock_EmptyFile(t *testing.T) {
	out, err := ApplyBlock(nil, "@constitution/constitution.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := mkBlock("@constitution/constitution.md") + "\n"
	if string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestApplyBlock_Idempotent(t *testing.T) {
	orig := []byte("# Keep\n\n" + mkBlock("X") + "\ntrailer\n")
	out, err := ApplyBlock(orig, "X")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != string(orig) {
		t.Fatalf("re-applying the same interior must be byte-identical:\n got %q\nwant %q", out, orig)
	}
}

func TestApplyBlock_MalformedIsError(t *testing.T) {
	_, err := ApplyBlock([]byte(BlockBegin+"\nno end\n"), "X")
	var me *MarkerError
	if !errors.As(err, &me) {
		t.Fatalf("expected *MarkerError, got %v", err)
	}
}
