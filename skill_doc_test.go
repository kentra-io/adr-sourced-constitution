// Structural assertions on the shipped constitution-init skill text (M6,
// "founding body as ADR body + config writer" plan, issue #19). These read
// SKILL.md through the embedded SkillsFS — never off disk — so they assert
// on what actually ships in the binary, not on a working-tree copy that
// might drift from what gets embedded.
package constitution

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
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

// --- Milestone 7 ("ship-docs-and-plugin-bump"): plugin-version and
// doc-sweep assertions, added on top of the M6 skill-text checks above. ---

// pluginManifestPath is .claude-plugin/plugin.json — not embedded (SkillsFS
// only carries skills/), read straight off disk. This package's directory
// is the repo root, so it's a direct sibling of this test file.
const pluginManifestPath = ".claude-plugin/plugin.json"

// lastSkillReleaseVersion is the plugin version that shipped the skills
// this change rewrites (skills/adr-draft/SKILL.md's #31 hazard block, and
// the `## Requires` block added to all four for #32). `claude plugin
// update` diffs plugin.json's version field, not skill content, so an edit
// that ships without a bump past this never reaches an installed copy.
const lastSkillReleaseVersion = "0.3.0"

// TestPluginVersionAheadOfLastRelease is milestone 7's criterion 1:
// .claude-plugin/plugin.json's version must be strictly greater than
// lastSkillReleaseVersion, so the bundled-skill edits in this change cannot
// ship inert.
func TestPluginVersionAheadOfLastRelease(t *testing.T) {
	b, err := os.ReadFile(pluginManifestPath)
	if err != nil {
		t.Fatalf("reading %s: %v", pluginManifestPath, err)
	}

	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatalf("parsing %s: %v", pluginManifestPath, err)
	}
	if manifest.Version == "" {
		t.Fatalf("%s has no \"version\" field", pluginManifestPath)
	}

	cmp, err := compareSemver(manifest.Version, lastSkillReleaseVersion)
	if err != nil {
		t.Fatalf("comparing %s's version %q against %q: %v",
			pluginManifestPath, manifest.Version, lastSkillReleaseVersion, err)
	}
	if cmp <= 0 {
		t.Errorf("%s version %q is not strictly greater than %q (the version that shipped the "+
			"skills this change rewrites) — claude plugin update diffs only plugin.json's version "+
			"field, never skill content, so the bundled-skill edits in this change would ship inert",
			pluginManifestPath, manifest.Version, lastSkillReleaseVersion)
	}
}

// compareSemver compares two plain "major.minor.patch" version strings
// (missing trailing components treated as 0) and returns -1/0/1 like
// strings.Compare. No pre-release/build-metadata support — this repo's
// plugin.json tags are plain semver, nothing fancier is needed.
func compareSemver(a, b string) (int, error) {
	pa, err := parseSemverInts(a)
	if err != nil {
		return 0, err
	}
	pb, err := parseSemverInts(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1, nil
			}
			return 1, nil
		}
	}
	return 0, nil
}

func parseSemverInts(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(strings.TrimSpace(v), ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, fmt.Errorf("version %q must have 1-3 dot-separated integer components", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return out, fmt.Errorf("version %q: component %q is not an integer: %w", v, p, err)
		}
		out[i] = n
	}
	return out, nil
}

// docsSweptForRemovedFoundingGrammar names every file milestone 7's
// criterion 2 scans: README.md and adr-sourced-constitution.md (read
// straight off disk — this package's directory is the repo root, and
// neither file lives under skills/, so SkillsFS does not carry them) plus
// all four skills/*/SKILL.md, read through the embedded SkillsFS so those
// checks assert on what actually ships in the binary.
func docsSweptForRemovedFoundingGrammar(t *testing.T) map[string]string {
	t.Helper()
	docs := make(map[string]string)

	for _, path := range []string{"README.md", "adr-sourced-constitution.md"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s off disk: %v", path, err)
		}
		docs[path] = string(b)
	}

	for _, skill := range []string{"adr-draft", "constitution-gov", "plan-gate", "constitution-init"} {
		path := "skills/" + skill + "/SKILL.md"
		b, err := SkillsFS.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s from the embedded SkillsFS: %v", path, err)
		}
		docs[path] = string(b)
	}

	return docs
}

