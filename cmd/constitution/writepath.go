package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/urfave/cli/v3"
	yaml "go.yaml.in/yaml/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
	"github.com/kentra-io/adr-sourced-constitution/internal/render"
)

// bootstrapSource is the reserved source value founding ADRs use (plan
// §2.8); it satisfies a non-"none" sourceTracking config without matching
// the configured issue/ticket pattern.
const bootstrapSource = "bootstrap"

// mutContext is the shared state a mutating verb needs: the repo root, its
// loaded config, the ADR directory, and the io streams. openRepo builds it
// once at the top of each verb.
type mutContext struct {
	root    string
	adrDir  string
	cfg     *config.Config
	stdout  io.Writer
	stderr  io.Writer
	stdin   io.Reader
	approve bool
}

// openRepo loads the repo rooted at the process working directory: the
// config must exist and be valid, or the verb refuses before touching the
// log.
func openRepo(cmd *cli.Command) (*mutContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(filepath.Join(cwd, "constitution.yml"))
	if err != nil {
		return nil, err
	}
	return &mutContext{
		root:    cwd,
		adrDir:  filepath.Join(cwd, "constitution", "adr"),
		cfg:     cfg,
		stdout:  cmd.Root().Writer,
		stderr:  cmd.Root().ErrWriter,
		stdin:   os.Stdin,
		approve: cmd.Bool("approve"),
	}, nil
}

// gate builds the consent gate for this repo/command, detecting whether
// stdin is an interactive terminal.
func (m *mutContext) gate() consentGate {
	return consentGate{
		policy:  m.cfg.Consent.Policy,
		approve: m.approve,
		isTTY:   isTerminal(os.Stdin),
		in:      m.stdin,
		out:     m.stderr,
	}
}

// regen re-derives constitution.md and the manifest after a mutation.
func (m *mutContext) regen() error {
	return regenAt(m.root, m.stdout, m.stderr)
}

// ruleSurfaceFlags is the flag set every rule-bearing mutation shares
// (new/supersede/edit): rules in, retirements out, vocabulary growth.
func ruleSurfaceFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringSliceFlag{Name: "rule", Usage: "a standing rule as \"<category>/<slug>: <text>\" (repeatable); composed into a ## Rules section. Mutually exclusive with a body-file that carries its own ## Rules"},
		&cli.StringSliceFlag{Name: "supersedes-rule", Usage: "retire a prior rule this ADR replaces: \"ADR-NNNN/<category>/<slug>\" (repeatable)"},
		&cli.StringSliceFlag{Name: "removes-rule", Usage: "retire a prior rule nothing replaces: \"ADR-NNNN/<category>/<slug>\" (repeatable)"},
		&cli.StringSliceFlag{Name: "new-category", Usage: "introduce a category into the vocabulary if a rule uses an unknown one (repeatable)"},
	}
}

// adrInput is the validated compose-input `adr new` and `supersede` share:
// the body with any --rule flags composed in, the error label naming where
// that body came from, and the up-front-validated retirement ref lists.
type adrInput struct {
	body            []byte
	label           string
	source          string
	supersedesRules []string
	removesRules    []string
}

// composeADRInput is the shared validate-and-compose prologue of the
// body-taking mutating verbs: read the body (file or stdin), reject the
// ambiguous "--rule AND a body-file with its own ## Rules", compose the
// flag rules in, validate MADR shape and source-ref contract, and validate
// the retirement ref flags' format. No log access, no writes — everything
// here fails before an id is allocated or a prompt is shown.
func (m *mutContext) composeADRInput(cmd *cli.Command) (*adrInput, error) {
	body, err := readBody(cmd.String("body-file"), m.stdin)
	if err != nil {
		return nil, err
	}
	ruleFlags := cmd.StringSlice("rule")
	if len(ruleFlags) > 0 && hasRulesSection(body) {
		return nil, fmt.Errorf(
			"both --rule and a --body-file that already contains a \"## Rules\" section were supplied; provide the rules exactly once (drop --rule, or remove the section from the body)")
	}
	body, err = composeRulesSection(body, ruleFlags)
	if err != nil {
		return nil, err
	}
	label := bodyLabel(ruleFlags)
	if err := adr.ValidateBody(body, label); err != nil {
		return nil, err
	}
	source := cmd.String("source")
	if err := validateSource(m.cfg.SourceTracking, source); err != nil {
		return nil, err
	}
	supersedesRules, err := ruleRefFlags("supersedes-rule", cmd.StringSlice("supersedes-rule"))
	if err != nil {
		return nil, err
	}
	removesRules, err := ruleRefFlags("removes-rule", cmd.StringSlice("removes-rule"))
	if err != nil {
		return nil, err
	}
	return &adrInput{
		body:            body,
		label:           label,
		source:          source,
		supersedesRules: supersedesRules,
		removesRules:    removesRules,
	}, nil
}

