package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
	"github.com/kentra-io/adr-sourced-constitution/internal/scaffold"
)

// starterCategories is the reference category vocabulary `init` proposes
// (plan §2.5) — a suggestion only, overridable with repeated --category.
var starterCategories = []string{"purpose", "architecture", "code-style", "testing", "process", "tooling", "security", "data"}

// initCommand implements `constitution init` (plan §4). It scaffolds
// constitution/adr/, writes constitution.yml at the repo root, seeds
// founding ADRs (source: bootstrap only when source tracking is enabled) via
// the same internal write path as
// `adr new`, renders the projection, and writes the managed pointer blocks +
// fanned-out skills, all drift-protected via constitution/.state. A re-run is
// idempotent (byte-identical tree); interior/file drift requires --force or
// an interactive confirm. init is deliberately NOT consent-gated — the
// consent gate covers adr new/supersede/deprecate; bootstrap is different.
func initCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "scaffold a constitution: config, founding ADRs, pointer blocks, skills",
		Description: "Creates constitution/adr/ and constitution.yml (repo root), seeds a\n" +
			"founding ADR per section of --founding-file, renders\n" +
			"constitution/constitution.md, writes managed pointer blocks into the chosen\n" +
			"agent-instruction targets (CLAUDE.md, AGENTS.md) and fans the Layer-2 skills\n" +
			"out to .claude/, .agents/, .cursor/. Re-running is a no-op on a clean tree;\n" +
			"a target hand-edited since init last wrote it requires --force.\n\n" +
			"--founding-file format: one principle per '## <title>' heading; the heading\n" +
			"becomes the ADR title and the text beneath it becomes its Decision Outcome.\n" +
			"A '## Rules' heading immediately following a principle carries that\n" +
			"principle's standing rules in the '### <category>' / '#### <slug>' grammar\n" +
			"(categories must be in the configured vocabulary); a principle with no\n" +
			"'## Rules' seeds a catalog-only record.",
		ArgsUsage: " ", // no positional args
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "founding-file", Usage: "path to a Markdown file of founding principles (one per '## ' heading)"},
			&cli.StringSliceFlag{Name: "target", Usage: "agent-instruction target for the pointer block: claude|agents (repeatable; default: both)"},
			&cli.StringSliceFlag{Name: "skills-tree", Usage: "skills fan-out tree: claude|agents|cursor (repeatable; default: all three)"},
			&cli.StringSliceFlag{Name: "category", Usage: "category vocabulary entry (repeatable; default: the starter list)"},
			&cli.StringFlag{Name: "consent", Value: config.ConsentStrict, Usage: "consent policy written to config: strict|off"},
			&cli.BoolFlag{Name: "force", Usage: "overwrite a managed target that drifted from what init last wrote"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runInit(cmd)
		},
	}
}

func runInit(cmd *cli.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	stdout := cmd.Root().Writer
	stderr := cmd.Root().ErrWriter

	// --- 1. resolve config: honor an existing one, else author it from flags
	// (in memory — not yet written, so pre-flight can refuse before any bytes
	// hit disk) ---
	cfg, isNew, err := buildOrLoadConfig(cmd, cwd, stderr)
	if err != nil {
		return err
	}

	// --- 2. pre-flight: refuse a structurally ambiguous target before any
	// writes, so a broken marker pair (exit 2) never leaves a half-scaffold ---
	if err := scaffold.PreflightBlocks(cwd, cfg); err != nil {
		var me *scaffold.MarkerError
		if errors.As(err, &me) {
			return &exitError{err: fmt.Errorf("init: %w", err), code: 2}
		}
		return err
	}

	// --- 2b. persist a freshly authored config (a re-run leaves the existing
	// one untouched, so the tree stays byte-identical), then reload it so the
	// config used downstream is the validated, on-disk verbatim ---
	if isNew {
		configPath := filepath.Join(cwd, "constitution.yml")
		if err := persistConfig(configPath, cfg); err != nil {
			return err
		}
		if cfg, err = config.Load(configPath); err != nil {
			return err
		}
	}

	// --- 3. seed founding ADRs (only on a fresh log, so re-runs stay idempotent) ---
	adrDir := filepath.Join(cwd, "constitution", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		return err
	}
	if err := seedFounding(cmd, cfg, adrDir, stdout); err != nil {
		return err
	}

	// --- 4. render the projection (constitution.md + manifest) ---
	if err := regenCore(cwd, cfg, stdout, stderr); err != nil {
		return err
	}

	// --- 5. write pointer blocks + fan out skills (confirm/force on drift) ---
	opts := scaffold.Options{
		Root:   cwd,
		Cfg:    cfg,
		Mode:   scaffold.ModeInit,
		Force:  cmd.Bool("force"),
		Stdout: stdout,
		Stderr: stderr,
	}
	if !cmd.Bool("force") && isTerminal(os.Stdin) {
		opts.Confirm = interactiveConfirm(os.Stdin, stderr)
	}
	if err := scaffold.Refresh(opts); err != nil {
		var me *scaffold.MarkerError
		if errors.As(err, &me) {
			return &exitError{err: fmt.Errorf("init: %w", err), code: 2}
		}
		return err
	}

	if _, err = fmt.Fprintln(stdout, "init: constitution ready in constitution/"); err != nil {
		return err
	}
	if cfg.Phase == config.PhaseDraft {
		_, err = fmt.Fprintln(stdout,
			"init: phase is draft — ADRs stay editable (adr edit / adr rm) until `constitution seal` makes the log append-only")
	}
	return err
}

