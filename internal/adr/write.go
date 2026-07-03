package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FormatID renders a numeric id as the canonical "ADR-NNNN" string,
// zero-padded to at least four digits. Past 9999 the width grows naturally
// ("ADR-10000"): ids stay unique and numerically sortable (the authoritative
// projection order sorts by Num, not by filename), at the cost of lexical
// filename order diverging from numeric order beyond 9999 — which affects
// only ParseDir's error-reporting order, never the constitution.md output.
func FormatID(num int) string {
	return fmt.Sprintf("ADR-%04d", num)
}

// NextID scans dir for the highest existing "ADR-NNNN" filename and returns
// the next id (one greater). An empty or absent adr/ directory yields
// ADR-0001. It reads filenames only — allocation is optimistic (plan §2.6),
// same as adr-tools; concurrent-branch collisions are a CI-time concern
// resolved by `guard` (M3) and the `renumber` escape hatch, not a lock here.
func NextID(dir string) (num int, id string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, FormatID(1), nil
		}
		return 0, "", err
	}

	highest := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fnID, ok := filenameID(e.Name())
		if !ok {
			continue
		}
		if n, ok := parseID(fnID); ok && n > highest {
			highest = n
		}
	}
	next := highest + 1
	return next, FormatID(next), nil
}

// ValidID reports whether id is a well-formed "ADR-NNNN" identifier. Used
// by the write path (renumber) to reject malformed ids before touching the
// log, with the same format rule the parser enforces.
func ValidID(id string) bool {
	_, ok := parseID(id)
	return ok
}

// ValidateBody checks that a supplied MADR body (the "## " sections a
// caller hands to `adr new`) contains every mandatory section (plan §1
// errata #1 / MandatorySections). It reuses the exact same extraction and
// validation seam the read path uses, so "valid on write" and "valid on
// read" can never drift apart. file is used only for the error's location.
func ValidateBody(body []byte, file string) error {
	body = normalize(body)
	sections, _ := ExtractSections(body)
	if err := validateSections(sections, MandatorySections, file); err != nil {
		return err
	}
	return validateRuleSection(sections, file)
}

// HasRuleSection reports whether a MADR body already carries a "## Rule"
// section. The write path uses it to reject supplying both --rule and a
// body-file that already carries its own Rule section (plan §2.12).
func HasRuleSection(body []byte) bool {
	sections, _ := ExtractSections(normalize(body))
	_, present := sections[RuleSection]
	return present
}

// AppendRuleSection appends a "## Rule" section carrying rule as the LAST
// body section (plan §2.12: the CLI composes --rule at the canonical last
// position). The body is normalized and its trailing blank lines trimmed so
// the composed section is separated by exactly one blank line. An
// empty/whitespace rule composes to an empty section that ValidateBody then
// rejects — the CLI never writes it.
func AppendRuleSection(body []byte, rule string) []byte {
	base := strings.TrimRight(normalizeToString(string(body)), "\n")
	stmt := strings.TrimSpace(normalizeToString(rule))
	return []byte(base + "\n\n## " + RuleSection + "\n\n" + stmt + "\n")
}

// NewADR is the input to Compose: the fields a freshly accepted ADR needs.
// Status is always "accepted" on creation (spec §5.1) and so is not a
// field here; Date is passed in (frozen at creation) rather than read from
// the clock, keeping Compose a pure function.
type NewADR struct {
	ID         string
	Title      string
	Category   string
	Date       string
	Source     string // "" when sourceTracking.type is none
	Supersedes string // "" unless created by `supersede`
	Body       string // the validated MADR "## " sections
}