// staleFoundingGrammarMarkers are substrings that only ever appear when a
// doc describes the pre-M4 multi-principle founding-file grammar — the
// exact grammar M4 deleted (parseFoundingFile, the principle type,
// gatherPrinciples, foundingBody, foundingLabel): one principle per
// "## <title>" heading, each optionally followed by its own attached
// "## Rules" block, one ADR seeded per principle. Mirrors the marker list
// TestConstitutionInitDropsStaleGrammarAndHandEdits already uses for
// constitution-init/SKILL.md, applied here to every doc in scope. None of
// the docs in scope need these markers to describe that grammar even
// historically — adr-sourced-constitution.md §7's account of what M2/M3
// shipped points at the current (M4-M6) description instead of restating
// the old mechanics, specifically so this stays a safe, unambiguous
// absence check.
var staleFoundingGrammarMarkers = []string{
	"one principle per",
	"attached `## Rules`",
	"attached ## Rules",
	"per principle",
}

// staleFoundingGrammarSeedsPerPrinciple catches the "one ADR per principle"
// idea by shape rather than exact phrase (init used to seed one ADR per
// principle; it now always seeds exactly one, total).
var staleFoundingGrammarSeedsPerPrinciple = regexp.MustCompile(`(?i)seeds? (one|a) (founding )?ADR per (each )?principle`)

// supersededHandEditInstructionMarkers are literal phrases, matched
// case-sensitively against whitespace-normalized doc text, that this repo's
// real history shows were once genuine instructions to hand-edit
// constitution.yml — not a general "sounds like an instruction" classifier,
// a plain regression guard against specific known-bad text returning:
//
//   - "edit `constitution.yml`'s `sourceTracking` block now" shipped in
//     skills/constitution-init/SKILL.md until issue #17's fix (commit
//     905471e) replaced it with a real interview question that states the
//     legal values instead of telling the model to go write YAML. That file
//     is separately guarded by TestConstitutionInitDropsStaleGrammarAndHandEdits;
//     this list extends the identical literal check across every doc in
//     scope, in case the phrase (or a close cousin of it) crept in
//     elsewhere.
//   - The next three markers are milestone 7 (this change)'s own near-miss:
//     an earlier draft of skills/adr-draft/SKILL.md and
//     skills/constitution-gov/SKILL.md told the agent to run
//     `constitution config schema` to learn the project's *category*
//     vocabulary, and explicitly told it not to read constitution.yml for
//     that. Verification against the built binary caught that `config
//     schema`'s `categories` entry carries no `values` array — categories
//     are per-project data with no closed enum — so that wording pointed an
//     agent at the one place the answer can never be, while forbidding the
//     one place it actually lives. These three phrases are that exact
//     draft's wording, guarded here so it cannot silently return.
//
// WHAT THIS TEST DOES AND DOES NOT GUARANTEE: this is a regression guard
// against these SPECIFIC known-bad phrasings reappearing verbatim (or
// near-verbatim) — it is NOT a general detector of "does this doc instruct
// a hand-edit of constitution.yml". A novel, differently-worded hand-edit
// instruction that matches none of these markers will NOT be caught. An
// earlier version of this test tried proximity-to-negation-word lexical
// detection instead; it was dropped because it both let injected
// instructions through (a trailing "Never" bullet elsewhere in the same doc
// was enough to whitelist an unrelated injected instruction inside the
// window) and flagged honest prose with no bearing on our own
// constitution.yml (the Spec-Kit comparison cell in
// adr-sourced-constitution.md, which describes a DIFFERENT tool's UX).
// Catching a genuinely novel hand-edit instruction still requires a human
// reading the diff — a green run of this test is evidence, not proof, that
// no doc instructs one.
var supersededHandEditInstructionMarkers = []string{
	"edit `constitution.yml`'s `sourceTracking` block now",
	"run `constitution config schema` to see it",
	"never hand-read `constitution.yml` for this",
	"`constitution config schema` for the current vocabulary — never a hand-edit",
}

