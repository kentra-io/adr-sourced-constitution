// Structural assertions on the shipped constitution-init skill text (M6,
// "founding body as ADR body + config writer" plan, issue #19). These read
// SKILL.md through the embedded SkillsFS — never off disk — so they assert
// on what actually ships in the binary, not on a working-tree copy that
// might drift from what gets embedded.
package constitution

import (
	"regexp"
	"strings"
	"testing"
)

const constitutionInitSkillPath = "skills/constitution-init/SKILL.md"

// readConstitutionInitSkill reads constitution-init's SKILL.md through the
// embedded SkillsFS (embed.go), matching what `constitution init` and
// `constitution regen` actually fan out to a target repo.
func readConstitutionInitSkill(t *testing.T) string {
	t.Helper()
	b, err := SkillsFS.ReadFile(constitutionInitSkillPath)
	if err != nil {
		t.Fatalf("reading %s from the embedded SkillsFS: %v", constitutionInitSkillPath, err)
	}
	return string(b)
}

// fence matches a fenced-code-block delimiter line (``` or ~~~, optionally
// indented, optionally followed by a language tag).
var fence = regexp.MustCompile("^\\s*(```|~~~)")

// fencedMask returns, for each line in lines, whether that line is a fence
// delimiter or falls inside a fenced code block — i.e. is example content,
// not document structure, and must be ignored when hunting for headings. A
// "## " (or "```" containing "##") line inside a worked example (a founding
// file, a shell command) is exactly the kind of content this must not
// mistake for a real heading.
func fencedMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inFence := false
	for i, line := range lines {
		if fence.MatchString(line) {
			inFence = !inFence
			mask[i] = true // the delimiter line itself is never a heading
			continue
		}
		mask[i] = inFence
	}
	return mask
}

// headingLines returns every Markdown heading line ("#"-prefixed) found
// outside fenced code blocks, in document order, with the leading "#"s and
// surrounding whitespace stripped.
func headingLines(doc string) []string {
	lines := strings.Split(doc, "\n")
	skip := fencedMask(lines)

	var out []string
	for i, line := range lines {
		if skip[i] {
			continue
		}
		trimmed := strings.TrimLeft(line, "#")
		if trimmed != line && strings.HasPrefix(strings.TrimSpace(line), "#") {
			out = append(out, strings.TrimSpace(trimmed))
		}
	}
	return out
}