// buildOrLoadConfig loads an existing constitution.yml (its target/category/
// skills selections win over flags on a re-run, plan §4) or authors a new one
// in memory from flags and defaults. isNew reports whether the caller must
// persist it; the config is not written here, so a pre-flight failure leaves
// the tree untouched.
func buildOrLoadConfig(cmd *cli.Command, root string, stderr io.Writer) (cfg *config.Config, isNew bool, err error) {
	configPath := filepath.Join(root, "constitution.yml")
	if _, statErr := os.Stat(configPath); statErr == nil {
		cfg, err = config.Load(configPath)
		if err == nil {
			noticeIgnoredReinitFlags(cmd, stderr)
		}
		return cfg, false, err
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, false, statErr
	}

	targets, err := normalizeChoices("target", cmd.StringSlice("target"),
		[]string{config.TargetClaude, config.TargetAgents},
		map[string]bool{config.TargetClaude: true, config.TargetAgents: true})
	if err != nil {
		return nil, false, err
	}
	trees, err := normalizeChoices("skills-tree", cmd.StringSlice("skills-tree"),
		[]string{config.SkillTreeClaude, config.SkillTreeAgents, config.SkillTreeCursor},
		map[string]bool{config.SkillTreeClaude: true, config.SkillTreeAgents: true, config.SkillTreeCursor: true})
	if err != nil {
		return nil, false, err
	}

	categories, err := resolveCategories(cmd.StringSlice("category"))
	if err != nil {
		return nil, false, err
	}

	consent := cmd.String("consent")
	if consent != config.ConsentStrict && consent != config.ConsentOff {
		return nil, false, &exitError{
			err:  fmt.Errorf("init: --consent must be %q or %q (got %q)", config.ConsentStrict, config.ConsentOff, consent),
			code: 2,
		}
	}

	return &config.Config{
		SchemaVersion:     config.SchemaVersion,
		AgentInstructions: config.AgentInstructions{Targets: targets},
		Consent:           config.Consent{Policy: consent},
		SourceTracking:    config.SourceTracking{Type: config.SourceTrackingNone},
		// A fresh constitution starts in draft (v0.2 proposal A3): founding
		// is a staged process, and sealing is always an explicit,
		// human-approved act — init never sells finality.
		Phase:      config.PhaseDraft,
		Categories: categories,
		Skills:     config.Skills{Trees: trees},
	}, true, nil
}

// noticeIgnoredReinitFlags prints a one-line stderr notice when init is run
// against a repo that already has a constitution.yml and the user passed
// config-shaping flags: the on-disk config wins on a re-run (plan §4), so
// those flags are ignored. It is purely informational and changes nothing —
// it just makes the "existing config wins" contract honest rather than silent.
func noticeIgnoredReinitFlags(cmd *cli.Command, stderr io.Writer) {
	var ignored []string
	for _, f := range []string{"target", "skills-tree", "category", "consent"} {
		if cmd.IsSet(f) {
			ignored = append(ignored, "--"+f)
		}
	}
	if len(ignored) == 0 || stderr == nil {
		return
	}
	_, _ = fmt.Fprintf(stderr,
		"init: existing constitution.yml wins; ignoring %s\n", strings.Join(ignored, ", "))
}

// resolveCategories applies the starter vocabulary when none were given, and
// otherwise rejects empty/duplicate entries up front (matching config
// validation) so a freshly authored config always reloads cleanly.
func resolveCategories(given []string) ([]string, error) {
	if len(given) == 0 {
		return append([]string(nil), starterCategories...), nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(given))
	for _, c := range given {
		if c == "" {
			return nil, &exitError{err: fmt.Errorf("init: --category entries must not be empty"), code: 2}
		}
		if seen[c] {
			return nil, &exitError{err: fmt.Errorf("init: --category %q given more than once", c), code: 2}
		}
		seen[c] = true
		out = append(out, c)
	}
	return out, nil
}

