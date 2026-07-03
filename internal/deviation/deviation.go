// Package deviation is the CLI-owned validator for deviation.json — the
// per-plan report the plan-gate skill writes (spec §8b,
// implementation-plan.md §2.9). It is exercised by the hidden plumbing verb
// `constitution deviation validate <path>`: the plan-gate skill writes the
// report and then runs the validator on it, so the schema and the
// citation/staleness rules live in the CLI, not in prose the agent has to
// reproduce.
//
// The validator layers three checks:
//
//  1. Structural — the JSON validates against the committed schema
//     (deviation.schema.json, embedded here and copied to docs/). Required
//     fields, the severity/recommendation enums, and the D-NNN / ADR-NNNN id
//     shapes are all schema-enforced.
//  2. Semantic — every deviation's adrId matches an ACTIVE (accepted) ADR in
//     the live log (the citation is the governance primitive, so a citation to
//     a nonexistent ADR — or to a superseded/deprecated one that is no longer
//     in force — is a hard failure), deviation ids are unique, and the summary
//     counts tally exactly with the per-severity totals.
//  3. Staleness (advisory) — constitutionHash is compared against the
//     current constitution/constitution.md. A mismatch is a HIGH-severity
//     advisory, not a failure: the report is structurally sound, it is just
//     describing a constitution that has since changed.
//
// Exit-code mapping lives in cmd/constitution: a non-nil error from Validate
// is "could not run" (exit 2), a Result with Errors is "invalid" (exit 1),
// and a Result without Errors is "valid" (exit 0) even if it carries
// Advisories.
package deviation

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
)

// schemaMsgPrinter localizes jsonschema leaf-error messages. The library's
// own defaultPrinter is unexported, so we supply our own (English) — the
// output ("missing property 'adrId'", "value must be one of …") is what the
// validator surfaces on stderr.
var schemaMsgPrinter = message.NewPrinter(language.English)

// SchemaBytes is the canonical deviation.json JSON Schema, embedded so the
// CLI self-validates against the exact document shipped under docs/. A test
// (TestDocsSchemaCopyInSync) holds the docs/ copy byte-identical to this one.
//
//go:embed deviation.schema.json
var SchemaBytes []byte

// Severities is the Spec-Kit severity vocabulary, in descending order. It is
// held in lockstep with the schema's enum by TestSeverityEnumLockstep.
var Severities = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"}

// Report is the deviation.json document (spec §8b, plan §2.9).
type Report struct {
	GeneratedAt      string      `json:"generatedAt"`
	ConstitutionHash string      `json:"constitutionHash"`
	Plan             string      `json:"plan"`
	Deviations       []Deviation `json:"deviations"`
	Summary          Summary     `json:"summary"`
}

// Deviation is one cited conflict between a plan and an active rule.
type Deviation struct {
	ID                   string   `json:"id"`
	ADRID                string   `json:"adrId"`
	Severity             string   `json:"severity"`
	Rule                 string   `json:"rule"`
	Location             Location `json:"location"`
	Summary              string   `json:"summary"`
	Recommendation       string   `json:"recommendation"`
	RecommendationDetail string   `json:"recommendationDetail,omitempty"`
}

// Location is where in the plan a deviation was found.
type Location struct {
	File  string `json:"file"`
	Lines string `json:"lines,omitempty"`
}

// Summary is the per-severity roll-up.
type Summary struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// Result is one validation outcome. Errors non-empty means the report is
// invalid (exit 1); Advisories carry non-fatal notes (e.g. a stale hash) and
// do not affect validity.
type Result struct {
	Errors     []string
	Advisories []string
}

// Valid reports whether the deviation.json passed (no hard errors).
func (r Result) Valid() bool { return len(r.Errors) == 0 }

var (
	compiledOnce sync.Once
	compiled     *jsonschema.Schema
	compileErr   error
)

func schema() (*jsonschema.Schema, error) {
	compiledOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaBytes))
		if err != nil {
			compileErr = fmt.Errorf("deviation: embedded schema is not valid JSON: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		const id = "deviation.schema.json"
		if err := c.AddResource(id, doc); err != nil {
			compileErr = fmt.Errorf("deviation: adding embedded schema: %w", err)
			return
		}
		compiled, compileErr = c.Compile(id)
	})
	return compiled, compileErr
}

// Validate checks the deviation.json in data against the schema and against
// the live constitution log rooted at root (the directory holding
// constitution.yml and constitution/). A non-nil error means the validator
// itself could not run — the log or the projection could not be read (exit
// 2). Otherwise the Result's Errors/Advisories carry the findings.
func Validate(root string, data []byte) (Result, error) {
	var res Result

	// --- 1. structural (schema) ---
	schemaErrs, err := structuralErrors(data)
	if err != nil {
		return Result{}, err
	}
	if len(schemaErrs) > 0 {
		// A shape failure means the semantic checks cannot be trusted
		// (fields may be missing or mistyped): report structure and stop.
		res.Errors = schemaErrs
		return res, nil
	}

	// Schema passed, so a strict decode cannot fail on shape; a decode error
	// here would be a validator bug, surfaced as could-not-run.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rep Report
	if err := dec.Decode(&rep); err != nil {
		return Result{}, fmt.Errorf("deviation: report passed schema but did not decode: %w", err)
	}

	// --- 2. semantic ---
	statuses, err := adrStatuses(root)
	if err != nil {
		return Result{}, err
	}
	res.Errors = append(res.Errors, semanticErrors(rep, statuses)...)

	// --- 3. staleness (advisory) ---
	expected, err := ConstitutionHash(root)
	if err != nil {
		return Result{}, err
	}
	if !hashMatches(rep.ConstitutionHash, expected) {
		got := rep.ConstitutionHash
		if got == "" {
			got = "(empty)"
		}
		res.Advisories = append(res.Advisories, fmt.Sprintf(
			"constitutionHash mismatch [HIGH — stale gate]: got %s, expected %s; "+
				"constitution/constitution.md changed since this gate ran — set constitutionHash to the expected value (or re-run the gate)",
			got, expected))
	}

	return res, nil
}

