package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatID(t *testing.T) {
	tests := []struct {
		num  int
		want string
	}{
		{1, "ADR-0001"},
		{7, "ADR-0007"},
		{42, "ADR-0042"},
		{9999, "ADR-9999"},
		{10000, "ADR-10000"}, // width grows naturally past 9999
		{123456, "ADR-123456"},
	}
	for _, tt := range tests {
		if got := FormatID(tt.num); got != tt.want {
			t.Errorf("FormatID(%d) = %q, want %q", tt.num, got, tt.want)
		}
	}
}

func TestNextID(t *testing.T) {
	t.Run("empty dir", func(t *testing.T) {
		num, id, err := NextID(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if num != 1 || id != "ADR-0001" {
			t.Errorf("NextID(empty) = %d/%q, want 1/ADR-0001", num, id)
		}
	})

	t.Run("absent dir", func(t *testing.T) {
		num, id, err := NextID(filepath.Join(t.TempDir(), "does-not-exist"))
		if err != nil {
			t.Fatalf("NextID(absent) should not error: %v", err)
		}
		if num != 1 || id != "ADR-0001" {
			t.Errorf("NextID(absent) = %d/%q, want 1/ADR-0001", num, id)
		}
	})

	t.Run("gaps and non-adr files ignored", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{
			"ADR-0001-a.md",
			"ADR-0003-c.md", // a gap at 2
			"ADR-0002-b.md",
			"README.md",              // not an ADR filename
			".manifest.sha256",       // manifest, ignored
			"constitution.md",        // not an ADR filename
			"notes.txt",              // wrong extension
			"ADR-0007-superseded.md", // superseded ADRs still count toward the max
		} {
			mustWrite(t, filepath.Join(dir, name))
		}
		num, id, err := NextID(dir)
		if err != nil {
			t.Fatal(err)
		}
		if num != 8 || id != "ADR-0008" {
			t.Errorf("NextID = %d/%q, want 8/ADR-0008", num, id)
		}
	})

	t.Run("past 9999 keeps counting", func(t *testing.T) {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "ADR-9999-x.md"))
		num, id, err := NextID(dir)
		if err != nil {
			t.Fatal(err)
		}
		if num != 10000 || id != "ADR-10000" {
			t.Errorf("NextID = %d/%q, want 10000/ADR-10000", num, id)
		}
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Use event sourcing", "use-event-sourcing"},
		{"Model the constitution as an event-sourced ADR log", "model-the-constitution-as-an-event-sourced-adr-log"},
		{"Trailing punctuation!!!", "trailing-punctuation"},
		{"  spaces  ", "spaces"},
		{"Multiple   spaces & symbols", "multiple-spaces-symbols"},
		{"CamelCase123", "camelcase123"},
		{"日本語", "adr"}, // no ascii alphanumerics -> fallback
		{"", "adr"},
	}
	for _, tt := range tests {
		if got := Slugify(tt.in); got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSlugifyBounded(t *testing.T) {
	// A pathologically long title must not yield an unbounded (>255-byte)
	// filename. The slug is capped at a hyphen boundary, stays non-empty, and
	// is deterministic across calls.
	words := make([]string, 60)
	for i := range words {
		words[i] = "word"
	}
	long := strings.Join(words, " ") // "word word word ..." -> 60 hyphenated tokens

	got := Slugify(long)
	if len(got) > maxSlugLen {
		t.Fatalf("Slugify(long) length = %d, want <= %d", len(got), maxSlugLen)
	}
	if got == "" || got == "adr" {
		t.Fatalf("Slugify(long) = %q, want a real bounded slug", got)
	}
	if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
		t.Fatalf("Slugify(long) = %q, must not have leading/trailing hyphens", got)
	}
	if got != Slugify(long) {
		t.Fatal("Slugify is not deterministic for a long title")
	}
	// The bounded slug keeps the leading words of the title (cut at a boundary).
	if !strings.HasPrefix(got, "word-word") {
		t.Fatalf("Slugify(long) = %q, expected to start with the title's words", got)
	}
	// A single unbroken over-long word is hard-truncated to the bound.
	oneWord := strings.Repeat("a", 200)
	if l := len(Slugify(oneWord)); l != maxSlugLen {
		t.Fatalf("Slugify(one long word) length = %d, want %d", l, maxSlugLen)
	}
}

func TestSlugifyUnderBoundUnchanged(t *testing.T) {
	// Titles under the bound must be untouched (golden-file stability).
	for _, s := range []string{
		"use-event-sourcing",
		"model-the-constitution-as-an-event-sourced-adr-log",
	} {
		if got := Slugify(strings.ReplaceAll(s, "-", " ")); got != s {
			t.Errorf("Slugify short title = %q, want %q", got, s)
		}
	}
}

func TestFilename(t *testing.T) {
	got := Filename("ADR-0007", "Use event sourcing")
	want := "ADR-0007-use-event-sourcing.md"
	if got != want {
		t.Errorf("Filename = %q, want %q", got, want)
	}
	// The composed filename must satisfy the parser's own pattern.
	if id, ok := filenameID(got); !ok || id != "ADR-0007" {
		t.Errorf("Filename produced %q, which does not round-trip through filenameID (id=%q ok=%v)", got, id, ok)
	}
}

