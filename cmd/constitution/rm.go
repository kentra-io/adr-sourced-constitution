package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
	"github.com/kentra-io/adr-sourced-constitution/internal/patch"
)

// rmCommand implements `constitution adr rm <id>` (v0.2 proposal §3):
// draft-phase-only deletion of an ADR from the working set. Later ids keep
// their numbers — a gap is honest history of the draft, and `renumber`
// remains the explicit escape hatch. Removal refuses when any other ADR
// still references the target (a rule-retirement ref, or a superseding
// record built on its slot) — with ONE heal case: removing an ADR that
// itself supersedes another is the draft-phase "undo a supersede", and
// restores its target to accepted in the same operation.
func rmCommand() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "delete an ADR from a draft-phase log",
		ArgsUsage: "<id>",
		Description: "Deletes the record outright — draft phase only; a sealed log is\n" +
			"append-only (supersede/deprecate instead). Refused while other ADRs\n" +
			"reference the target via supersedes-rules/removes-rules (edit those\n" +
			"first) or while a later ADR supersedes it (rm that one first).\n" +
			"Removing an ADR that supersedes another restores the superseded ADR\n" +
			"to accepted (undo of the supersede). Ids are not renumbered.",
		Flags: []cli.Flag{approveFlag()},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runRm(cmd)
		},
	}
}

func runRm(cmd *cli.Command) error {
	m, err := openRepo(cmd)
	if err != nil {
		return err
	}
	if m.cfg.Phase != config.PhaseDraft {
		return fmt.Errorf(
			"rm: phase is sealed — the log is append-only; supersede or deprecate the decision instead")
	}

	id := cmd.Args().Get(0)
	if id == "" {
		return fmt.Errorf("rm: usage: constitution adr rm <id>")
	}
	targetPath, found, err := adr.FindByID(m.adrDir, id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("rm: no ADR with id %q found in %s", id, m.adrDir)
	}

	all, err := m.parseLog()
	if err != nil {
		return err
	}

	// Reference scan over the rest of the log: rule refs and a superseding
	// successor block the removal (each has a better fix); a superseded
	// PREDECESSOR is the heal case — restore it.
	var healIDs []string
	for i := range all {
		a := &all[i]
		if a.ID == id {
			continue
		}
		if a.Supersedes == id {
			return fmt.Errorf(
				"rm: %s is superseded by %s, which builds on its slot; rm %s first",
				id, a.ID, a.ID)
		}
		if a.SupersededBy == id {
			healIDs = append(healIDs, a.ID)
		}
		var refs []string
		for _, r := range a.SupersedesRules {
			if r.ADRID == id {
				refs = append(refs, r.String())
			}
		}
		for _, r := range a.RemovesRules {
			if r.ADRID == id {
				refs = append(refs, r.String())
			}
		}
		if len(refs) > 0 {
			return fmt.Errorf(
				"rm: %s still retires rules of %s (%s); edit %s's ref list first",
				a.ID, id, strings.Join(refs, ", "), a.ID)
		}
	}

	// Compute the heal patches now (nothing written yet), and preflight the
	// post-rm log: a restored predecessor re-activates its own rules AND its
	// own retirement directives (A7), which can collide with retirements
	// other ADRs declared while it was superseded — that double-retire must
	// refuse here, before the consent gate.
	type heal struct {
		path    string
		patched []byte
		parsed  *adr.ADR
	}
	heals := make(map[string]heal, len(healIDs))
	for _, hid := range healIDs {
		hPath, hFound, err := adr.FindByID(m.adrDir, hid)
		if err != nil {
			return err
		}
		if !hFound {
			return fmt.Errorf("rm: %s's superseded predecessor %s is missing from the log", id, hid)
		}
		raw, err := os.ReadFile(hPath)
		if err != nil {
			return err
		}
		patched, err := patch.Unsupersede(raw)
		if err != nil {
			return fmt.Errorf("rm: restoring %s: %w", hid, err)
		}
		parsed, err := adr.ParseBytesUnnamed(patched, hPath)
		if err != nil {
			return fmt.Errorf("rm: restoring %s: %w", hid, err)
		}
		heals[hid] = heal{path: hPath, patched: patched, parsed: parsed}
	}
	preflight := make([]adr.ADR, 0, len(all))
	for i := range all {
		if all[i].ID == id {
			continue
		}
		if h, ok := heals[all[i].ID]; ok {
			preflight = append(preflight, *h.parsed)
			continue
		}
		preflight = append(preflight, all[i])
	}
	if err := m.preflightFold(preflight, nil); err != nil {
		return err
	}

	// --- consent gate ---
	if err := m.gate().confirm("remove " + id); err != nil {
		return err
	}

	// --- ordered writes: heal restores first, then the delete. A crash
	// between the two leaves the predecessor accepted alongside the not-yet-
	// removed successor; re-running rm converges (Unsupersede no-ops on an
	// accepted file). ---
	for _, hid := range healIDs {
		h := heals[hid]
		if err := atomicwrite.WriteFile(h.path, h.patched, 0o644); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(m.stdout, "restored %s to accepted\n", hid); err != nil {
			return err
		}
	}
	crashCheckpoint("after-heal")
	if err := os.Remove(targetPath); err != nil {
		return err
	}
	crashCheckpoint("after-removed")

	if _, err := fmt.Fprintf(m.stdout, "removed %s\n", targetPath); err != nil {
		return err
	}
	return m.regen()
}
