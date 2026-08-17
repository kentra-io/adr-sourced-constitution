package main

import (
	"bytes"
	"context"
	"encoding/json"
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
