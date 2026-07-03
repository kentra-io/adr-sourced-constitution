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
var starterCategories = []string{"architecture", "code-style", "process", "testing", "security", "data"}

// initCommand implements `constitution init` (plan §4). It scaffolds
// constitution/adr/, writes constitution.yml at the repo root, seeds
// founding ADRs (source: bootstrap) via the same internal write path as
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
			"founding ADR per --principle and per section of --founding-file, renders\n" +
			"constitution/constitution.md, writes managed pointer blocks into the chosen\n" +
			"agent-instruction targets (CLAUDE.md, AGENTS.md) and fans the Layer-2 skills\n" +
			"out to .claude/, .agents/, .cursor/. Re-running is a no-op on a clean tree;\n" +
			"a target hand-edited since init last wrote it requires --force.\n\n" +
			"--founding-file format: a Markdown file with one principle per '## ' heading;\n" +
			"the heading becomes the ADR title and the text beneath it (until the next\n" +
			"heading) becomes the rule statement (its Decision Outcome).",
		ArgsUsage: " ", // no positional args
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "principle", Usage: "a founding principle (repeatable); the text is both the ADR title and its rule statement"},
			&cli.StringFlag{Name: "founding-file", Usage: "path to a Markdown file of founding principles (one per '## ' heading)"},
			&cli.StringFlag{Name: "founding-category", Usage: "category to file founding ADRs under (default: the first configured category)"},
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

	_, err = fmt.Fprintln(stdout, "init: constitution ready in constitution/")
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
		Categories:        categories,
		Skills:            config.Skills{Trees: trees},
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

// principle is one founding rule: Title becomes the ADR heading, Statement
// its Decision Outcome (the rendered rule text).
type principle struct {
	Title     string
	Statement string
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

	category, err := resolveFoundingCategory(cmd.String("founding-category"), cfg)
	if err != nil {
		return err
	}

	base, _, err := adr.NextID(adrDir)
	if err != nil {
		return err
	}

	// Pre-flight: compose EVERY founding ADR and run it through the same
	// read-path validator regen uses, before writing any of them. A single
	// invalid principle (e.g. an empty '## ' heading yielding a blank title)
	// must fail here — exit 2, nothing written — rather than land a poisoned
	// `title: ""` record on disk that makes every later regen fail. Ids are
	// allocated deterministically from `base` (the log is empty at this point,
	// guaranteed by the len(existing)>0 guard above).
	type seed struct {
		dest    string
		content []byte
	}
	seeds := make([]seed, len(principles))
	for i, p := range principles {
		id := adr.FormatID(base + i)
		content := adr.Compose(adr.NewADR{
			ID:       id,
			Title:    p.Title,
			Category: category,
			Date:     today(),
			Source:   bootstrapSource,
			Body:     foundingBody(p.Statement),
		})
		if _, err := adr.ParseBytesUnnamed(content, foundingLabel(p.Title)); err != nil {
			return &exitError{err: fmt.Errorf("init: invalid %s: %w", foundingLabel(p.Title), err), code: 2}
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

// gatherPrinciples collects founding principles from --principle (each string
// is both title and statement) and --founding-file (one per '## ' heading), in
// that order.
func gatherPrinciples(cmd *cli.Command) ([]principle, error) {
	var ps []principle
	for _, p := range cmd.StringSlice("principle") {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, &exitError{err: fmt.Errorf("init: --principle must not be empty"), code: 2}
		}
		ps = append(ps, principle{Title: p, Statement: p})
	}
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

// parseFoundingFile splits a founding-principles Markdown file into one
// principle per '## ' heading: the heading is the title, the text until the
// next heading is the statement (falling back to the title if empty).
func parseFoundingFile(content string) ([]principle, error) {
	var ps []principle
	var title string
	var body []string
	inSection := false

	flush := func() {
		if !inSection {
			return
		}
		statement := strings.TrimSpace(strings.Join(body, "\n"))
		if statement == "" {
			statement = title
		}
		ps = append(ps, principle{Title: title, Statement: statement})
	}

	for _, line := range strings.Split(content, "\n") {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			title = strings.TrimSpace(h)
			body = nil
			inSection = true
			continue
		}
		if inSection {
			body = append(body, line)
		}
	}
	flush()

	if len(ps) == 0 {
		return nil, fmt.Errorf("no principles found (expected one or more '## ' headings)")
	}
	return ps, nil
}

// foundingBody builds a minimal, MADR-valid body (all three mandatory
// sections) around a principle's statement, which becomes the Decision
// Outcome the projection renders.
func foundingBody(statement string) string {
	return "## Context and Problem Statement\n\n" +
		"Established at project bootstrap by `constitution init`.\n\n" +
		"## Considered Options\n\n" +
		"- Adopt this founding principle\n" +
		"- Leave the convention implicit\n\n" +
		"## Decision Outcome\n\n" +
		statement + "\n"
}

// resolveFoundingCategory picks the category founding ADRs are filed under:
// the --founding-category flag (validated against the vocabulary) or the
// first configured category.
func resolveFoundingCategory(flagVal string, cfg *config.Config) (string, error) {
	if flagVal != "" {
		for _, c := range cfg.Categories {
			if c == flagVal {
				return flagVal, nil
			}
		}
		return "", &exitError{
			err:  fmt.Errorf("init: --founding-category %q is not in the configured vocabulary %s", flagVal, formatCategories(cfg.Categories)),
			code: 2,
		}
	}
	// cfg.Categories is guaranteed non-empty (config validation).
	return cfg.Categories[0], nil
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