func TestValidID(t *testing.T) {
	for _, id := range []string{"ADR-0001", "ADR-9999", "ADR-10000"} {
		if !ValidID(id) {
			t.Errorf("ValidID(%q) = false, want true", id)
		}
	}
	for _, id := range []string{"ADR-1", "adr-0001", "ADR-001", "0001", "ADR-000X", ""} {
		if ValidID(id) {
			t.Errorf("ValidID(%q) = true, want false", id)
		}
	}
}

func TestValidateBody(t *testing.T) {
	full := "## Context and Problem Statement\n\nx\n\n## Considered Options\n\n- a\n\n## Decision Outcome\n\ny\n"
	if err := ValidateBody([]byte(full), "b.md"); err != nil {
		t.Errorf("ValidateBody(full) = %v, want nil", err)
	}

	missing := "## Context and Problem Statement\n\nx\n\n## Decision Outcome\n\ny\n"
	err := ValidateBody([]byte(missing), "b.md")
	if err == nil {
		t.Fatal("ValidateBody(missing Considered Options) = nil, want error")
	}
	want := `b.md: field "Considered Options": required section "## Considered Options" is missing`
	if err.Error() != want {
		t.Errorf("ValidateBody error = %q, want %q", err.Error(), want)
	}
}

func TestValidateBodyCRLF(t *testing.T) {
	// A CRLF-authored body validates the same as its LF twin.
	full := "## Context and Problem Statement\r\n\r\nx\r\n\r\n## Considered Options\r\n\r\n- a\r\n\r\n## Decision Outcome\r\n\r\ny\r\n"
	if err := ValidateBody([]byte(full), "b.md"); err != nil {
		t.Errorf("ValidateBody(CRLF) = %v, want nil", err)
	}
}

func TestCompose(t *testing.T) {
	body := "## Context and Problem Statement\n\nWhy.\n\n## Considered Options\n\n- a\n\n## Decision Outcome\n\nDo it.\n"
	out := Compose(NewADR{
		ID: "ADR-0007", Title: "Use event sourcing", Category: "architecture",
		Date: "2026-07-01", Source: "FS-0042", Supersedes: "ADR-0003", Body: body,
	})

	// It must parse back to exactly the model we composed.
	a, err := ParseBytes(out, "ADR-0007-use-event-sourcing.md")
	if err != nil {
		t.Fatalf("composed ADR does not parse: %v\n%s", err, out)
	}
	if a.ID != "ADR-0007" || a.Title != "Use event sourcing" || a.Category != "architecture" ||
		a.Date != "2026-07-01" || a.Source != "FS-0042" || a.Supersedes != "ADR-0003" ||
		a.Status != StatusAccepted {
		t.Errorf("composed model mismatch: %+v", a)
	}
	if a.Sections[DecisionOutcomeSection] != "Do it." {
		t.Errorf("Decision Outcome = %q, want %q", a.Sections[DecisionOutcomeSection], "Do it.")
	}
}

func TestComposeOmitsOptionalFields(t *testing.T) {
	body := "## Context and Problem Statement\n\nx\n\n## Considered Options\n\n- a\n\n## Decision Outcome\n\ny\n"
	out := string(Compose(NewADR{
		ID: "ADR-0001", Title: "T", Category: "architecture", Date: "2026-07-01", Body: body,
	}))
	if contains(out, "source:") {
		t.Errorf("expected no source line when Source is empty:\n%s", out)
	}
	if contains(out, "supersedes:") {
		t.Errorf("expected no supersedes line when Supersedes is empty:\n%s", out)
	}
}

func TestComposeQuotesAmbiguousTitle(t *testing.T) {
	body := "## Context and Problem Statement\n\nx\n\n## Considered Options\n\n- a\n\n## Decision Outcome\n\ny\n"
	// A title containing ": " would break an unquoted YAML scalar; Compose
	// must quote it and it must round-trip.
	title := "Prefer A: not B"
	out := Compose(NewADR{ID: "ADR-0001", Title: title, Category: "architecture", Date: "2026-07-01", Body: body})
	a, err := ParseBytes(out, "ADR-0001-x.md")
	if err != nil {
		t.Fatalf("composed ADR with ambiguous title does not parse: %v\n%s", err, out)
	}
	if a.Title != title {
		t.Errorf("title round-trip = %q, want %q", a.Title, title)
	}
}

func TestFindByID(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ADR-0001-a.md"))
	mustWrite(t, filepath.Join(dir, "ADR-0002-b.md"))

	path, ok, err := FindByID(dir, "ADR-0002")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || filepath.Base(path) != "ADR-0002-b.md" {
		t.Errorf("FindByID(ADR-0002) = %q/%v, want ADR-0002-b.md/true", path, ok)
	}

	if _, ok, _ := FindByID(dir, "ADR-0099"); ok {
		t.Error("FindByID(ADR-0099) ok = true, want false")
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