// structuralErrors validates data against the embedded schema and returns one
// human-readable string per schema violation, each anchored to its JSON
// location. A non-nil error is a validator-internal failure (bad embedded
// schema), not a report defect.
func structuralErrors(data []byte) ([]string, error) {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return []string{"not valid JSON: " + err.Error()}, nil
	}
	sch, err := schema()
	if err != nil {
		return nil, err
	}
	verr := sch.Validate(inst)
	if verr == nil {
		return nil, nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(verr, &ve) {
		return []string{verr.Error()}, nil
	}
	return flattenSchemaErrors(ve), nil
}

// flattenSchemaErrors walks a jsonschema ValidationError tree and returns its
// LEAF failures (the nodes carrying the concrete keyword message — "missing
// property 'adrId'", "value must be one of …") as "at <instanceLocation>:
// <message>" lines, sorted and de-duplicated so the output is stable for
// testing regardless of the library's internal traversal order. Intermediate
// wrapper nodes (allOf/$ref/schema) have causes and are skipped.
func flattenSchemaErrors(ve *jsonschema.ValidationError) []string {
	seen := map[string]bool{}
	var lines []string
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		line := fmt.Sprintf("at %s: %s",
			instancePointer(e.InstanceLocation),
			e.ErrorKind.LocalizedString(schemaMsgPrinter))
		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	walk(ve)
	if len(lines) == 0 {
		lines = append(lines, ve.Error())
	}
	sort.Strings(lines)
	return lines
}

// instancePointer renders a jsonschema InstanceLocation ([]segment) as a JSON
// Pointer ("/deviations/0/adrId"), or "(root)" for the document root.
func instancePointer(segments []string) string {
	if len(segments) == 0 {
		return "(root)"
	}
	return "/" + strings.Join(segments, "/")
}

// semanticErrors checks the citation and tally invariants the schema cannot
// express: every adrId names an ACTIVE (accepted) ADR — a citation to a
// superseded or deprecated rule is a hard failure, since the plan-gate must
// only measure a plan against rules still in force — deviation ids are
// unique, and the summary counts match the per-severity totals.
func semanticErrors(rep Report, statuses map[string]adr.Status) []string {
	var errs []string

	seen := map[string]bool{}
	tally := Summary{}
	for i, d := range rep.Deviations {
		where := fmt.Sprintf("deviations[%d] (%s)", i, d.ID)
		if seen[d.ID] {
			errs = append(errs, fmt.Sprintf("%s: duplicate deviation id %q", where, d.ID))
		}
		seen[d.ID] = true

		if status, ok := statuses[d.ADRID]; !ok {
			errs = append(errs, fmt.Sprintf(
				"%s: adrId %q does not match any ADR in the log; every deviation must cite a real active rule",
				where, d.ADRID))
		} else if status != adr.StatusAccepted {
			errs = append(errs, fmt.Sprintf(
				"deviations[%d].adrId: %s is %s (deviations must cite active ADRs)",
				i, d.ADRID, status))
		}

		switch d.Severity {
		case "CRITICAL":
			tally.Critical++
		case "HIGH":
			tally.High++
		case "MEDIUM":
			tally.Medium++
		case "LOW":
			tally.Low++
		}
	}

	if rep.Summary != tally {
		errs = append(errs, fmt.Sprintf(
			"summary counts do not match the deviations: summary=%s, actual=%s",
			fmtSummary(rep.Summary), fmtSummary(tally)))
	}
	return errs
}

func fmtSummary(s Summary) string {
	return fmt.Sprintf("{critical:%d high:%d medium:%d low:%d}", s.Critical, s.High, s.Medium, s.Low)
}

// adrStatuses maps each ADR id in the live log to its lifecycle status, so the
// semantic check can require a citation to point at an ACTIVE (accepted) rule.
// A read failure is a could-not-run condition (the validator cannot confirm
// citations without the log).
func adrStatuses(root string) (map[string]adr.Status, error) {
	adrDir := filepath.Join(root, "constitution", "adr")
	adrs, err := adr.ParseDir(adrDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("deviation: no ADR log at %s; run validate from a constitution project root", adrDir)
		}
		return nil, fmt.Errorf("deviation: reading the ADR log: %w", err)
	}
	statuses := make(map[string]adr.Status, len(adrs))
	for _, a := range adrs {
		statuses[a.ID] = a.Status
	}
	return statuses, nil
}

// ConstitutionHash computes the canonical sha256 of the on-disk
// constitution/constitution.md ("sha256:<hex>") — the value deviation.json's
// constitutionHash field should carry.
func ConstitutionHash(root string) (string, error) {
	path := filepath.Join(root, "constitution", "constitution.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("deviation: reading %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// hashMatches reports whether a report's constitutionHash equals expected,
// accepting either the canonical "sha256:<hex>" form or a bare 64-hex digest,
// case-insensitively on the hex.
func hashMatches(got, expected string) bool {
	return normalizeHash(got) == normalizeHash(expected)
}

func normalizeHash(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.TrimPrefix(h, "sha256:")
	h = strings.TrimPrefix(h, "sha256-")
	return h
}
