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

// foundingTitle is the fixed title of the single ADR init seeds from
// --founding-file. init only seeds on an empty log (the len(existing) > 0
// guard in seedFounding), and adr.NextID returns 1 there, so the founding
// ADR is always ADR-0001 deterministically — nothing needs to identify it
// beyond that, so its title needs no per-run configurability. This mirrors
// bootstrapSource: a fixed, reserved value rather than a new flag surface.
const foundingTitle = "Founding constitution"

// initCommand implements `constitution init` (plan §4). It scaffolds
// constitution/adr/, writes constitution.yml at the repo root, seeds a
// founding ADR (source: bootstrap only when source tracking is enabled) via
// the same internal write path as
// `adr new`, renders the projection, and writes the managed pointer blocks +
// fanned-out skills, all drift-protected via constitution/.state. A re-run is
// idempotent (byte-identical tree); interior/file drift requires --force or
// an interactive confirm. init is deliberately NOT consent-gated — the
// consent gate covers adr new/supersede/deprecate; bootstrap is different.
func initCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "scaffold a constitution: config, a founding ADR, pointer blocks, skills",
		Description: "Creates constitution/adr/ and constitution.yml (repo root), seeds a\n" +
			"single founding ADR (ADR-0001) from --founding-file, renders\n" +
			"constitution/constitution.md, writes managed pointer blocks into the chosen\n" +
			"agent-instruction targets (CLAUDE.md, AGENTS.md) and fans the Layer-2 skills\n" +
			"out to .claude/, .agents/, .cursor/. Re-running is a no-op on a clean tree;\n" +
			"a target hand-edited since init last wrote it requires --force.\n\n" +
			"--founding-file takes exactly what `adr new --body-file` takes: a MADR\n" +
			"body (the '## ' sections), or - for stdin. It must carry every mandatory\n" +
			"section (Context and Problem Statement, Considered Options, Decision\n" +
			"Outcome) and MAY carry a '## Rules' section in the same\n" +
			"'### <category>' / '#### <slug>' grammar `adr new --rule` composes\n" +
			"(categories must be in the configured vocabulary), making the founding\n" +
			"ADR rule-bearing; omitting it seeds a catalog-only record. Any other,\n" +
			"non-mandatory section (e.g. a prose '## Deferred bets') is preserved\n" +
			"verbatim in the log but never rendered into constitution.md.",
		ArgsUsage: " ", // no positional args
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "founding-file", Usage: "path to a MADR body (the ## sections) to seed as the founding ADR (ADR-0001), or - for stdin"},
			&cli.StringSliceFlag{Name: "target", Usage: "agent-instruction target for the pointer block: claude|agents (repeatable; default: both)"},
			&cli.StringSliceFlag{Name: "skills-tree", Usage: "skills fan-out tree: claude|agents|cursor (repeatable; default: all three)"},
			&cli.StringSliceFlag{Name: "category", Usage: "category vocabulary entry (repeatable; default: the starter list)"},
			&cli.StringFlag{Name: "consent", Value: config.ConsentStrict, Usage: "consent policy written to config: strict|off"},
			&cli.StringFlag{Name: "source-tracking", Value: config.SourceTrackingNone,
				Usage: "sourceTracking.type written to config: none|generic|github-issue|jira"},
			&cli.StringFlag{Name: "source-pattern",
				Usage: "sourceTracking.pattern written to config (only meaningful with --source-tracking other than \"none\")"},
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

	// --- 2b. pre-flight --founding-file: read, validate and compose it
	// entirely in memory BEFORE any byte is written, so a bad founding
	// file leaves the working tree untouched (issues #22/#30). The
	// vocabulary it is checked against is the in-memory config; the
	// on-disk reload below is still what everything downstream uses. ---
	adrDir := filepath.Join(cwd, "constitution", "adr")
	founding, err := prepareFounding(cmd, cfg, adrDir)
	if err != nil {
		return err
	}

	// --- 2c. persist a freshly authored config (a re-run leaves the
	// existing one untouched, so the tree stays byte-identical), then
	// reload it so the config used downstream is the validated, on-disk
	// verbatim ---
	if isNew {
		configPath := filepath.Join(cwd, "constitution.yml")
		if err := persistConfig(configPath, cfg); err != nil {
			return err
		}
		if cfg, err = config.Load(configPath); err != nil {
			return err
		}
	}

	// --- 3. seed the founding ADR (only on a fresh log, so re-runs
	// stay idempotent) ---
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		return err
	}
	if err := writeFounding(founding, stdout); err != nil {
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

	sourceTrackingType := cmd.String("source-tracking")
	sourcePattern := cmd.String("source-pattern")
	if sourcePattern != "" && sourceTrackingType == config.SourceTrackingNone {
		return nil, false, &exitError{
			err: fmt.Errorf(
				"init: --source-pattern is meaningless when --source-tracking is %q (or unset): pass a non-%q --source-tracking value",
				config.SourceTrackingNone, config.SourceTrackingNone),
			code: 2,
		}
	}

	cfg = &config.Config{
		SchemaVersion:     config.SchemaVersion,
		AgentInstructions: config.AgentInstructions{Targets: targets},
		Consent:           config.Consent{Policy: consent},
		SourceTracking:    config.SourceTracking{Type: sourceTrackingType, Pattern: sourcePattern},
		// A fresh constitution starts in draft (v0.2 proposal A3): founding
		// is a staged process, and sealing is always an explicit,
		// human-approved act — init never sells finality.
		Phase:      config.PhaseDraft,
		Categories: categories,
		Skills:     config.Skills{Trees: trees},
	}
	// Reuse Config's own validator for sourceTracking.type instead of
	// hand-listing the four legal values a second time here (issue #17 was
	// exactly that kind of duplicated vocabulary going stale).
	if err := cfg.Validate(); err != nil {
		return nil, false, &exitError{err: fmt.Errorf("init: %w", err), code: 2}
	}
	return cfg, true, nil
}

