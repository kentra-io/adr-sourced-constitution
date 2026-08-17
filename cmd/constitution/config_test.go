package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

// runConfigSchemaCLI drives `constitution config schema` in-process through
// the real cli.Command tree, capturing stdout on a buffer via the root
// Writer (cmd.Root().Writer, what runConfigSchema writes to).
func runConfigSchemaCLI(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	root := &cli.Command{
		Name:     "constitution",
		Writer:   &out,
		Commands: []*cli.Command{configCommand()},
	}
	if err := root.Run(context.Background(), []string{"constitution", "config", "schema"}); err != nil {
		t.Fatalf("config schema: %v", err)
	}
	return out.Bytes()
}

func TestConfigSchemaOutputsIndentedJSON(t *testing.T) {
	out := runConfigSchemaCLI(t)

	var fields []config.Field
	if err := json.Unmarshal(out, &fields); err != nil {
		t.Fatalf("config schema output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(fields) == 0 {
		t.Fatal("config schema emitted zero fields")
	}

	// "indented JSON" (milestone deliverable): re-marshaling the decoded
	// value with the same indent must reproduce the output byte-for-byte,
	// modulo the trailing newline json.Encoder appends.
	want, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if got := bytes.TrimRight(out, "\n"); string(got) != string(want) {
		t.Errorf("config schema output is not indented JSON matching MarshalIndent:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestConfigSchemaOutputShape(t *testing.T) {
	out := runConfigSchemaCLI(t)

	var fields []config.Field
	if err := json.Unmarshal(out, &fields); err != nil {
		t.Fatalf("config schema output is not valid JSON: %v", err)
	}

	byKey := make(map[string]config.Field, len(fields))
	for _, f := range fields {
		byKey[f.Key] = f
	}

	st, ok := byKey["sourceTracking.type"]
	if !ok {
		t.Fatal("config schema output missing field \"sourceTracking.type\"")
	}
	wantSourceTracking := map[string]bool{"none": true, "generic": true, "github-issue": true, "jira": true}
	if len(st.Values) != len(wantSourceTracking) {
		t.Errorf("sourceTracking.type Values = %v, want exactly %v", st.Values, wantSourceTracking)
	}
	for _, v := range st.Values {
		if !wantSourceTracking[v] {
			t.Errorf("sourceTracking.type Values contains unexpected %q", v)
		}
	}

	ph, ok := byKey["phase"]
	if !ok {
		t.Fatal("config schema output missing field \"phase\"")
	}
	wantPhase := map[string]bool{"draft": true, "sealed": true}
	if len(ph.Values) != len(wantPhase) {
		t.Errorf("phase Values = %v, want exactly %v", ph.Values, wantPhase)
	}
	for _, v := range ph.Values {
		if !wantPhase[v] {
			t.Errorf("phase Values contains unexpected %q", v)
		}
	}
	if !ph.Required {
		t.Error("phase Field.Required = false, want true")
	}
}

// --- config set ---

// TestConfigSetRejectsIllegalEnumValue proves an illegal enum value fails
// at exit 2, names all four legal sourceTracking.type values, and leaves
// constitution.yml byte-identical to before — the write never happens, it
// isn't undone.
func TestConfigSetRejectsIllegalEnumValue(t *testing.T) {
	setupRepo(t, "off", "architecture")
	before := mustReadFile(t, "constitution.yml")

	err := runCLI(t, "config", "set", "sourceTracking.type", "github")
	if err == nil {
		t.Fatal("config set(sourceTracking.type, github) = nil, want error")
	}
	if got := exitCode(err); got != 2 {
		t.Errorf("exitCode = %d, want 2", got)
	}
	for _, v := range []string{`"none"`, `"generic"`, `"github-issue"`, `"jira"`} {
		if !strings.Contains(err.Error(), v) {
			t.Errorf("error = %q, want it to name legal value %s", err, v)
		}
	}
	if after := mustReadFile(t, "constitution.yml"); after != before {
		t.Errorf("constitution.yml changed despite the refusal:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestConfigSetRefusesGovernedKeys proves each governed key (phase,
// categories, schemaVersion) refuses at exit 2 and names its owning verb —
// or, for schemaVersion, names no writer at all — without touching
// constitution.yml.
func TestConfigSetRefusesGovernedKeys(t *testing.T) {
	setupRepo(t, "off", "architecture")
	before := mustReadFile(t, "constitution.yml")

	redirects := []struct {
		key, value, wantOwner string
	}{
		{"phase", "sealed", "constitution seal"},
		{"categories", "architecture", "adr new --new-category"},
	}
	for _, c := range redirects {
		err := runCLI(t, "config", "set", c.key, c.value)
		if err == nil {
			t.Fatalf("config set(%s) = nil, want error", c.key)
		}
		if got := exitCode(err); got != 2 {
			t.Errorf("config set(%s): exitCode = %d, want 2", c.key, got)
		}
		if !strings.Contains(err.Error(), c.wantOwner) {
			t.Errorf("config set(%s): error = %q, want it to name %q", c.key, err, c.wantOwner)
		}
	}

	err := runCLI(t, "config", "set", "schemaVersion", "2")
	if err == nil {
		t.Fatal("config set(schemaVersion) = nil, want error")
	}
	if got := exitCode(err); got != 2 {
		t.Errorf("config set(schemaVersion): exitCode = %d, want 2", got)
	}
	// Positive assertion, not just "names no owning verb": this is what
	// actually detects schemaVersion becoming settable (a value of "2" is
	// independently rejected by Config.Validate's schemaVersion check, so a
	// purely negative "doesn't name a verb" assertion would keep passing
	// even if the governed-key refusal itself were removed).
	const wantRefusal = "schemaVersion has no writer in this build"
	if !strings.Contains(err.Error(), wantRefusal) {
		t.Errorf("config set(schemaVersion): error = %q, want it to contain %q", err, wantRefusal)
	}
	for _, verb := range []string{"constitution seal", "adr new --new-category"} {
		if strings.Contains(err.Error(), verb) {
			t.Errorf("config set(schemaVersion): error = %q names a writer verb, want none", err)
		}
	}

	if after := mustReadFile(t, "constitution.yml"); after != before {
		t.Errorf("constitution.yml changed despite the refusals:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestConfigSetUnknownKeyRefused proves a key outside both the settable
// and governed vocabularies refuses at exit 2 without touching
// constitution.yml, rather than silently no-op'ing or corrupting the file.
func TestConfigSetUnknownKeyRefused(t *testing.T) {
	setupRepo(t, "off", "architecture")
	before := mustReadFile(t, "constitution.yml")

	err := runCLI(t, "config", "set", "nope.notreal", "x")
	if err == nil {
		t.Fatal("config set(unknown key) = nil, want error")
	}
	if got := exitCode(err); got != 2 {
		t.Errorf("exitCode = %d, want 2", got)
	}
	if after := mustReadFile(t, "constitution.yml"); after != before {
		t.Error("constitution.yml changed despite the refusal")
	}
}

// TestConfigSetEmptyValueRefused proves an empty <value> is refused at
// exit 2 rather than silently resetting the key to its zero-value/default
// (e.g. `config set sourceTracking.type ""` would otherwise write "none"
// without the operator ever having typed it) — config set's whole premise
// is an explicit value.
func TestConfigSetEmptyValueRefused(t *testing.T) {
	setupRepo(t, "off", "architecture")
	before := mustReadFile(t, "constitution.yml")

	err := runCLI(t, "config", "set", "sourceTracking.type", "")
	if err == nil {
		t.Fatal("config set(<value> empty) = nil, want error")
	}
	if got := exitCode(err); got != 2 {
		t.Errorf("exitCode = %d, want 2", got)
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("error = %q, want it to explain the empty <value> is refused", err)
	}
	if after := mustReadFile(t, "constitution.yml"); after != before {
		t.Errorf("constitution.yml changed despite the refusal:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestConfigSetDegenerateListValueRefused proves a list-key <value> that is
// not the literal empty string, but yields zero entries after
// splitConfigList (all-comma / all-whitespace), hits the same
// "must not be empty" refusal — not a silent list-clearing write. This is
// the hole a bare `value == ""` check misses: skills.trees " , " used to
// exit 0 and drop the whole `skills:` key from the file (omitempty).
func TestConfigSetDegenerateListValueRefused(t *testing.T) {
	for _, v := range []string{",", " , "} {
		t.Run(v, func(t *testing.T) {
			setupRepo(t, "off", "architecture")
			before := mustReadFile(t, "constitution.yml")

			err := runCLI(t, "config", "set", "skills.trees", v)
			if err == nil {
				t.Fatalf("config set(skills.trees, %q) = nil, want error", v)
			}
			if got := exitCode(err); got != 2 {
				t.Errorf("exitCode = %d, want 2", got)
			}
			if !strings.Contains(err.Error(), "must not be empty") {
				t.Errorf("error = %q, want it to explain the degenerate <value> is refused", err)
			}
			if after := mustReadFile(t, "constitution.yml"); after != before {
				t.Errorf("constitution.yml changed despite the refusal:\nbefore: %s\nafter:  %s", before, after)
			}
		})
	}
}

// TestConfigSetRoundTrip proves a legal scalar-key set round-trips: the
// command exits 0 and the rewritten constitution.yml reloads cleanly with
// the new value.
func TestConfigSetRoundTrip(t *testing.T) {
	setupRepo(t, "off", "architecture")

	if err := runCLI(t, "config", "set", "sourceTracking.type", "github-issue"); err != nil {
		t.Fatalf("config set(sourceTracking.type, github-issue) = %v, want nil", err)
	}
	cfg, err := config.Load("constitution.yml")
	if err != nil {
		t.Fatalf("reload after config set: %v", err)
	}
	if cfg.SourceTracking.Type != config.SourceTrackingGitHubIssue {
		t.Errorf("SourceTracking.Type = %q, want %q", cfg.SourceTracking.Type, config.SourceTrackingGitHubIssue)
	}
}

// TestConfigSetListKeyCommaSeparated proves the two list-typed settable
// keys accept config set's single positional <value> as a comma-separated
// list.
func TestConfigSetListKeyCommaSeparated(t *testing.T) {
	setupRepo(t, "off", "architecture")

	if err := runCLI(t, "config", "set", "skills.trees", "claude,cursor"); err != nil {
		t.Fatalf("config set(skills.trees) = %v, want nil", err)
	}
	cfg, err := config.Load("constitution.yml")
	if err != nil {
		t.Fatalf("reload after config set: %v", err)
	}
	want := []string{"claude", "cursor"}
	if len(cfg.Skills.Trees) != len(want) || cfg.Skills.Trees[0] != want[0] || cfg.Skills.Trees[1] != want[1] {
		t.Errorf("Skills.Trees = %v, want %v", cfg.Skills.Trees, want)
	}
}
