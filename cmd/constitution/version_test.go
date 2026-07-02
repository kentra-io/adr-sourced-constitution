package main

import (
	"runtime/debug"
	"testing"
)

// setLdflagsVars overrides the ldflags-injected package vars for a test and
// restores the originals on cleanup.
func setLdflagsVars(t *testing.T, v, c, d string) {
	t.Helper()
	origVersion, origCommit, origDate := version, commit, date
	t.Cleanup(func() {
		version, commit, date = origVersion, origCommit, origDate
	})
	version, commit, date = v, c, d
}

func TestBuildVersionLdflags(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		want    string
	}{
		{
			name:    "version only",
			version: "v1.2.3",
			want:    "v1.2.3",
		},
		{
			name:    "version and commit",
			version: "v1.2.3",
			commit:  "abc1234",
			want:    "v1.2.3 (abc1234)",
		},
		{
			name:    "version, commit, and date",
			version: "v1.2.3",
			commit:  "abc1234",
			date:    "2026-07-02T18:00:00Z",
			want:    "v1.2.3 (abc1234) built 2026-07-02T18:00:00Z",
		},
		{
			name:    "version and date without commit",
			version: "v1.2.3",
			date:    "2026-07-02T18:00:00Z",
			want:    "v1.2.3 built 2026-07-02T18:00:00Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setLdflagsVars(t, tt.version, tt.commit, tt.date)
			if got := buildVersion(); got != tt.want {
				t.Errorf("buildVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBuildVersionFallsBackToBuildInfo exercises the real ReadBuildInfo
// path: with no ldflags values set, the test binary always carries build
// info, so buildVersion must return something non-empty and not "unknown"
// (test binaries report at least a Main.Version of "(devel)").
func TestBuildVersionFallsBackToBuildInfo(t *testing.T) {
	setLdflagsVars(t, "", "", "")
	got := buildVersion()
	if got == "" || got == "unknown" {
		t.Errorf("buildVersion() = %q, want a build-info-derived version", got)
	}
}

// TestBuildVersionBuildInfoNotOK covers the ReadBuildInfo-failure branch,
// which cannot be triggered for real inside a test binary.
func TestBuildVersionBuildInfoNotOK(t *testing.T) {
	setLdflagsVars(t, "", "", "")
	orig := readBuildInfo
	t.Cleanup(func() { readBuildInfo = orig })
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	if got := buildVersion(); got != "unknown" {
		t.Errorf("buildVersion() = %q, want %q", got, "unknown")
	}
}

// TestBuildVersionUsesBuildInfo verifies buildVersion routes through the
// build-info formatter when ldflags are absent, using synthetic info.
func TestBuildVersionUsesBuildInfo(t *testing.T) {
	setLdflagsVars(t, "", "", "")
	orig := readBuildInfo
	t.Cleanup(func() { readBuildInfo = orig })
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "b99f76df546328b3674791884f93d5a62809a077"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	info.Main.Version = "v0.0.0-20260702183017-b99f76df5463"
	readBuildInfo = func() (*debug.BuildInfo, bool) { return info, true }

	want := "v0.0.0-20260702183017-b99f76df5463 (b99f76df5463-dirty)"
	if got := buildVersion(); got != want {
		t.Errorf("buildVersion() = %q, want %q", got, want)
	}
}

func TestBuildInfoVersion(t *testing.T) {
	tests := []struct {
		name     string
		main     string
		settings []debug.BuildSetting
		want     string
	}{
		{
			name: "module version only, no vcs settings",
			main: "v0.0.0-20260702183017-b99f76df5463",
			want: "v0.0.0-20260702183017-b99f76df5463",
		},
		{
			name: "empty version, no settings",
			main: "",
			want: "unknown",
		},
		{
			name: "short revision kept as-is",
			main: "(devel)",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123"},
			},
			want: "(devel) (abc123)",
		},
		{
			name: "long revision truncated to 12 chars",
			main: "(devel)",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "b99f76df546328b3674791884f93d5a62809a077"},
			},
			want: "(devel) (b99f76df5463)",
		},
		{
			name: "dirty working tree",
			main: "(devel)",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "b99f76df546328b3674791884f93d5a62809a077"},
				{Key: "vcs.modified", Value: "true"},
			},
			want: "(devel) (b99f76df5463-dirty)",
		},
		{
			name: "vcs.modified false is not dirty",
			main: "(devel)",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123"},
				{Key: "vcs.modified", Value: "false"},
			},
			want: "(devel) (abc123)",
		},
		{
			name: "empty version with revision",
			main: "",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123"},
			},
			want: "unknown (abc123)",
		},
		{
			name: "unrelated settings ignored",
			main: "v1.0.0",
			settings: []debug.BuildSetting{
				{Key: "GOOS", Value: "linux"},
				{Key: "vcs.time", Value: "2026-07-02T18:00:00Z"},
			},
			want: "v1.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &debug.BuildInfo{Settings: tt.settings}
			info.Main.Version = tt.main
			if got := buildInfoVersion(info); got != tt.want {
				t.Errorf("buildInfoVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
