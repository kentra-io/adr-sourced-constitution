package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/config"
	"github.com/kentra-io/adr-sourced-constitution/internal/manifest"
	"github.com/kentra-io/adr-sourced-constitution/internal/render"
)

// sealCommand implements `constitution seal` (v0.2 proposal D1/A3): the
// explicit, human-approved act that ends the founding draft — it writes the
// first manifest baseline and flips constitution.yml to `phase: sealed`,
// after which the log is append-only forever (adr edit/rm refuse; full
// guard semantics begin). Before the consent gate it re-renders and prints
// the pre-seal review checklist (every long-rule warning, one final time),
// because this is the last moment a fix is a cheap `adr edit` rather than a
// supersede.
//
// Write order is manifest -> phase flip -> regen, which makes every crash
// window convergent: a manifest without the flip is removed by any
// draft-phase regen and seal simply re-runs; a flip without the final regen
// is healed by it (the manifest was already written one step earlier).
func sealCommand() *cli.Command {
	return &cli.Command{
		Name:      "seal",
		Usage:     "end the founding draft: baseline the manifest, make the log append-only",
		ArgsUsage: " ", // no positional args
		Flags:     []cli.Flag{approveFlag()},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runSeal(cmd)
		},
	}
}

func runSeal(cmd *cli.Command) error {
	m, err := openRepo(cmd)
	if err != nil {
		return err
	}
	if m.cfg.Phase != config.PhaseDraft {
		return fmt.Errorf("seal: already sealed — the log has been append-only since the manifest baseline was written")
	}

	// Validate the whole log the way regen would BEFORE any prompt: a log
	// that does not render must be fixed (cheaply, it's still draft), not
	// sealed.
	adrs, err := m.parseLog()
	if err != nil {
		return err
	}
	if _, _, err := render.Render(m.cfg, adrs); err != nil {
		return err
	}

	// Pre-seal review checklist: every rule-length warning, one final time
	// (v0.2 proposal §5 — after this, warnings only re-fire on the ADR a
	// write touches). Resurrection/fold warnings print via the closing
	// regen below, so the seal invocation surfaces those too.
	if err := warnLongRules(m.stderr, adrs); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(m.stderr,
		"seal: this is the last cheap edit — sealing makes the log append-only forever (adr edit/rm stop working; changes become supersede/deprecate records)"); err != nil {
		return err
	}

	// --- consent gate ---
	if err := m.gate().confirm("seal the constitution"); err != nil {
		return err
	}

	// --- ordered writes: manifest baseline, phase flip, closing regen ---
	if err := manifest.Write(m.adrDir, adrs); err != nil {
		return err
	}
	crashCheckpoint("after-manifest-baseline")
	m.cfg.Phase = config.PhaseSealed
	if err := persistConfig(filepath.Join(m.root, "constitution.yml"), m.cfg); err != nil {
		return err
	}
	crashCheckpoint("after-phase-flip")

	if _, err := fmt.Fprintf(m.stdout, "sealed: %d ADR(s) baselined in %s; the log is now append-only\n",
		len(adrs), filepath.Join("constitution", "adr", manifest.FileName)); err != nil {
		return err
	}
	return m.regen()
}
