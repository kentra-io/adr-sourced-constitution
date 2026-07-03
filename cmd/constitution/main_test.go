package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain lets testscript scripts under testdata/script `exec constitution
// ...` against the real CLI logic, in-process, per the standard
// testscript.Main pattern: this test binary re-execs itself as `constitution`
// when os.Args[0] matches.
//
// The registered wrapper mirrors main() exactly — including os.Exit(exitCode(err))
// — so black-box scripts observe the real exit contract (0 clean, 1
// violations, 2 could-not-run; plan §2.7), not a flattened "1 on any error".
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"constitution": func() {
			if err := run(context.Background(), os.Args); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(exitCode(err))
			}
		},
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			// exitcode <n> <command> [args...] runs a command and asserts its
			// process exit code equals <n>, capturing stdout/stderr so the
			// usual stdout/stderr builtins still match afterwards. testscript's
			// own `! exec` only distinguishes zero from non-zero; guard's
			// contract needs 1 (violations) told apart from 2 (could not run),
			// so this is how the e2e suite asserts the exact code.
			"exitcode": cmdExitcode,
			// injecthash <jsonfile> replaces the literal token __HASH__ in
			// <jsonfile> with "sha256:<hex>" of constitution/constitution.md,
			// so a deviation-validate e2e can build a report whose
			// constitutionHash actually matches the rendered projection without
			// depending on a platform sha256 binary. This mirrors the plan-gate
			// skill's real loop (fill the hash the validator reports).
			"injecthash": cmdInjecthash,
		},
	})
}

func cmdInjecthash(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: injecthash <jsonfile>")
	}
	md, err := os.ReadFile(ts.MkAbs("constitution/constitution.md"))
	if err != nil {
		ts.Fatalf("injecthash: reading constitution.md: %v", err)
	}
	sum := sha256.Sum256(md)
	hash := "sha256:" + hex.EncodeToString(sum[:])

	target := ts.MkAbs(args[0])
	data, err := os.ReadFile(target)
	if err != nil {
		ts.Fatalf("injecthash: reading %s: %v", args[0], err)
	}
	out := strings.ReplaceAll(string(data), "__HASH__", hash)
	if err := os.WriteFile(target, []byte(out), 0o644); err != nil {
		ts.Fatalf("injecthash: writing %s: %v", args[0], err)
	}
}

func cmdExitcode(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: exitcode <n> <command> [args...]")
	}
	want, err := strconv.Atoi(args[0])
	if err != nil {
		ts.Fatalf("exitcode: first argument must be a number, got %q", args[0])
	}

	runErr := ts.Exec(args[1], args[2:]...)
	got := 0
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			got = ee.ExitCode()
		} else {
			ts.Fatalf("exitcode: running %v: %v", args[1:], runErr)
		}
	}

	if neg {
		if got == want {
			ts.Fatalf("exitcode: %v exited %d, did not want %d", args[1:], got, want)
		}
		return
	}
	if got != want {
		ts.Fatalf("exitcode: %v exited %d, want %d", args[1:], got, want)
	}
}
