package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/deviation"
)

// Exit codes for `deviation validate`, mirroring guard's contract (plan
// §2.9): 0 valid · 1 invalid · 2 could not run.
const (
	deviationExitInvalid     = 1
	deviationExitCouldNotRun = 2
)

// deviationCommand is the hidden plumbing group behind deviation.json (plan
// §2.9). It is NOT one of the seven user verbs (§4): the plan-gate skill runs
// `constitution deviation validate` on the report it writes, so schema and
// citation rules stay in the CLI rather than in prose. Hidden keeps it out of
// the top-level help.
func deviationCommand() *cli.Command {
	return &cli.Command{
		Name:   "deviation",
		Usage:  "plumbing for deviation.json (plan-gate output)",
		Hidden: true,
		Commands: []*cli.Command{
			deviationValidateCommand(),
		},
	}
}

func deviationValidateCommand() *cli.Command {
	return &cli.Command{
		Name:      "validate",
		Usage:     "validate a deviation.json against the schema and the live ADR log",
		ArgsUsage: "<path>",
		Description: "Checks a deviation.json report (spec §8b, plan §2.9): it must be\n" +
			"structurally valid against the deviation schema, every deviation's adrId\n" +
			"must cite an active ADR in the log, deviation ids must be unique, and\n" +
			"the summary counts must tally with the deviations. constitutionHash is\n" +
			"checked against constitution/constitution.md as an ADVISORY: a mismatch is\n" +
			"a HIGH-severity staleness warning on stderr, not a failure.\n\n" +
			"Exit codes: 0 valid, 1 invalid (schema/citation/tally errors on stderr),\n" +
			"2 could not run (report unreadable, not a constitution project root, or the\n" +
			"log could not be read). Run from a constitution project root.",
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return &exitError{err: fmt.Errorf("deviation validate: %w", err), code: deviationExitCouldNotRun}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runDeviationValidate(cmd)
		},
	}
}

func runDeviationValidate(cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return &exitError{
			err:  fmt.Errorf("deviation validate: exactly one <path> argument is required"),
			code: deviationExitCouldNotRun,
		}
	}
	path := cmd.Args().First()

	data, err := os.ReadFile(path)
	if err != nil {
		return &exitError{err: fmt.Errorf("deviation validate: reading %s: %w", path, err), code: deviationExitCouldNotRun}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return &exitError{err: fmt.Errorf("deviation validate: %w", err), code: deviationExitCouldNotRun}
	}
	if _, err := os.Stat(filepath.Join(cwd, "constitution.yml")); err != nil {
		return &exitError{
			err:  fmt.Errorf("deviation validate: no constitution.yml in %s; run from a constitution project root", cwd),
			code: deviationExitCouldNotRun,
		}
	}

	res, err := deviation.Validate(cwd, data)
	if err != nil {
		// Validator could not run (log/projection unreadable): exit 2.
		return &exitError{err: err, code: deviationExitCouldNotRun}
	}

	stderr := cmd.Root().ErrWriter

	// Advisories (e.g. a stale constitutionHash) print regardless of validity —
	// they do not change the exit code.
	for _, a := range res.Advisories {
		if _, werr := fmt.Fprintln(stderr, "deviation validate: "+a); werr != nil {
			return &exitError{err: werr, code: deviationExitCouldNotRun}
		}
	}

	if !res.Valid() {
		for _, e := range res.Errors {
			if _, werr := fmt.Fprintln(stderr, "deviation validate: "+e); werr != nil {
				return &exitError{err: werr, code: deviationExitCouldNotRun}
			}
		}
		return &exitError{
			err:  fmt.Errorf("deviation validate: %s is invalid (%d error(s))", path, len(res.Errors)),
			code: deviationExitInvalid,
		}
	}

	if _, werr := fmt.Fprintf(cmd.Root().Writer, "deviation validate: %s is valid\n", path); werr != nil {
		return &exitError{err: werr, code: deviationExitCouldNotRun}
	}
	return nil
}