// normalizeChoices validates a repeatable choice flag against an allowed set,
// applying a default when none were given. Order is preserved and duplicates
// collapsed. An unknown value is a usage error (exit 2).
func normalizeChoices(flag string, given, def []string, allowed map[string]bool) ([]string, error) {
	if len(given) == 0 {
		return append([]string(nil), def...), nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(given))
	for _, v := range given {
		if !allowed[v] {
			return nil, &exitError{err: fmt.Errorf("init: --%s: unknown value %q", flag, v), code: 2}
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out, nil
}

// principle is one founding decision: Title becomes the ADR heading,
// Statement its Decision Outcome, and Rules the verbatim content of its
// optional "## Rules" section (the h3/h4 grammar). HasRules distinguishes
// a catalog-only principle from one whose Rules heading was present but
// empty — the latter must compose an (invalid) empty section so the
// grammar rejects it instead of silently seeding a record-only ADR.
type principle struct {
	Title     string
	Statement string
	Rules     string
	HasRules  bool
}

// seedFounding writes one ADR per founding principle — but only on a fresh
// log (no ADRs yet), so a re-run never double-seeds and the tree stays
// byte-identical. Reuses the internal write path (id allocation, atomic
// write); the manifest is refreshed by the regenCore that follows.
func seedFounding(cmd *cli.Command, cfg *config.Config, adrDir string, stdout io.Writer) error {
	principles, err := gatherPrinciples(cmd)
	if err != nil {
		return err
	}
	if len(principles) == 0 {
		return nil
	}

	existing, err := adr.ParseDir(adrDir)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		// The log already has ADRs: bootstrap seeding is a one-time action, so
		// this is a no-op (re-run idempotency). Not an error — a re-init of an
		// established repo simply refreshes integration, not the log.
		return nil
	}

	// Founding ADRs carry the reserved `bootstrap` source ONLY when source
	// tracking is enabled: under `type: none` no ADR may carry a `source`
	// field at all (plan §2.8; the "founding ADRs use bootstrap" sentence
	// applies only when type != none — erratum #8). When enabled, `bootstrap`
	// is a reserved value that bypasses the configured pattern check.
	foundingSource := ""
	if cfg.SourceTracking.Type != config.SourceTrackingNone {
		foundingSource = bootstrapSource
	}

	base, _, err := adr.NextID(adrDir)
	if err != nil {
		return err
	}

	// Pre-flight: compose EVERY founding ADR and run it through the same
	// read-path validator regen uses, before writing any of them. A single
	// invalid principle (e.g. an empty '## ' heading yielding a blank title,
	// or a malformed Rules section) must fail here — exit 2, nothing written —
	// rather than land a poisoned record on disk that makes every later regen
	// fail. Ids are allocated deterministically from `base` (the log is empty
	// at this point, guaranteed by the len(existing)>0 guard above).
	//
	// Seed rules may only use categories already in the configured vocabulary
	// (--category / the starter list): there is no --new-category at init —
	// the vocabulary was chosen seconds earlier, so an unknown category is a
	// typo, not growth.
	vocab := make(map[string]bool, len(cfg.Categories))
	for _, c := range cfg.Categories {
		vocab[c] = true
	}
	type seed struct {
		dest    string
		content []byte
	}
	seeds := make([]seed, len(principles))
	for i, p := range principles {
		id := adr.FormatID(base + i)
		content := adr.Compose(adr.NewADR{
			ID:     id,
			Title:  p.Title,
			Date:   today(),
			Source: foundingSource,
			Body:   foundingBody(p),
		})
		parsed, err := adr.ParseBytesUnnamed(content, foundingLabel(p.Title))
		if err != nil {
			return &exitError{err: fmt.Errorf("init: invalid %s: %w", foundingLabel(p.Title), err), code: 2}
		}
		for _, r := range parsed.Rules {
			if !vocab[r.Category] {
				return &exitError{err: fmt.Errorf(
					"init: %s: rule category %q is not in the configured vocabulary %v",
					foundingLabel(p.Title), r.Category, cfg.Categories), code: 2}
			}
		}
		seeds[i] = seed{dest: filepath.Join(adrDir, adr.Filename(id, p.Title)), content: content}
	}

	for i := range seeds {
		if err := atomicwrite.WriteFile(seeds[i].dest, seeds[i].content, 0o644); err != nil {
			return fmt.Errorf("init: writing %s: %w", foundingLabel(principles[i].Title), err)
		}
		if _, err := fmt.Fprintf(stdout, "created %s\n", seeds[i].dest); err != nil {
			return err
		}
	}
	return nil
}

// foundingLabel names a founding principle for error/label text: quoted by
// its title when it has one, or a bare description when the title is blank
// (the case pre-flight validation is meant to catch).
func foundingLabel(title string) string {
	if t := strings.TrimSpace(title); t != "" {
		return fmt.Sprintf("founding principle %q", t)
	}
	return "founding principle (empty title)"
}

// gatherPrinciples collects founding principles from --founding-file (one
// per '## ' heading).
func gatherPrinciples(cmd *cli.Command) ([]principle, error) {
	var ps []principle
	if f := cmd.String("founding-file"); f != "" {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("init: reading --founding-file: %w", err)
		}
		parsed, err := parseFoundingFile(string(data))
		if err != nil {
			return nil, &exitError{err: fmt.Errorf("init: --founding-file %s: %w", f, err), code: 2}
		}
		ps = append(ps, parsed...)
	}
	return ps, nil
}

