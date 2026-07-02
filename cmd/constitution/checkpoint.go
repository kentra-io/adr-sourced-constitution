package main

import "os"

// crashEnvVar names the undocumented, test-only hook that drives the
// crash-injection tests (M2 DoD). When set to a checkpoint name, the process
// exits hard (os.Exit) the instant it reaches that checkpoint between two
// file operations in a multi-write sequence — simulating a power loss or
// kill at exactly that seam. It exists to prove the "log is truth, regen
// self-heals" recovery story on real processes; it is never referenced by
// normal operation and is deliberately absent from --help.
const crashEnvVar = "CONSTITUTION_CRASH_AFTER"

// crashCheckpoint exits hard if the crash-injection env var names this
// checkpoint. Placed between successive writes in the mutating sequences so
// a test can sever the process at each seam and then assert the ADR log
// still parses and a following regen converges.
func crashCheckpoint(name string) {
	if os.Getenv(crashEnvVar) == name {
		// 128+SIGKILL(9); an unmistakable "killed mid-write" exit code.
		os.Exit(137)
	}
}
