package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/patch"
)

// deprecateCommand implements `constitution deprecate <id>` (spec §5.2,
// plan §4): retire an accepted ADR with no replacement by flipping its
// status to deprecated (a status-only line patch), then regen.
func deprecateCommand() *cli.Command {
	return &cli.Command{
		Name:      "deprecate",
		Usage:     "deprecate an accepted ADR (retire a rule with no replacement)",
		ArgsUsage: "<id>",
		Flags:     []cli.Flag{approveFlag()},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runDeprecate(cmd)
		},
	}
}

func runDeprecate(cmd *cli.Command) error {
	m, err := openRepo(cmd)
	if err != nil {
		return err
	}

	id := cmd.Args().Get(0)
	if id == "" {
		return fmt.Errorf("deprecate: missing <id> argument")
	}

	path, found, err := adr.FindByID(m.adrDir, id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("deprecate: no ADR with id %q found in %s", id, m.adrDir)
	}
	target, err := adr.Parse(path)
	if err != nil {
		return err
	}
	if target.Status != adr.StatusAccepted {
		return fmt.Errorf("deprecate: %s is not accepted (status: %s); only an accepted ADR can be deprecated", id, target.Status)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	patched, err := patch.SetStatus(raw, string(adr.StatusDeprecated))
	if err != nil {
		return fmt.Errorf("deprecate: patch %s: %w", path, err)
	}

	if err := m.gate().confirm("deprecate " + id); err != nil {
		return err
	}

	if err := atomicwrite.WriteFile(path, patched, 0o644); err != nil {
		return err
	}
	crashCheckpoint("after-old-patch")

	if _, err := fmt.Fprintf(m.stdout, "deprecated %s\n", id); err != nil {
		return err
	}
	return m.regen()
}
