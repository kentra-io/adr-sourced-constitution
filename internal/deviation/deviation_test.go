package deviation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scratchProject writes a minimal but valid constitution project (one ADR,
// ADR-0001, plus a constitution.md) into a temp dir and returns its root and
// the canonical hash of its projection.
func scratchProject(t *testing.T) (root, hash string) {
	t.Helper()
	root = t.TempDir()

	mustWrite(t, filepath.Join(root, "constitution.yml"),
		"schemaVersion: 1\nconsent:\n  policy: off\nsourceTracking:\n  type: none\ncategories:\n  - architecture\n")

	adrDir := filepath.Join(root, "constitution", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(adrDir, "ADR-0001-first-rule.md"),
		"---\nid: ADR-0001\ntitle: First rule\ncategory: architecture\ndate: 2026-07-01\nstatus: accepted\n---\n\n"+
			"## Context and Problem Statement\n\nc\n\n## Considered Options\n\n- A\n- B\n\n## Decision Outcome\n\nAdopt A.\n")

	mustWrite(t, filepath.Join(root, "constitution", "constitution.md"),
		"# Constitution\n\n## Architecture\n\n### First rule\n\nAdopt A.\n")

	h, err := ConstitutionHash(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, h
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func report(hash string, devs []Deviation, sum Summary) []byte {
	r := Report{
		GeneratedAt:      "2026-07-03T00:00:00Z",
		ConstitutionHash: hash,
		Plan:             "plan.md",
		Deviations:       devs,
		Summary:          sum,
	}
	b, _ := json.Marshal(r)
	return b
}

func dev(id, adrID, sev string) Deviation {
	return Deviation{
		ID: id, ADRID: adrID, Severity: sev, Rule: "r",
		Location: Location{File: "plan.md"}, Summary: "s", Recommendation: "conform",
	}
}

func TestValidateHappyPath(t *testing.T) {
	root, hash := scratchProject(t)
	data := report(hash,
		[]Deviation{dev("D-001", "ADR-0001", "HIGH")},
		Summary{High: 1})

	res, err := Validate(root, data)
	if err != nil {
		t.Fatalf("could not run: %v", err)
	}
	if !res.Valid() {
		t.Fatalf("want valid, got errors: %v", res.Errors)
	}
	if len(res.Advisories) != 0 {
		t.Fatalf("want no advisories, got: %v", res.Advisories)
	}
}

func TestValidateEmptyDeviationsIsValid(t *testing.T) {
	root, hash := scratchProject(t)
	res, err := Validate(root, report(hash, []Deviation{}, Summary{}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid() || len(res.Advisories) != 0 {
		t.Fatalf("clean conforming report should be valid with no advisories: errors=%v advisories=%v", res.Errors, res.Advisories)
	}
}

func TestValidateUnknownADRID(t *testing.T) {
	root, hash := scratchProject(t)
	data := report(hash,
		[]Deviation{dev("D-001", "ADR-9999", "HIGH")},
		Summary{High: 1})

	res, err := Validate(root, data)
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid() {
		t.Fatal("want invalid for unknown adrId")
	}
	if !containsSubstr(res.Errors, "ADR-9999") || !containsSubstr(res.Errors, "does not match any ADR") {
		t.Fatalf("error should cite the unknown adrId: %v", res.Errors)
	}
}

func TestValidateDuplicateDeviationID(t *testing.T) {
	root, hash := scratchProject(t)
	data := report(hash,
		[]Deviation{dev("D-001", "ADR-0001", "HIGH"), dev("D-001", "ADR-0001", "LOW")},
		Summary{High: 1, Low: 1})

	res, err := Validate(root, data)
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid() || !containsSubstr(res.Errors, "duplicate deviation id") {
		t.Fatalf("want duplicate-id error, got: %v", res.Errors)
	}
}

func TestValidateBadSummaryCounts(t *testing.T) {
	root, hash := scratchProject(t)
	data := report(hash,
		[]Deviation{dev("D-002", "ADR-0001", "HIGH")},
		Summary{Critical: 3}) // claims 3 critical, has 1 high

	res, err := Validate(root, data)
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid() || !containsSubstr(res.Errors, "summary counts do not match") {
		t.Fatalf("want summary-tally error, got: %v", res.Errors)
	}
}

func TestValidateStaleHashIsAdvisory(t *testing.T) {
	root, hash := scratchProject(t)
	_ = hash
	data := report("sha256:deadbeef",
		[]Deviation{dev("D-001", "ADR-0001", "HIGH")},
		Summary{High: 1})

	res, err := Validate(root, data)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid() {
		t.Fatalf("stale hash must NOT make the report invalid: %v", res.Errors)
	}
	if !containsSubstr(res.Advisories, "constitutionHash mismatch") {
		t.Fatalf("want a stale-hash advisory, got: %v", res.Advisories)
	}
}

func TestValidateEmptyHashIsAdvisory(t *testing.T) {
	root, _ := scratchProject(t)
	data := report("", []Deviation{}, Summary{})
	res, err := Validate(root, data)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid() {
		t.Fatalf("empty hash must be advisory, not invalid: %v", res.Errors)
	}
	if !containsSubstr(res.Advisories, "(empty)") {
		t.Fatalf("advisory should note the empty hash and the expected value: %v", res.Advisories)
	}
}

func TestValidateSchemaFailureStopsBeforeSemantic(t *testing.T) {
	root, hash := scratchProject(t)
	// Missing adrId (schema) AND unknown-if-parsed: only the schema error
	// should surface, since a shape failure short-circuits semantic checks.
	raw := `{"generatedAt":"x","constitutionHash":"` + hash + `","plan":"p",` +
		`"deviations":[{"id":"D-001","severity":"HIGH","rule":"r","location":{"file":"p"},"summary":"s","recommendation":"conform"}],` +
		`"summary":{"critical":0,"high":1,"medium":0,"low":0}}`
	res, err := Validate(root, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid() {
		t.Fatal("want invalid for missing adrId")
	}
	if !containsSubstr(res.Errors, "missing property 'adrId'") {
		t.Fatalf("want a precise schema error, got: %v", res.Errors)
	}
}

func TestValidateCannotRunWithoutLog(t *testing.T) {
	root := t.TempDir() // no constitution/adr, no constitution.md
	_, err := Validate(root, report("", []Deviation{}, Summary{}))
	if err == nil {
		t.Fatal("want a could-not-run error when there is no ADR log")
	}
}

func TestConstitutionHashFormat(t *testing.T) {
	root, hash := scratchProject(t)
	_ = root
	if !strings.HasPrefix(hash, "sha256:") || len(hash) != len("sha256:")+64 {
		t.Fatalf("hash should be sha256:<64-hex>, got %q", hash)
	}
}

func TestHashMatches(t *testing.T) {
	const hex64 = "abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd"
	cases := []struct {
		got, exp string
		want     bool
	}{
		{"sha256:" + hex64, "sha256:" + hex64, true},
		{hex64, "sha256:" + hex64, true},                              // bare hex accepted
		{"sha256:" + strings.ToUpper(hex64), "sha256:" + hex64, true}, // case-insensitive
		{"sha256-" + hex64, "sha256:" + hex64, true},                  // alt separator
		{"sha256:deadbeef", "sha256:" + hex64, false},
		{"", "sha256:" + hex64, false},
	}
	for _, c := range cases {
		if got := hashMatches(c.got, c.exp); got != c.want {
			t.Errorf("hashMatches(%q,%q)=%v want %v", c.got, c.exp, got, c.want)
		}
	}
}

// --- fixture + schema-contract tests ---

func TestValidFixtureStructural(t *testing.T) {
	data := readFixture(t, "valid.json")
	errs, err := structuralErrors(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Fatalf("valid.json should pass the schema, got: %v", errs)
	}
}

func TestInvalidFixtureStructural(t *testing.T) {
	data := readFixture(t, "invalid.json")
	errs, err := structuralErrors(data)
	if err != nil {
		t.Fatal(err)
	}
	// invalid.json plants: missing adrId, bad id pattern, bad severity enum,
	// bad recommendation enum — every one must be reported.
	for _, want := range []string{"adrId", "severity", "recommendation", "id"} {
		if !containsSubstr(errs, want) {
			t.Errorf("invalid.json schema errors should mention %q, got: %v", want, errs)
		}
	}
}

func TestDocsSchemaCopyInSync(t *testing.T) {
	docs, err := os.ReadFile(filepath.Join("..", "..", "docs", "deviation.schema.json"))
	if err != nil {
		t.Fatalf("read docs schema copy: %v", err)
	}
	if string(docs) != string(SchemaBytes) {
		t.Error("docs/deviation.schema.json has drifted from the embedded internal/deviation/deviation.schema.json; keep them byte-identical")
	}
}

func TestSeverityEnumLockstep(t *testing.T) {
	var doc struct {
		Defs struct {
			Deviation struct {
				Properties struct {
					Severity struct {
						Enum []string `json:"enum"`
					} `json:"severity"`
				} `json:"properties"`
			} `json:"deviation"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(SchemaBytes, &doc); err != nil {
		t.Fatal(err)
	}
	schemaEnum := doc.Defs.Deviation.Properties.Severity.Enum
	if len(schemaEnum) != len(Severities) {
		t.Fatalf("schema severity enum %v != Severities %v", schemaEnum, Severities)
	}
	for i := range Severities {
		if schemaEnum[i] != Severities[i] {
			t.Errorf("severity enum drift at %d: schema=%q code=%q", i, schemaEnum[i], Severities[i])
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func containsSubstr(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
