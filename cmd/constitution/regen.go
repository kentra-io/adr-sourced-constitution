package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
	"github.com/kentra-io/adr-sourced-constitution/internal/manifest"
	"github.com/kentra-io/adr-sourced-constitution/internal/render"
	"github.com/kentra-io/adr-sourced-constitution/internal/scaffold"
)

// lineWarningThreshold is the ~200-line constitution.md adherence
// guideline (adr-sourced-constitution.md §13.1, implementation-plan.md
// §2.1): regen warns, it does not block.
const lineWarningThreshold = 200

// ruleLineWarningThreshold is the per-Rule length guideline (plan §2.12): a
// standing rule should read as a 1–3 line normative statement. regen warns
// (never blocks) when a rule-bearing active ADR's Rule section exceeds this.
const ruleLineWarningThreshold = 5

func regenCommand() *cli.Command {
	return &cli.Command{
		Name:  "regen",
		Usage: "regenerate constitution/constitution.md from the ADR log",
		Description: "Reads constitution.yml and every ADR under constitution/adr/, resolves\n" +
			"the active set (status: accepted), groups it by category, and\n" +
			"deterministically renders constitution/constitution.md. Never edits\n" +
			"the ADR log; constitution.md is always a faithful projection of it.",
		Action: func(_ context.Context, cmd *cli.Command) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			return regenAt(cwd, cmd.Root().Writer, cmd.Root().ErrWriter)
		},
	}
}

// regenAt implements `constitution regen` (spec §6, implementation-plan
// §4) rooted at `root` rather than the process cwd, so the mutating verbs
// can regenerate the repo they just wrote to without a chdir. It renders the
// projection + manifest (regenCore), then refreshes the managed pointer
// blocks and fanned-out skills in warn-don't-block mode: drift in a user
// file is warned about and left alone, never a hard failure — a mutating
// verb's auto-regen must not be blockable by the state of an unrelated
// CLAUDE.md (plan §4).
func regenAt(root string, stdout, stderr io.Writer) error {
	cfg, err := config.Load(filepath.Join(root, "constitution.yml"))
	if err != nil {
		return err
	}
	if err := regenCore(root, cfg, stdout, stderr); err != nil {
		return err
	}
	return scaffold.Refresh(scaffold.Options{
		Root:   root,
		Cfg:    cfg,
		Mode:   scaffold.ModeWarn,
		Stdout: stdout,
		Stderr: stderr,
	})
}

// regenCore is the pure-projection half of regen: read all ADRs -> active
// set -> group -> render -> atomically write constitution.md, then rewrite
// the manifest. constitution.md and the manifest are pure projections of the
// log: a crash between the two writes leaves the log untouched, and the next
// regen re-derives both. It does NOT touch managed blocks or skills — that
// is the caller's concern (regenAt refreshes them in warn mode; init in
// confirm/force mode), so the two integration policies stay separate.
func regenCore(root string, cfg *config.Config, stdout, stderr io.Writer) error {
	adrDir := filepath.Join(root, "constitution", "adr")
	adrs, err := adr.ParseDir(adrDir)
	if err != nil {
		return err
	}

	out, err := render.Render(cfg, adrs)
	if err != nil {
		return err
	}

	outPath := filepath.Join(root, "constitution", "constitution.md")
	if err := atomicwrite.WriteFile(outPath, out, 0o644); err != nil {
		return err
	}

	crashCheckpoint("after-projection")

	if err := manifest.Write(adrDir, adrs); err != nil {
		return err
	}

	if lines := bytes.Count(out, []byte("\n")); lines > lineWarningThreshold {
		if _, err := fmt.Fprintf(stderr, "warning: constitution.md is %d lines, exceeding the ~200-line adherence guideline (adr-sourced-constitution.md §13.1)\n", lines); err != nil {
			return err
		}
	}

	if err := warnLongRules(stderr, adrs); err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "wrote %s\n", outPath)
	return err
}

// warnLongRules warns (stderr, never blocks) for each rule-bearing active ADR
// whose Rule section runs longer than ruleLineWarningThreshold lines (plan
// §2.12) — a standing rule should be a terse 1–3 line statement. Only the
// ADRs that actually project (accepted + rule-bearing) are checked; a frozen
// Rule on a superseded record is not the author's live concern.
func warnLongRules(stderr io.Writer, adrs []adr.ADR) error {
	for i := range adrs {
		a := &adrs[i]
		if a.Status != adr.StatusAccepted || !a.IsRuleBearing() {
			continue
		}
		if n := strings.Count(a.Rule(), "\n") + 1; n > ruleLineWarningThreshold {
			if _, err := fmt.Fprintf(stderr,
				"warning: %s has a %d-line Rule section, exceeding the %d-line guideline; keep standing rules to a terse statement (plan §2.12)\n",
				a.ID, n, ruleLineWarningThreshold); err != nil {
				return err
			}
		}
	}
	return nil
}
