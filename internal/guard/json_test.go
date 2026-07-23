package guard

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var update = flag.Bool("update", false, "update golden files (testdata/guard.golden.json)")

// encodeResult marshals a Result exactly as cmd/constitution/guard.go's
// writeGuardJSON does (json.Encoder with two-space indent, trailing
// newline) so the golden and the schema-validation tests exercise the real
// on-the-wire bytes, not a differently-configured marshaler.
func encodeResult(t *testing.T, res Result) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(res); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return b.Bytes()
}

// sampleResult is one Result carrying at least one violation of EVERY Kind
// (plan §2.7 enum), each with the optional fields its kind populates, run
// through the same sort Run applies. It is the fixture both the golden
// marshaling test and the schema-conformance test share, so a single
// payload proves both stability and schema-validity across the whole enum.
func sampleResult() Result {
	vs := []Violation{
		{
			Kind: KindBodyChanged, ID: "ADR-0001", File: "constitution/adr/ADR-0001-a.md",
			Message: "ADR-0001: body changed; only status (and its derived superseded-by) may change on an existing ADR",
		},
		{
			Kind: KindFrozenFieldChanged, ID: "ADR-0002", File: "constitution/adr/ADR-0002-b.md", Field: "title",
			Message: `ADR-0002: frozen field "title" changed from "Old" to "New"`,
		},
		{
			Kind: KindFileDeleted, ID: "ADR-0003", File: "constitution/adr/ADR-0003-c.md",
			Message: "ADR-0003: deleted (present at HEAD, absent from the working tree); the ADR log is append-only",
		},
		{
			Kind: KindFileRenamed, ID: "ADR-0004", File: "constitution/adr/ADR-0004-renamed.md",
			OldFile: "constitution/adr/ADR-0004-original.md",
			Message: "ADR-0004: renamed from constitution/adr/ADR-0004-original.md to constitution/adr/ADR-0004-renamed.md; an accepted ADR's filename is frozen along with its content",
		},
		{
			Kind: KindManifestMismatch, ID: "ADR-0005", File: "constitution/adr/ADR-0005-e.md",
			Message: "ADR-0005: on-disk frozen-content hash does not match the hash recorded in .manifest.sha256; the file changed without going through the constitution CLI",
		},
		{
			Kind: KindIDCollision, ID: "ADR-0006", File: "constitution/adr/ADR-0006-f.md",
			Files:   []string{"constitution/adr/ADR-0006-f.md", "constitution/adr/ADR-0006-g.md"},
			Message: "ADR-0006: id used by 2 files: constitution/adr/ADR-0006-f.md, constitution/adr/ADR-0006-g.md",
		},
		{
			Kind: KindUnknownCategory, ID: "ADR-0007", File: "constitution/adr/ADR-0007-h.md",
			Message: `rule tooling/pin-versions uses category "tooling", which is not in the configured vocabulary [architecture testing]`,
		},
	}
	sortViolations(vs)
	return Result{
		Violations: vs,
		Summary:    Summary{Checked: 7, Violations: len(vs), Clean: false},
	}
}