// parseFoundingFile splits a founding-principles Markdown file on its '## '
// headings: any heading starts a new principle (the heading is the title,
// the text until the next heading is the statement, falling back to the
// title if empty) — except "## Rules", which attaches its content verbatim
// as the standing rules of the PRECEDING principle. A "## Rules" that is
// the first heading, or that follows another "## Rules", has no principle
// to attach to and is an error. The founding file is not an ADR body, so
// this deliberately walks raw '## ' lines rather than ExtractSections
// (whose section map would collapse the duplicate "Rules" headings two
// rule-bearing principles produce).
func parseFoundingFile(content string) ([]principle, error) {
	var ps []principle
	var body []string
	cur := -1        // index into ps of the principle being accumulated
	inRules := false // accumulating a "## Rules" section's content

	// flush assigns the accumulated lines to the current principle's
	// Statement or Rules, per the section being closed.
	flush := func() {
		if cur < 0 {
			body = nil
			return
		}
		text := strings.TrimSpace(strings.Join(body, "\n"))
		if inRules {
			ps[cur].Rules = text
			ps[cur].HasRules = true
		} else {
			ps[cur].Statement = text
		}
		body = nil
	}

	for _, line := range strings.Split(content, "\n") {
		h, ok := strings.CutPrefix(line, "## ")
		if !ok {
			if cur >= 0 {
				body = append(body, line)
			}
			continue
		}
		heading := strings.TrimSpace(h)
		if heading == adr.RulesSection {
			if cur < 0 {
				return nil, fmt.Errorf(
					"\"## Rules\" cannot be the first heading: a Rules section carries the standing rules of the principle heading preceding it")
			}
			if inRules {
				return nil, fmt.Errorf(
					"\"## Rules\" directly after another \"## Rules\": each principle carries at most one Rules section")
			}
			flush()
			inRules = true
			continue
		}
		flush()
		inRules = false
		ps = append(ps, principle{Title: heading})
		cur = len(ps) - 1
	}
	flush()

	if len(ps) == 0 {
		return nil, fmt.Errorf("no principles found (expected one or more '## ' headings)")
	}
	for i := range ps {
		if ps[i].Statement == "" {
			ps[i].Statement = ps[i].Title
		}
	}
	return ps, nil
}

// foundingBody builds a minimal, MADR-valid body (all three mandatory
// sections) around a principle's statement (its Decision Outcome), plus its
// "## Rules" section verbatim when the founding file carried one.
func foundingBody(p principle) string {
	body := "## Context and Problem Statement\n\n" +
		"Established at project bootstrap by `constitution init`.\n\n" +
		"## Considered Options\n\n" +
		"- Adopt this founding principle\n" +
		"- Leave the convention implicit\n\n" +
		"## Decision Outcome\n\n" +
		p.Statement + "\n"
	if p.HasRules {
		body += "\n## " + adr.RulesSection + "\n\n" + p.Rules + "\n"
	}
	return body
}

// interactiveConfirm builds a y/N prompt reader for init's drift confirm on a
// terminal. It reuses the same "y"/"yes" acceptance as the consent gate.
func interactiveConfirm(in io.Reader, out io.Writer) func(string) (bool, error) {
	reader := bufio.NewReader(in)
	return func(prompt string) (bool, error) {
		if _, err := fmt.Fprintf(out, "%s [y/N] ", prompt); err != nil {
			return false, err
		}
		line, _ := reader.ReadString('\n')
		s := strings.ToLower(strings.TrimSpace(line))
		return s == "y" || s == "yes", nil
	}
}
