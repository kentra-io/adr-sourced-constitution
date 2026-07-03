package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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
		},
	})
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