// TestGuardJSONGolden pins the exact bytes of the --format json payload
// (plan §8 golden class; deliverable "JSON marshaling stability"). A change
// to the Result struct's shape, field order, tags, or omitempty behavior
// changes these bytes and fails here — the machine contract does not drift
// silently.
func TestGuardJSONGolden(t *testing.T) {
	got := encodeResult(t, sampleResult())

	golden := filepath.Join("testdata", "guard.golden.json")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (run go test -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("guard JSON payload does not match golden %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
	}
}

// TestKindEnumLockstep holds three things in one enforced agreement (M4):
// the Go enum (allKinds), the JSON-schema fixture's kind enum, and the golden
// sample payload (sampleResult). Adding a 7th Kind without also extending the
// schema enum AND adding a sample violation of that kind fails here — so the
// machine contract (schema + golden bytes) can never silently drift behind the
// code. Order-insensitive on purpose: the schema lists kinds in a fixed order,
// but the guarantee is set-equality, not ordering.
func TestKindEnumLockstep(t *testing.T) {
	want := make(map[string]bool, len(allKinds))
	for _, k := range allKinds {
		want[string(k)] = true
	}
	if len(want) != len(allKinds) {
		t.Fatalf("allKinds contains a duplicate: %v", allKinds)
	}

	t.Run("schema enum == allKinds", func(t *testing.T) {
		got := schemaKindEnum(t)
		if len(got) != len(want) {
			t.Fatalf("schema kind enum has %d entries, allKinds has %d\n schema: %v\n allKinds: %v", len(got), len(want), got, allKinds)
		}
		for _, k := range got {
			if !want[k] {
				t.Errorf("schema kind enum lists %q, which is not in allKinds", k)
			}
		}
	})

	t.Run("sample payload covers exactly allKinds", func(t *testing.T) {
		seen := map[string]bool{}
		for _, v := range sampleResult().Violations {
			if !want[string(v.Kind)] {
				t.Errorf("sample payload carries kind %q not in allKinds", v.Kind)
			}
			seen[string(v.Kind)] = true
		}
		for k := range want {
			if !seen[k] {
				t.Errorf("sample payload is missing a violation of kind %q (every Kind must be exercised by the golden fixture)", k)
			}
		}
	})
}

// schemaKindEnum reads the committed schema fixture and returns its
// violation.kind enum as plain strings.
func schemaKindEnum(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "guard.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var doc struct {
		Defs struct {
			Violation struct {
				Properties struct {
					Kind struct {
						Enum []string `json:"enum"`
					} `json:"kind"`
				} `json:"properties"`
			} `json:"violation"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	enum := doc.Defs.Violation.Properties.Kind.Enum
	if len(enum) == 0 {
		t.Fatal("schema $defs.violation.properties.kind.enum is empty or unreadable")
	}
	return enum
}

// loadSchema compiles the committed JSON Schema fixture once.
func loadSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	const id = "guard.schema.json"
	data, err := os.ReadFile(filepath.Join("testdata", "guard.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(id, doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	sch, err := c.Compile(id)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return sch
}

// validateAgainstSchema decodes JSON bytes into the generic form the
// validator wants and validates them, failing with the schema's own error
// detail on a violation.
func validateAgainstSchema(t *testing.T, sch *jsonschema.Schema, data []byte) {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("re-decode payload: %v", err)
	}
	if err := sch.Validate(v); err != nil {
		t.Errorf("guard JSON payload does not validate against testdata/guard.schema.json:\n%v\n--- payload ---\n%s", err, data)
	}
}

// TestGuardJSONValidatesAgainstSchema is the DoD's "JSON output validates
// against its schema": the all-kinds payload, a clean (zero-violation)
// payload, and — as a negative control proving the schema actually
// constrains — a payload with an unknown kind must be REJECTED.
func TestGuardJSONValidatesAgainstSchema(t *testing.T) {
	sch := loadSchema(t)

	t.Run("all violation kinds", func(t *testing.T) {
		validateAgainstSchema(t, sch, encodeResult(t, sampleResult()))
	})

	t.Run("clean result", func(t *testing.T) {
		clean := Result{Violations: []Violation{}, Summary: Summary{Checked: 3, Violations: 0, Clean: true}}
		validateAgainstSchema(t, sch, encodeResult(t, clean))
	})

	t.Run("unknown kind is rejected (schema is load-bearing)", func(t *testing.T) {
		bad := Result{
			Violations: []Violation{{Kind: "totally_made_up", ID: "ADR-0001", File: "x", Message: "y"}},
			Summary:    Summary{Checked: 1, Violations: 1, Clean: false},
		}
		var v any
		if err := json.Unmarshal(encodeResult(t, bad), &v); err != nil {
			t.Fatal(err)
		}
		if err := sch.Validate(v); err == nil {
			t.Error("schema accepted a violation with an unknown kind; the enum constraint is not being enforced")
		}
	})
}
