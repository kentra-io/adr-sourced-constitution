package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/patch"
)

// supersedeCommand implements `constitution supersede <id>` (spec §5.2,
// plan §4): write a new ADR that supersedes <id>, then flip <id>'s status to
// superseded with a derived superseded-by back-link. The two writes are
// ordered — new ADR first, then the status patch — so a crash between them
// leaves a log that still parses and that regen converges (plan §3, "the log
// is truth"). No transaction; recovery is a regen.
func supersedeCommand() *cli.Command {
	return &cli.Command{
		Name:      "supersede",
		Usage:     "supersede an accepted ADR with a new one",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Required: true, Usage: "title of the superseding ADR"},
			&cli.StringFlag{Name: "category", Usage: "category of the new ADR (defaults to the superseded ADR's category)"},
			&cli.StringFlag{Name: "source", Usage: "source ref (required when sourceTracking.type != none)"},
			&cli.StringFlag{Name: "body-file", Required: true, Usage: "path to the new ADR's MADR body, or - for stdin"},
			&cli.StringFlag{Name: "rule", Usage: "standing-rule text for the superseding ADR; composed as a ## Rule section (makes it rule-bearing). Mutually exclusive with a body-file that carries its own ## Rule section"},
			&cli.BoolFlag{Name: "new-category", Usage: "introduce --category into the vocabulary if it is unknown"},
			approveFlag(),
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runSupersede(cmd)
		},
	}
}

func runSupersede(cmd *cli.Command) error {
	m, err := openRepo(cmd)
	if err != nil {
		return err
	}

	oldID := cmd.Args().Get(0)
	if oldID == "" {
		return fmt.Errorf("supersede: missing <id> argument")
	}

	oldPath, found, err := adr.FindByID(m.adrDir, oldID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("supersede: no ADR with id %q found in %s", oldID, m.adrDir)
	}
	oldADR, err := adr.Parse(oldPath)
	if err != nil {
		return err
	}
	if oldADR.Status != adr.StatusAccepted {
		return fmt.Errorf("supersede: %s is not accepted (status: %s); only an accepted ADR can be superseded", oldID, oldADR.Status)
	}

	title := cmd.String("title")
	source := cmd.String("source")
	category := cmd.String("category")
	if category == "" {
		category = oldADR.Category // default to the superseded ADR's category
	}

	// --- validate up front ---
	body, err := readBody(cmd.String("body-file"), m.stdin)
	if err != nil {
		return err
	}
	body, err = applyRuleFlag(cmd, body)
	if err != nil {
		return err
	}
	if err := adr.ValidateBody(body, "--body-file"); err != nil {
		return err
	}
	if err := validateSource(m.cfg.SourceTracking, source); err != nil {
		return err
	}
	isNewCategory, err := m.checkCategory(category, cmd.Bool("new-category"))
	if err != nil {
		return err
	}

	_, newID, err := adr.NextID(m.adrDir)
	if err != nil {
		return err
	}
	newFile := adr.Compose(adr.NewADR{
		ID:         newID,
		Title:      title,
		Category:   category,
		Date:       today(),
		Source:     source,
		Supersedes: oldID,
		Body:       string(body),
	})

	// Read the old ADR's raw bytes and compute its status patch now, so the
	// only work left after the consent gate is the sequence of writes.
	oldRaw, err := os.ReadFile(oldPath)
	if err != nil {
		return err
	}
	patchedOld, err := patch.Supersede(oldRaw, newID)
	if err != nil {
		return fmt.Errorf("supersede: patch %s: %w", oldPath, err)
	}

	// --- consent gate ---
	if err := m.gate().confirm(fmt.Sprintf("supersede %s with %s", oldID, newID)); err != nil {
		return err
	}

	// --- ordered writes: new ADR, then old-ADR status patch, then regen ---
	if isNewCategory {
		if err := m.appendCategory(category); err != nil {
			return err
		}
	}
	newDest := filepath.Join(m.adrDir, adr.Filename(newID, title))
	if err := atomicwrite.WriteFile(newDest, newFile, 0o644); err != nil {
		return err
	}
	crashCheckpoint("after-new-adr")

	if err := atomicwrite.WriteFile(oldPath, patchedOld, 0o644); err != nil {
		return err
	}
	crashCheckpoint("after-old-patch")

	if _, err := fmt.Fprintf(m.stdout, "created %s\nsuperseded %s\n", newDest, oldID); err != nil {
		return err
	}
	return m.regen()
}