// readBody reads a MADR body from a file path, or from stdin when path is
// "-" (plan §2.3).
func readBody(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

// hasRulesSection reports whether a raw MADR body already carries a
// "## Rules" section (the replacement for the pre-v0.2 HasRuleSection).
// Used to reject the ambiguous "--rule AND a body-file with its own Rules"
// combination before any composition happens.
func hasRulesSection(body []byte) bool {
	sections, _ := adr.ExtractSections(body)
	_, ok := sections[adr.RulesSection]
	return ok
}

// composeRulesSection turns repeated --rule values ("<cat>/<slug>: text")
// into a "## Rules" section appended to body. Flags are grouped by category
// in first-appearance order, and within a category rules keep flag order —
// so non-consecutive repeats of a category ("a/x", "b/y", "a/z") compose a
// single "### a" section rather than a re-opened one the grammar would
// reject. Returns body unchanged when no flags. Kebab-case, duplicate-slug,
// and vocabulary validation all happen downstream (ValidateBody / the fold
// preflight) — only the flag's "<category>/<slug>: <text>" shape is checked
// here.
func composeRulesSection(body []byte, ruleFlags []string) ([]byte, error) {
	if len(ruleFlags) == 0 {
		return body, nil
	}
	type flagRule struct{ slug, text string }
	var order []string
	grouped := map[string][]flagRule{}
	for _, f := range ruleFlags {
		head, text, ok := strings.Cut(f, ":")
		parts := strings.Split(strings.TrimSpace(head), "/")
		if !ok || len(parts) != 2 || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("--rule %q: must be \"<category>/<slug>: <text>\"", f)
		}
		cat, slug := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if _, seen := grouped[cat]; !seen {
			order = append(order, cat)
		}
		grouped[cat] = append(grouped[cat], flagRule{slug: slug, text: strings.TrimSpace(text)})
	}
	var b strings.Builder
	b.WriteString("\n\n## " + adr.RulesSection + "\n")
	for _, cat := range order {
		b.WriteString("\n### " + cat + "\n")
		for _, r := range grouped[cat] {
			b.WriteString("\n#### " + r.slug + "\n" + r.text + "\n")
		}
	}
	base := strings.TrimRight(string(body), "\n")
	return []byte(base + b.String()), nil
}

// bodyLabel names the error source for the composed body in validation and
// parse errors: the body-file alone, or the --rule-composed combination when
// rule flags contributed — so a grammar error in a --rule value is not
// misattributed to the body-file the user wrote.
func bodyLabel(ruleFlags []string) string {
	if len(ruleFlags) > 0 {
		return "--body-file (with --rule composed \"## Rules\")"
	}
	return "--body-file"
}

// ruleRefFlags validates each repeated retirement-ref flag value
// ("ADR-NNNN/<category>/<slug>") up front, so a malformed ref fails with
// the flag's name rather than surfacing later from the composed file's
// frontmatter parse. Semantic checks (dangling/forward/self/double-retire)
// are the fold preflight's job. Returns the raw strings for Compose.
func ruleRefFlags(flagName string, values []string) ([]string, error) {
	for _, v := range values {
		if _, err := adr.ParseRuleRef(v); err != nil {
			return nil, fmt.Errorf("--%s: %w", flagName, err)
		}
	}
	return values, nil
}

