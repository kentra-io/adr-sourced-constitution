package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/patch"
)

// renumberCommand implements `constitution adr renumber <old> <new>` (plan
// §2.6): the id-collision escape hatch. Safe by construction — a colliding
// ADR is by definition not yet merged, so nothing references it — this is
// the ONE permitted id edit. It refuses if <new> is already taken or if any
// other ADR references <old>, then renames the file and rewrites its id
// line via the patch package, and regens.
func renumberCommand() *cli.Command {
	return &cli.Command{
		Name:      "renumber",
		Usage:     "reassign an ADR's id (id-collision escape hatch)",
		ArgsUsage: "<old-id> <new-id>",
		Flags:     []cli.Flag{approveFlag()},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runRenumber(cmd)
		},
	}
}

func runRenumber(cmd *cli.Command) error {
	m, err := openRepo(cmd)
	if err != nil {
		return err
	}

	oldID := cmd.Args().Get(0)
	newID := cmd.Args().Get(1)
	if oldID == "" || newID == "" {
		return fmt.Errorf("renumber: usage: constitution adr renumber <old-id> <new-id>")
	}
	if !adr.ValidID(oldID) {
		return fmt.Errorf("renumber: %q is not a valid ADR id (want ADR-NNNN)", oldID)
	}
	if !adr.ValidID(newID) {
		return fmt.Errorf("renumber: %q is not a valid ADR id (want ADR-NNNN)", newID)
	}
	if oldID == newID {
		return fmt.Errorf("renumber: <old-id> and <new-id> are the same (%s)", oldID)
	}

	oldPath, found, err := adr.FindByID(m.adrDir, oldID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("renumber: no ADR with id %q found in %s", oldID, m.adrDir)
	}
	if _, taken, err := adr.FindByID(m.adrDir, newID); err != nil {
		return err
	} else if taken {
		return fmt.Errorf("renumber: id %q is already taken", newID)
	}

	// Refuse if any *other* ADR references <old> — renumber is only safe for
	// an unreferenced (not-yet-merged) ADR (plan §2.6).
	adrs, err := adr.ParseDir(m.adrDir)
	if err != nil {
		return err
	}
	for _, a := range adrs {
		if a.ID == oldID {
			continue
		}
		if a.Supersedes == oldID || a.SupersededBy == oldID {
			return fmt.Errorf("renumber: %s is referenced by %s (supersedes/superseded-by); cannot renumber a referenced ADR", oldID, a.ID)
		}
	}

	raw, err := os.ReadFile(oldPath)
	if err != nil {
		return err
	}
	patched, err := patch.SetID(raw, newID)
	if err != nil {
		return fmt.Errorf("renumber: patch %s: %w", oldPath, err)
	}

	// New filename keeps the old slug, only the id prefix changes.
	base := filepath.Base(oldPath)
	slug := strings.TrimSuffix(strings.TrimPrefix(base, oldID+"-"), ".md")
	newPath := filepath.Join(m.adrDir, newID+"-"+slug+".md")

	if err := m.gate().confirm(fmt.Sprintf("renumber %s to %s", oldID, newID)); err != nil {
		return err
	}

	// Write the renamed file first, then remove the old one. A crash between
	// the two leaves two well-formed files with distinct ids (no duplicate);
	// regen converges and the operator can re-run.
	if err := atomicwrite.WriteFile(newPath, patched, 0o644); err != nil {
		return err
	}
	crashCheckpoint("after-new-adr")
	if err := os.Remove(oldPath); err != nil {
		return err
	}
	crashCheckpoint("after-old-removed")

	if _, err := fmt.Fprintf(m.stdout, "renumbered %s -> %s\n", oldID, newID); err != nil {
		return err
	}
	return m.regen()
}