// TestNoShippedDocTeachesTheRemovedFoundingGrammar is milestone 7's
// criterion 2: none of README.md, adr-sourced-constitution.md, or the four
// skills/*/SKILL.md files may still describe the removed multi-principle
// founding-file grammar (staleFoundingGrammarMarkers /
// staleFoundingGrammarSeedsPerPrinciple — a safe absence check, since no doc
// in scope needs those phrases even to describe the grammar historically:
// adr-sourced-constitution.md §7 points at the current description instead
// of restating the old mechanics), or reintroduce one of this repo's
// specific known-bad hand-edit instructions
// (supersededHandEditInstructionMarkers — see that var's doc comment for
// exactly what this test does and does not guarantee about hand-edit
// instructions in general).
func TestNoShippedDocTeachesTheRemovedFoundingGrammar(t *testing.T) {
	docs := docsSweptForRemovedFoundingGrammar(t)

	for path, doc := range docs {
		normalized := strings.Join(strings.Fields(doc), " ")

		for _, marker := range staleFoundingGrammarMarkers {
			if strings.Contains(normalized, marker) {
				t.Errorf("%s still contains %q, a marker of the removed multi-principle "+
					"founding-file grammar (one principle per heading, one attached ## Rules "+
					"block per principle, one ADR seeded per principle) — --founding-file now "+
					"takes a single MADR body and init seeds exactly one founding ADR (ADR-0001)",
					path, marker)
			}
		}
		if staleFoundingGrammarSeedsPerPrinciple.MatchString(normalized) {
			t.Errorf("%s still describes seeding one ADR per principle; init now seeds exactly "+
				"one founding ADR (ADR-0001) total", path)
		}

		for _, marker := range supersededHandEditInstructionMarkers {
			if strings.Contains(normalized, marker) {
				t.Errorf("%s contains %q, a known-bad instruction to hand-edit constitution.yml "+
					"(see supersededHandEditInstructionMarkers for why this exact phrase is "+
					"guarded) — reading constitution.yml is fine, but nothing should tell an "+
					"agent to write it directly", path, marker)
			}
		}
	}
}

// TestAdrDraftSkillWarnsAboutBodyFileRuleReplacement is issue #31:
// `adr edit --body-file` replaces the ENTIRE body, so any rule the
// replacement omits is deleted — silently. The ADR still validates,
// guard still reports clean, and regen simply drops the rule from the
// projection. The failure is silent at every layer, so the skill that
// drives the command has to carry the warning.
func TestAdrDraftSkillWarnsAboutBodyFileRuleReplacement(t *testing.T) {
	b, err := SkillsFS.ReadFile("skills/adr-draft/SKILL.md")
	if err != nil {
		t.Fatalf("reading the adr-draft skill from the embedded SkillsFS: %v", err)
	}
	doc := string(b)

	for _, want := range []string{"--body-file", "deleted", "To change rules deliberately, use"} {
		if !strings.Contains(doc, want) {
			t.Errorf("adr-draft SKILL.md never mentions %q; the --body-file rule-loss hazard is undocumented", want)
		}
	}
	if !strings.Contains(doc, "byte-identical") {
		t.Error("adr-draft SKILL.md gives no procedure for proving a prose-only edit left the Rules section untouched")
	}
}

// bundledSkillPaths is every SKILL.md this binary embeds and fans out
// on init/regen.
var bundledSkillPaths = []string{
	"skills/adr-draft/SKILL.md",
	"skills/constitution-gov/SKILL.md",
	"skills/constitution-init/SKILL.md",
	"skills/plan-gate/SKILL.md",
}

// TestSkillsDeclareTheMinimumCLIVersion is issue #32: the skills ship
// through two independent channels — the plugin catalog and this
// binary's //go:embed — with no handshake between them. A 0.2.1
// plugin copy drove a 0.3.0 binary and told the agent to hand-edit
// constitution.yml to do something the CLI by then supported directly,
// which the newer skill forbids outright. Every SKILL.md must state
// the version it is written for, from the single const, so the two
// can never disagree silently.
func TestSkillsDeclareTheMinimumCLIVersion(t *testing.T) {
	want := "`constitution` " + SkillsMinCLIVersion + " or newer"
	for _, path := range bundledSkillPaths {
		t.Run(path, func(t *testing.T) {
			b, err := SkillsFS.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s from the embedded SkillsFS: %v", path, err)
			}
			if !strings.Contains(string(b), want) {
				t.Errorf("%s does not declare its minimum CLI version (expected the literal %q); "+
					"bump the skills and SkillsMinCLIVersion together", path, want)
			}
		})
	}
}

// TestSkillsInstructAVersionCheck proves the declaration is an
// instruction the agent executes, not a note it reads past: the
// dangerous case is confidently following instructions that no longer
// match the binary being driven.
func TestSkillsInstructAVersionCheck(t *testing.T) {
	for _, path := range bundledSkillPaths {
		t.Run(path, func(t *testing.T) {
			b, err := SkillsFS.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s from the embedded SkillsFS: %v", path, err)
			}
			doc := string(b)
			if !strings.Contains(doc, "constitution --version") {
				t.Errorf("%s never tells the agent to run `constitution --version`", path)
			}
			if !strings.Contains(doc, "**stop**") {
				t.Errorf("%s does not tell the agent to stop on an older binary", path)
			}
		})
	}
}