// resolveNewCategories checks every rule category of the parsed ADR against
// the configured vocabulary (plan §2.5): an unknown category is a hard
// error unless it was explicitly introduced with --new-category. It does
// NOT persist — it returns the categories to append so the caller can defer
// the config write until after the consent gate, keeping the
// "refuse ⇒ nothing written" invariant. A --new-category no rule of this
// ADR uses is an error (vocabulary growth stays coupled to the rule that
// needs it — and a typo'd category surfaces instead of silently landing in
// the config); one that names an already-configured category a rule uses is
// a forgiving no-op.
func (m *mutContext) resolveNewCategories(parsed *adr.ADR, newCategories []string) (toAppend []string, err error) {
	newCats := make(map[string]bool, len(newCategories))
	for _, c := range newCategories {
		if c == "" {
			return nil, fmt.Errorf("--new-category entries must not be empty")
		}
		if newCats[c] {
			return nil, fmt.Errorf("--new-category %q given more than once", c)
		}
		newCats[c] = true
	}
	existing := make(map[string]bool, len(m.cfg.Categories))
	for _, c := range m.cfg.Categories {
		existing[c] = true
	}
	used := map[string]bool{}
	appended := map[string]bool{}
	for _, r := range parsed.Rules {
		if newCats[r.Category] {
			used[r.Category] = true
		}
		if existing[r.Category] {
			continue
		}
		if !newCats[r.Category] {
			return nil, fmt.Errorf(
				"rule category %q is not in the configured vocabulary %v; pass --new-category %s to introduce it",
				r.Category, m.cfg.Categories, r.Category)
		}
		if !appended[r.Category] {
			appended[r.Category] = true
			toAppend = append(toAppend, r.Category)
		}
	}
	for _, c := range newCategories {
		if !used[c] {
			return nil, fmt.Errorf(
				"--new-category %q is not used by any rule of this ADR; drop the flag or file a rule under it", c)
		}
	}
	return toAppend, nil
}

// parseLog reads the full ADR log for the fold preflight. A repo whose
// adr/ directory does not exist yet simply has an empty log (`adr new`
// creates the directory only after the consent gate).
func (m *mutContext) parseLog() ([]adr.ADR, error) {
	adrs, err := adr.ParseDir(m.adrDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return adrs, nil
}

// preflightFold renders the log as it would exist after the mutation —
// with the vocabulary grown by toAppend — so dangling/forward/self/
// double-retire refs are caught BEFORE the consent gate: a refusal or bad
// input never leaves an invalid log (proposal §2). Warnings (rule
// resurrections) are deliberately suppressed here: the post-write regen
// renders the same projection and prints them once.
func (m *mutContext) preflightFold(adrs []adr.ADR, toAppend []string) error {
	preCfg := *m.cfg
	preCfg.Categories = append(append([]string(nil), m.cfg.Categories...), toAppend...)
	_, _, err := render.Render(&preCfg, adrs)
	return err
}

// appendCategories appends the vocabulary growth --new-category authorized
// and atomically rewrites constitution.yml. Called only after the consent
// gate has passed, so a refused mutation never touches the config (plan
// §2.5 — vocabulary growth is still an ordinary ADR write, no meta-record).
func (m *mutContext) appendCategories(cats []string) error {
	if len(cats) == 0 {
		return nil
	}
	m.cfg.Categories = append(m.cfg.Categories, cats...)
	return persistConfig(filepath.Join(m.root, "constitution.yml"), m.cfg)
}

// validateSource enforces the source-ref contract (plan §2.8) against the
// project's sourceTracking config: forbidden when type is none, required
// (and pattern-checked) otherwise. The reserved bootstrap source always
// passes when a source is expected.
func validateSource(st config.SourceTracking, source string) error {
	if st.Type == config.SourceTrackingNone {
		if source != "" {
			return fmt.Errorf("--source is not allowed when sourceTracking.type is %q", config.SourceTrackingNone)
		}
		return nil
	}
	if source == "" {
		return fmt.Errorf("--source is required when sourceTracking.type is %q", st.Type)
	}
	if source == bootstrapSource {
		return nil
	}

	pattern := st.Pattern
	if pattern == "" {
		switch st.Type {
		case config.SourceTrackingGitHubIssue:
			pattern = `#\d+`
		case config.SourceTrackingJira:
			pattern = `[A-Z]+-\d+`
		case config.SourceTrackingGeneric:
			return nil // no default shape; any non-empty source is accepted
		}
	}
	re, err := regexp.Compile(config.WrapSourcePattern(pattern))
	if err != nil {
		return fmt.Errorf("sourceTracking.pattern %q is not a valid regexp: %w", pattern, err)
	}
	if !re.MatchString(source) {
		return fmt.Errorf("--source %q does not match the required pattern %q", source, pattern)
	}
	return nil
}

// persistConfig atomically rewrites constitution.yml from cfg. Used by
// init's config authoring and by appendCategories' vocabulary growth. It
// re-serializes the struct (comments in a hand-authored or hand-tuned
// config are not preserved) — an acceptable tradeoff since the config is
// CLI-managed.
func persistConfig(path string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return atomicwrite.WriteFile(path, data, 0o644)
}

// isTerminal reports whether f is an interactive character device. Used to
// pick between prompting and refusing under the strict consent policy.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
