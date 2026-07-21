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
// atomically, then regen. All validation happens before the consent gate,
// and the gate before any write, so a refusal or a bad input leaves the log
// untouched.
//
// v0.2 interim (Rules model, M1 Task 3): the body-file may carry its own
// "## Rules" section to make the ADR rule-bearing. The rule flags
// (--rule/--supersedes-rule/--removes-rule/--new-category) and the
// vocabulary-growth preflight land with the full CLI rework (M1 Task 5).
func newCommand() *cli.Command {
	return &cli.Command{
		Name:      "new",
		Usage:     "create a new accepted ADR from a MADR body",
		ArgsUsage: " ", // no positional args
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Required: true, Usage: "ADR title"},
			&cli.StringFlag{Name: "source", Usage: "source ref (required when sourceTracking.type != none)"},
			&cli.StringFlag{Name: "body-file", Required: true, Usage: "path to the MADR body (the ## sections, optionally including ## Rules), or - for stdin"},
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
	if err := adr.ValidateBody(body, "--body-file"); err != nil {
		return err
	}
	if err := validateSource(m.cfg.SourceTracking, source); err != nil {
		return err
	}

	_, id, err := adr.NextID(m.adrDir)
	if err != nil {
		return err
	}
	file := adr.Compose(adr.NewADR{
		ID:     id,
		Title:  title,
		Date:   today(),
		Source: source,
		Body:   string(body),
	})

	// --- consent gate: last check before the first byte is written ---
	if err := m.gate().confirm("create " + id); err != nil {
		return err
	}

	// --- writes ---
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
