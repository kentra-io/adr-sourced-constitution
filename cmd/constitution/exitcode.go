package main

import "errors"

// exitError pairs an error with the process exit code main() should use.
// Every other verb returns a plain error, which exitCode below maps to the
// existing default of 1 — this only changes behavior for guard, which
// needs to distinguish "violations found" (1) from "guard could not run"
// (2) per its exit contract (plan §2.7).
type exitError struct {
	err  error
	code int
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// exitCode returns the process exit code an error maps to: the code
// carried by an *exitError, or the pre-existing default of 1 for every
// other error (including nil-safe: exitCode is only ever called when err
// != nil, from main).
func exitCode(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return 1
}