// Compose builds the full byte content of a new ADR file: a frontmatter
// block followed by the body. Field order is fixed (id, title, category,
// date, status, then the optional source/supersedes) so newly created ADRs
// are uniform; the order is immaterial to correctness since parsing is
// key-based, but a stable order keeps the log readable and diffs clean.
// The output is always LF-terminated with a single trailing newline.
func Compose(n NewADR) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + n.ID + "\n")
	b.WriteString("title: " + yamlScalar(n.Title) + "\n")
	b.WriteString("category: " + yamlScalar(n.Category) + "\n")
	b.WriteString("date: " + n.Date + "\n")
	b.WriteString("status: " + string(StatusAccepted) + "\n")
	if n.Source != "" {
		b.WriteString("source: " + yamlScalar(n.Source) + "\n")
	}
	if n.Supersedes != "" {
		b.WriteString("supersedes: " + n.Supersedes + "\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(normalizeToString(n.Body)))
	b.WriteString("\n")
	return []byte(b.String())
}

// Filename returns the "ADR-NNNN-slug.md" filename for an ADR with the
// given id and title, matching the convention the parser enforces
// (filenamePattern).
func Filename(id, title string) string {
	return id + "-" + Slugify(title) + ".md"
}

// maxSlugLen bounds the slug portion of a filename so a very long title can
// never produce a filename past the common 255-byte path-component limit
// (the id prefix + ".md" suffix leave ample headroom). The slug is only for
// human-readable filenames; the full title is preserved verbatim in
// frontmatter, so truncating it here is lossless for the log's meaning.
const maxSlugLen = 80

// Slugify turns a title into a filename-safe slug: lowercase, runs of
// non-alphanumeric characters collapsed to a single hyphen, trimmed, and
// bounded to maxSlugLen at a hyphen boundary. An empty result (a title with
// no alphanumerics) falls back to "adr" so a filename is always well-formed.
func Slugify(title string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(title) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "adr"
	}
	return boundSlug(slug)
}

// boundSlug caps slug at maxSlugLen, preferring to cut back to the last
// hyphen boundary so a word is not split; a single unbroken word longer than
// the bound is hard-truncated. The result is deterministic and never empty
// (slug is already non-empty and hyphen-trimmed on entry).
func boundSlug(slug string) string {
	if len(slug) <= maxSlugLen {
		return slug
	}
	truncated := slug[:maxSlugLen]
	if i := strings.LastIndexByte(truncated, '-'); i > 0 {
		truncated = truncated[:i]
	}
	return strings.Trim(truncated, "-")
}

// FindByID returns the path of the ADR file in dir whose filename encodes
// id, or ok=false if none does. Used by the mutating verbs to locate the
// target of a supersede/deprecate/renumber.
func FindByID(dir, id string) (path string, ok bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if fnID, ok := filenameID(e.Name()); ok && fnID == id {
			return filepath.Join(dir, e.Name()), true, nil
		}
	}
	return "", false, nil
}

// normalizeToString applies the same BOM-strip + CRLF->LF normalization the
// parser uses, so a body supplied from a Windows editor composes to the
// same bytes as one from a unix editor.
func normalizeToString(s string) string {
	return string(normalize([]byte(s)))
}

// yamlScalar renders s as a YAML scalar, double-quoting only when a plain
// scalar would be misparsed (contains ": " or a trailing/leading space, is
// empty, or starts with a YAML indicator character). Simple titles stay
// unquoted for readability; anything ambiguous is quoted and escaped.
func yamlScalar(s string) string {
	if s == "" || needsQuoting(s) {
		r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
		return `"` + r.Replace(s) + `"`
	}
	return s
}

func needsQuoting(s string) bool {
	if s != strings.TrimSpace(s) {
		return true
	}
	if strings.Contains(s, ": ") || strings.HasSuffix(s, ":") {
		return true
	}
	if strings.ContainsAny(s, "\n\"#") {
		return true
	}
	// Leading YAML indicator characters that change how a plain scalar parses.
	switch s[0] {
	case '-', '?', ':', ',', '[', ']', '{', '}', '&', '*', '!', '|', '>',
		'\'', '"', '%', '@', '`':
		return true
	}
	return false
}
