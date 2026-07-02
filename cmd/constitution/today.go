package main

import (
	"os"
	"time"
)

// todayEnvVar is an undocumented, test-only override for the creation date
// stamped onto a new ADR, so lifecycle tests produce byte-stable fixtures
// without freezing the wall clock. Format is "YYYY-MM-DD"; anything else is
// ignored and the real local date is used.
const todayEnvVar = "CONSTITUTION_TODAY"

// today returns the ADR creation date (frozen forever once written, spec
// §4.1). It honors CONSTITUTION_TODAY for tests, else the local date.
func today() string {
	if v := os.Getenv(todayEnvVar); v != "" {
		if _, err := time.Parse("2006-01-02", v); err == nil {
			return v
		}
	}
	return time.Now().Format("2006-01-02")
}