// noticeIgnoredReinitFlags prints a one-line stderr notice when init is run
// against a repo that already has a constitution.yml and the user passed
// config-shaping flags: the on-disk config wins on a re-run (plan §4), so
// those flags are ignored. It is purely informational and changes nothing —
// it just makes the "existing config wins" contract honest rather than silent.
func noticeIgnoredReinitFlags(cmd *cli.Command, stderr io.Writer) {
	var ignored []string
	for _, f := range []string{"target", "skills-tree", "category", "consent", "source-tracking", "source-pattern"} {
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

// foundingWrite is a fully validated founding ADR, composed in memory
// and ready to write: by the time one exists, everything init can
// refuse about --founding-file has already been refused.
type foundingWrite struct {
	dest    string
	content []byte
}

// prepareFounding reads and fully validates --founding-file WITHOUT
// writing anything (issues #22/#30). It returns nil when there is
// nothing to seed: no --founding-file, or a log that already has ADRs
// (bootstrap seeding is one-time, so a re-init of an established repo
// refreshes integration, not the log).
//
// The body is validated through adr.ValidateBody — the exact function
// `adr new --body-file` uses — and the composed record is then run
// through the same full parse the read path uses, exactly like runNew
// does, so valid-on-write and valid-on-read can never drift apart.
func prepareFounding(cmd *cli.Command, cfg *config.Config, adrDir string) (*foundingWrite, error) {
	foundingFile := cmd.String("founding-file")
	if foundingFile == "" {
		return nil, nil
	}

	// The pre-flight runs before anything creates constitution/adr/,
	// so a missing directory is simply an empty log.
	existing, err := adr.ParseDir(adrDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, nil
	}

	body, err := readBody(foundingFile, os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("init: reading --founding-file: %w", err)
	}
	const label = "--founding-file"
	if err := adr.ValidateBody(body, label); err != nil {
		return nil, &exitError{err: fmt.Errorf("init: %w", err), code: 2}
	}

	// Founding ADRs carry the reserved `bootstrap` source ONLY when
	// source tracking is enabled: under `type: none` no ADR may carry a
	// `source` field at all (plan §2.8, erratum #8).
	foundingSource := ""
	if cfg.SourceTracking.Type != config.SourceTrackingNone {
		foundingSource = bootstrapSource
	}

	_, id, err := adr.NextID(adrDir)
	if err != nil {
		return nil, err
	}
	content := adr.Compose(adr.NewADR{
		ID:     id,
		Title:  foundingTitle,
		Date:   today(),
		Source: foundingSource,
		Body:   string(body),
	})

	parsed, err := adr.ParseBytesUnnamed(content, label)
	if err != nil {
		return nil, &exitError{err: fmt.Errorf("init: invalid %s: %w", label, err), code: 2}
	}

	// Seed rules may only use categories already in the configured
	// vocabulary: there is no --new-category at init — the vocabulary
	// was chosen seconds earlier, so an unknown category is a typo.
	vocab := make(map[string]bool, len(cfg.Categories))
	for _, c := range cfg.Categories {
		vocab[c] = true
	}
	for _, r := range parsed.Rules {
		if !vocab[r.Category] {
			return nil, &exitError{err: fmt.Errorf(
				"init: %s: rule category %q is not in the configured vocabulary %v",
				label, r.Category, cfg.Categories), code: 2}
		}
	}

	return &foundingWrite{
		dest:    filepath.Join(adrDir, adr.Filename(id, foundingTitle)),
		content: content,
	}, nil
}

// writeFounding commits a prepared founding ADR to disk. A nil fw is
// the "nothing to seed" case and writes nothing.
func writeFounding(fw *foundingWrite, stdout io.Writer) error {
	if fw == nil {
		return nil
	}
	if err := atomicwrite.WriteFile(fw.dest, fw.content, 0o644); err != nil {
		return fmt.Errorf("init: writing --founding-file: %w", err)
	}
	_, err := fmt.Fprintf(stdout, "created %s\n", fw.dest)
	return err
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
