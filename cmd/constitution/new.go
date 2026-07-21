package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
)

// newCommand implements `constitution adr new` (plan §2.3, §4): compose a
// new accepted ADR from a supplied MADR body, allocate its id, and write it
// atomically, then regen. Standing rules arrive either as repeated --rule
// flags (composed into a "## Rules" section) or verbatim in the body-file;
// --supersedes-rule/--removes-rule retire rules of earlier ADRs, and
// --new-category grows the vocabulary for a rule that needs it. All
// validation — including a full fold preflight of the log with this ADR
// appended — happens before the consent gate, and the gate before any
// write, so a refusal or a bad input leaves the log untouched.
func newCommand() *cli.Command {
	return &cli.Command{
		Name:      "new",
		Usage:     "create a new accepted ADR from a MADR body",
		ArgsUsage: " ", // no positional args
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Required: true, Usage: "ADR title"},
			&cli.StringFlag{Name: "source", Usage: "source ref (required when sourceTracking.type != none)"},
			&cli.StringFlag{Name: "body-file", Required: true, Usage: "path to the MADR body (the ## sections), or - for stdin; may carry its own ## Rules section"},
			&cli.StringSliceFlag{Name: "rule", Usage: "a standing rule as \"<category>/<slug>: <text>\" (repeatable); composed into a ## Rules section. Mutually exclusive with a body-file that carries its own ## Rules"},
			&cli.StringSliceFlag{Name: "supersedes-rule", Usage: "retire a prior rule this ADR replaces: \"ADR-NNNN/<category>/<slug>\" (repeatable)"},
			&cli.StringSliceFlag{Name: "removes-rule", Usage: "retire a prior rule nothing replaces: \"ADR-NNNN/<category>/<slug>\" (repeatable)"},
			&cli.StringSliceFlag{Name: "new-category", Usage: "introduce a category into the vocabulary if a rule uses an unknown one (repeatable)"},
			approveFlag(),
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runNew(cmd)
		},
	}
}

func runNew(cmd *cli.Command) error {
	m, err := openRepo(cmd)
	if err != nil {
		return err
	}

	title := cmd.String("title")
	source := cmd.String("source")

	// --- validate everything up front (no writes, no prompt yet) ---
	body, err := readBody(cmd.String("body-file"), m.stdin)
	if err != nil {
		return err
	}
	ruleFlags := cmd.StringSlice("rule")
	if len(ruleFlags) > 0 && hasRulesSection(body) {
		return fmt.Errorf(
			"both --rule and a --body-file that already contains a \"## Rules\" section were supplied; provide the rules exactly once (drop --rule, or remove the section from the body)")
	}
	body, err = composeRulesSection(body, ruleFlags)
	if err != nil {
		return err
	}
	label := bodyLabel(ruleFlags)
	if err := adr.ValidateBody(body, label); err != nil {
		return err
	}
	if err := validateSource(m.cfg.SourceTracking, source); err != nil {
		return err
	}
	supersedesRules, err := ruleRefFlags("supersedes-rule", cmd.StringSlice("supersedes-rule"))
	if err != nil {
		return err
	}
	removesRules, err := ruleRefFlags("removes-rule", cmd.StringSlice("removes-rule"))
	if err != nil {
		return err
	}

	_, id, err := adr.NextID(m.adrDir)
	if err != nil {
		return err
	}
	file := adr.Compose(adr.NewADR{
		ID:              id,
		Title:           title,
		Date:            today(),
		Source:          source,
		SupersedesRules: supersedesRules,
		RemovesRules:    removesRules,
		Body:            string(body),
	})

	// Full parse of the composed record: the exact bytes about to be written
	// must satisfy the read path (rules grammar, frontmatter refs) before
	// anything else happens.
	parsed, err := adr.ParseBytesUnnamed(file, label)
	if err != nil {
		return err
	}

	// Vocabulary: every rule category must be configured or explicitly new.
	toAppend, err := m.resolveNewCategories(parsed, cmd.StringSlice("new-category"))
	if err != nil {
		return err
	}

	// Fold preflight: the log with this ADR appended must render cleanly.
	existing, err := m.parseLog()
	if err != nil {
		return err
	}
	if err := m.preflightFold(append(existing, *parsed), toAppend); err != nil {
		return err
	}

	// --- consent gate: last check before the first byte is written ---
	if err := m.gate().confirm("create " + id); err != nil {
		return err
	}

	// --- writes ---
	if err := m.appendCategories(toAppend); err != nil {
		return err
	}
	if err := os.MkdirAll(m.adrDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(m.adrDir, adr.Filename(id, title))
	if err := atomicwrite.WriteFile(dest, file, 0o644); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(m.stdout, "created %s\n", dest); err != nil {
		return err
	}

	return m.regen()
}
