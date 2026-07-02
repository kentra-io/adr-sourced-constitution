package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/guard"
)

// Exit codes for `guard`, per the plan §2.7 contract this verb alone needs
// (every other verb keeps the pre-existing 0/1 behavior via exitCode's
// default).
const (
	guardExitViolations  = 1
	guardExitCouldNotRun = 2
)

// guardCommand implements `constitution guard` (spec §5.3/§5.4, plan
// §2.7): check the ADR log for out-of-band mutation, in git mode (default),
// manifest-only mode (--no-git), or CI merge-base mode (--merge-base).
func guardCommand() *cli.Command {
	return &cli.Command{
		Name:  "guard",
		Usage: "check the ADR log for illegal out-of-band mutation (spec §5.3)",
		Description: "Three checks always run: id-uniqueness, a manifest cross-check\n" +
			"(constitution/adr/.manifest.sha256 vs each ADR's current frozen-content\n" +
			"hash), and — unless --no-git or the directory isn't a usable git\n" +
			"repository — a structured git diff against a base ref that allows only\n" +
			"a legal status transition to differ. Exit codes: 0 clean, 1 violations\n" +
			"found, 2 guard could not run at all.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "base", Usage: "git ref to diff against in git mode (default: HEAD)"},
			&cli.StringFlag{Name: "merge-base", Usage: "diff against `git merge-base <target> HEAD`, computed locally (CI mode)"},
			&cli.BoolFlag{Name: "no-git", Usage: "skip git mode; run the manifest + id-uniqueness checks only"},
			&cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text|json"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runGuard(cmd)
		},
	}
}

func runGuard(cmd *cli.Command) error {
	format := cmd.String("format")
	if format != "text" && format != "json" {
		return fmt.Errorf("guard: --format must be %q or %q (got %q)", "text", "json", format)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	opts := guard.Options{
		Root:      cwd,
		Base:      cmd.String("base"),
		MergeBase: cmd.String("merge-base"),
		NoGit:     cmd.Bool("no-git"),
	}

	res, err := guard.Run(opts)
	if err != nil {
		// "Guard could not run at all" (plan §2.7 exit 2): nothing was
		// printed to stdout, so a JSON-consuming caller sees a clean empty
		// pipe rather than a malformed payload. guard.Run's errors already
		// carry a "guard: " prefix, so err is used as-is (no double prefix).
		return &exitError{err: err, code: guardExitCouldNotRun}
	}

	stdout := cmd.Root().Writer
	if format == "json" {
		if err := writeGuardJSON(stdout, res); err != nil {
			return err
		}
	} else {
		if err := writeGuardText(stdout, res); err != nil {
			return err
		}
	}

	if !res.Summary.Clean {
		return &exitError{
			err:  fmt.Errorf("guard: %d violation(s) found", res.Summary.Violations),
			code: guardExitViolations,
		}
	}
	return nil
}

// writeGuardJSON emits ONLY the machine payload (plan §2.7: "JSON-only on
// stdout, pipeable") — violations + summary, nothing else, on one encoder
// so nothing else can interleave with it on stdout.
func writeGuardJSON(w io.Writer, res guard.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func writeGuardText(w io.Writer, res guard.Result) error {
	if res.Summary.Clean {
		if res.Mode == "git" {
			_, err := fmt.Fprintf(w, "guard: clean (%d ADR(s) checked, git mode vs %s)\n", res.Summary.Checked, res.Base)
			return err
		}
		_, err := fmt.Fprintf(w, "guard: clean (%d ADR(s) checked, manifest-only mode)\n", res.Summary.Checked)
		return err
	}

	for _, v := range res.Violations {
		if _, err := fmt.Fprintf(w, "[%s] %s %s\n", v.Kind, v.ID, v.File); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %s\n", v.Message); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "guard: %d ADR(s) checked, %d violation(s) found\n", res.Summary.Checked, res.Summary.Violations)
	return err
}