// level2Section returns the text of the level-2 ("## ") section whose
// heading contains headingSubstr (first match, document order, fenced code
// blocks excluded from the heading search), from just after that heading
// line up to (but not including) the next level-2 heading or EOF. Nested
// "### "/"#### " subheadings, and any fenced examples, stay inside the
// section verbatim — only another real "## " heading closes it.
func level2Section(t *testing.T, doc, headingSubstr string) string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	skip := fencedMask(lines)
	level2 := regexp.MustCompile(`^## `)

	start := -1
	for i, line := range lines {
		if skip[i] {
			continue
		}
		if level2.MatchString(line) && strings.Contains(line, headingSubstr) {
			start = i + 1
			break
		}
	}
	if start == -1 {
		t.Fatalf("no level-2 heading containing %q found in constitution-init/SKILL.md", headingSubstr)
	}

	end := len(lines)
	for i := start; i < len(lines); i++ {
		if skip[i] {
			continue
		}
		if level2.MatchString(lines[i]) {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// wordBoundary compiles a case-sensitive \b<word>\b matcher.
func wordBoundary(word string) *regexp.Regexp {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
}

// TestConstitutionInitTeachesFoundValidateSettle asserts the ordering fix
// issue #19 opens with: the Found/Validate/Settle roadmap must be a spoken
// preamble that appears BEFORE the interview question catalog, not buried
// after it.
func TestConstitutionInitTeachesFoundValidateSettle(t *testing.T) {
	doc := readConstitutionInitSkill(t)
	headings := headingLines(doc)
	if len(headings) == 0 {
		t.Fatalf("no headings found in constitution-init/SKILL.md — is it empty or malformed?")
	}

	found, validate, settle := wordBoundary("Found"), wordBoundary("Validate"), wordBoundary("Settle")
	interview := wordBoundary("Interview")

	roadmapIdx, interviewIdx := -1, -1
	for i, h := range headings {
		if roadmapIdx == -1 && found.MatchString(h) && validate.MatchString(h) && settle.MatchString(h) {
			roadmapIdx = i
		}
		if interviewIdx == -1 && interview.MatchString(h) {
			interviewIdx = i
		}
	}

	if roadmapIdx == -1 {
		t.Fatalf("no heading names Found, Validate and Settle together; headings in order: %q", headings)
	}
	if interviewIdx == -1 {
		t.Fatalf("no heading names the Interview question catalog; headings in order: %q", headings)
	}
	if roadmapIdx >= interviewIdx {
		t.Fatalf("the Found/Validate/Settle roadmap heading (%q, position %d) must appear BEFORE the "+
			"Interview question-catalog heading (%q, position %d) — this is the ordering defect issue #19 opens with",
			headings[roadmapIdx], roadmapIdx, headings[interviewIdx], interviewIdx)
	}
}

// TestConstitutionInitDropsStaleGrammarAndHandEdits asserts the skill no
// longer teaches the two mechanisms M1-M4 deleted: hand-editing
// constitution.yml (superseded by `init --source-tracking/--source-pattern`
// and `constitution config set`), and the pre-M4 multi-principle founding
// grammar (one principle per "## <title>" heading, each with its own
// attached "## Rules" block, one ADR seeded per principle).
func TestConstitutionInitDropsStaleGrammarAndHandEdits(t *testing.T) {
	doc := readConstitutionInitSkill(t)
	normalized := strings.Join(strings.Fields(doc), " ")

	normalize := func(s string) string {
		return strings.Join(strings.Fields(s), " ")
	}

	staleHandEditInstruction := normalize("edit `constitution.yml`'s `sourceTracking` block now")
	if strings.Contains(normalized, staleHandEditInstruction) {
		t.Errorf("SKILL.md still instructs hand-editing constitution.yml's sourceTracking block "+
			"(found %q); this is superseded by `init --source-tracking/--source-pattern` and "+
			"`constitution config set`", staleHandEditInstruction)
	}

	staleMultiPrincipleGrammar := []string{
		"one principle per",
		"attached `## Rules`",
		"attached ## Rules",
		"per principle",
		"catalog-only (record-only) ADR",
	}
	for _, phrase := range staleMultiPrincipleGrammar {
		needle := normalize(phrase)
		if strings.Contains(normalized, needle) {
			t.Errorf("SKILL.md still contains %q, a marker of the deleted multi-principle founding "+
				"grammar (one ADR per principle, one attached ## Rules block per principle) — the "+
				"founding file now seeds exactly ONE founding ADR", phrase)
		}
	}

	// The multi-ADR-per-principle idea specifically: init used to seed one
	// ADR per principle. It now always seeds exactly one.
	if regexp.MustCompile(`(?i)seeds? (one|a) (founding )?ADR per (each )?principle`).MatchString(normalized) {
		t.Errorf("SKILL.md still describes seeding one ADR per principle; init now seeds exactly one founding ADR (ADR-0001) total")
	}
}

// TestConstitutionInitElicitsPurpose asserts the gap issue #19 names is
// closed: the question catalog must explicitly ask what the project is
// for, not just ship `purpose` in the starter category vocabulary and
// never ask about it.
func TestConstitutionInitElicitsPurpose(t *testing.T) {
	doc := readConstitutionInitSkill(t)
	catalog := level2Section(t, doc, "Interview")

	purposeQuestion := regexp.MustCompile(`(?is)\*\*Purpose\.\*\*[^\n]*\?`)
	if !purposeQuestion.MatchString(catalog) {
		t.Fatalf("the Interview question catalog contains no explicit question eliciting the "+
			"project's purpose (expected something like \"**Purpose.** What is this project for?\"); "+
			"catalog section:\n%s", catalog)
	}

	// The elicited purpose must be tied to where it lands, so the human
	// isn't left wondering where their answer went. This must name the
	// `purpose` CATEGORY specifically (step 5's requirement) — a mention of
	// a "`purpose` rule" elsewhere doesn't satisfy it, since that names the
	// triage outcome, not the category the rule renders under.
	if !strings.Contains(catalog, "`purpose` category") {
		t.Errorf("the catalog asks about purpose but never states that a settled answer lands as " +
			"rules under the `purpose` category (expected the literal phrase \"`purpose` category\")")
	}
}
