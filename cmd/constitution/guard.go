package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/config"
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
		// A flag-parse/usage error (unknown flag, missing value) is a
		// "could not run" condition, not "violations found": map it to exit 2
		// like every other guard usage error, so a JSON consumer never reads a
		// bare parse failure as exit 1 (plan §2.7 exit contract).
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return &exitError{err: fmt.Errorf("guard: %w", err), code: guardExitCouldNotRun}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runGuard(cmd)
		},
	}
}

func runGuard(cmd *cli.Command) error {
	// Usage errors are "could not run" (exit 2), never "violations" (exit 1):
	// a JSON consumer must be able to tell a bad invocation from a dirty log.
	format := cmd.String("format")
	if format != "text" && format != "json" {
		return &exitError{
			err:  fmt.Errorf("guard: --format must be %q or %q (got %q)", "text", "json", format),
			code: guardExitCouldNotRun,
		}
	}

	base, mergeBase := cmd.String("base"), cmd.String("merge-base")
	if base != "" && mergeBase != "" {
		return &exitError{
			err:  fmt.Errorf("guard: --base and --merge-base are mutually exclusive (--base pins an explicit ref; --merge-base computes one against a target — pick one)"),
			code: guardExitCouldNotRun,
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return &exitError{err: fmt.Errorf("guard: %w", err), code: guardExitCouldNotRun}
	}

	// Validate that cwd actually is a constitution project root before
	// reporting anything: guard's Root is the current directory, and without
	// this a run from a subdirectory (or anywhere with no project at all)
	// scans zero ADRs and reports a false "clean". Require constitution.yml
	// here. A project WITH constitution.yml but no ADRs yet is still
	// legitimately clean — that case is handled downstream, not rejected here.
	configPath := filepath.Join(cwd, "constitution.yml")
	if _, err := os.Stat(configPath); err != nil {
		return &exitError{
			err:  fmt.Errorf("guard: no constitution.yml in %s; run guard from a constitution project root", cwd),
			code: guardExitCouldNotRun,
		}
	}
	// The config decides which guard semantics apply (phase, vocabulary), so
	// an unreadable/invalid config is a "could not run", not a violation.
	cfg, err := config.Load(configPath)
	if err != nil {
		return &exitError{err: fmt.Errorf("guard: %w", err), code: guardExitCouldNotRun}
	}

	opts := guard.Options{
		Root:       cwd,
		Base:       base,
		MergeBase:  mergeBase,
		NoGit:      cmd.Bool("no-git"),
		Phase:      cfg.Phase,
		Categories: cfg.Categories,
	}

	res, err := guard.Run(opts)
	if err != nil {
		// "Guard could not run at all" (plan §2.7 exit 2): nothing was
		// printed to stdout, so a JSON-consuming caller sees a clean empty
		// pipe rather than a malformed payload. guard.Run's errors already
		// carry a "guard: " prefix, so err is used as-is (no double prefix).
		return &exitError{err: err, code: guardExitCouldNotRun}
	}

	// An output-write failure is a "could not run" condition (exit 2), not
	// "violations found" (exit 1): a JSON consumer that got a truncated/failed
	// write must not mistake it for a clean-vs-dirty signal.
	stdout := cmd.Root().Writer
	if format == "json" {
		if err := writeGuardJSON(stdout, res); err != nil {
			return &exitError{err: fmt.Errorf("guard: writing JSON output: %w", err), code: guardExitCouldNotRun}
		}
	} else {
		if err := writeGuardText(stdout, res); err != nil {
			return &exitError{err: fmt.Errorf("guard: writing output: %w", err), code: guardExitCouldNotRun}
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
		_, err := fmt.Fprintf(w, "guard: clean (%d ADR(s) checked, %s mode)\n", res.Summary.Checked, res.Mode)
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
