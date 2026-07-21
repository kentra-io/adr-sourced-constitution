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
// superseded with a derived superseded-by back-link. The new ADR carries the
// same rule surface as `adr new` (--rule/--supersedes-rule/--removes-rule/
// --new-category): superseding a rule-bearing ADR is exactly where its rules
// get retired or replaced. The two writes are ordered — new ADR first, then
// the status patch — so a crash between them leaves a log that still parses
// and that regen converges (plan §3, "the log is truth"). No transaction;
// recovery is a regen.
func supersedeCommand() *cli.Command {
	return &cli.Command{
		Name:      "supersede",
		Usage:     "supersede an accepted ADR with a new one",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Required: true, Usage: "title of the superseding ADR"},
			&cli.StringFlag{Name: "source", Usage: "source ref (required when sourceTracking.type != none)"},
			&cli.StringFlag{Name: "body-file", Required: true, Usage: "path to the new ADR's MADR body (the ## sections), or - for stdin; may carry its own ## Rules section"},
			&cli.StringSliceFlag{Name: "rule", Usage: "a standing rule as \"<category>/<slug>: <text>\" (repeatable); composed into a ## Rules section. Mutually exclusive with a body-file that carries its own ## Rules"},
			&cli.StringSliceFlag{Name: "supersedes-rule", Usage: "retire a prior rule this ADR replaces: \"ADR-NNNN/<category>/<slug>\" (repeatable)"},
			&cli.StringSliceFlag{Name: "removes-rule", Usage: "retire a prior rule nothing replaces: \"ADR-NNNN/<category>/<slug>\" (repeatable)"},
			&cli.StringSliceFlag{Name: "new-category", Usage: "introduce a category into the vocabulary if a rule uses an unknown one (repeatable)"},
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

	// --- validate up front ---
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

	_, newID, err := adr.NextID(m.adrDir)
	if err != nil {
		return err
	}
	newFile := adr.Compose(adr.NewADR{
		ID:              newID,
		Title:           title,
		Date:            today(),
		Source:          source,
		Supersedes:      oldID,
		SupersedesRules: supersedesRules,
		RemovesRules:    removesRules,
		Body:            string(body),
	})

	// Full parse of the composed record before anything else happens.
	newParsed, err := adr.ParseBytesUnnamed(newFile, label)
	if err != nil {
		return err
	}

	// Vocabulary: every rule category must be configured or explicitly new.
	toAppend, err := m.resolveNewCategories(newParsed, cmd.StringSlice("new-category"))
	if err != nil {
		return err
	}

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

	// Fold preflight on the log as it will exist AFTER both writes: the old
	// ADR with its status flipped (its own retirements stop applying — A7)
	// plus the new one appended.
	patchedParsed, err := adr.ParseBytesUnnamed(patchedOld, oldPath)
	if err != nil {
		return fmt.Errorf("supersede: patch %s: %w", oldPath, err)
	}
	existing, err := m.parseLog()
	if err != nil {
		return err
	}
	preflight := make([]adr.ADR, 0, len(existing)+1)
	for i := range existing {
		if existing[i].ID == oldADR.ID {
			preflight = append(preflight, *patchedParsed)
			continue
		}
		preflight = append(preflight, existing[i])
	}
	preflight = append(preflight, *newParsed)
	if err := m.preflightFold(preflight, toAppend); err != nil {
		return err
	}

	// --- consent gate ---
	if err := m.gate().confirm(fmt.Sprintf("supersede %s with %s", oldID, newID)); err != nil {
		return err
	}

	// --- ordered writes: config growth, new ADR, old-ADR status patch, regen ---
	if err := m.appendCategories(toAppend); err != nil {
		return err
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
