package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

// editCommand implements `constitution adr edit <id>` (v0.2 proposal §3):
// draft-phase-only revision of an existing accepted ADR, per facet — each
// provided flag replaces that facet, omitted facets keep their current
// value. The record's id, date, status, and supersede links are never edit
// facets: date is the decision's original date (an edit is a draft revision,
// not a new decision), and status/links belong to supersede/deprecate/rm.
// All validation, including a fold preflight of the edited log, runs before
// the consent gate; a refusal writes nothing.
func editCommand() *cli.Command {
	return &cli.Command{
		Name:      "edit",
		Usage:     "revise an ADR in place (draft phase only)",
		ArgsUsage: "<id>",
		Description: "Per-facet replacement on a draft-phase log:\n" +
			"  --title        replaces the title (the file is renamed to the new slug)\n" +
			"  --source       replaces the source ref (validated against sourceTracking)\n" +
			"  --body-file    replaces the ENTIRE body, including the \"## Rules\" section's\n" +
			"                 presence or absence — exactly the shape `adr new` accepts\n" +
			"  --rule         replaces only the \"## Rules\" section, keeping the other\n" +
			"                 sections (combinable with --body-file if the file has no Rules)\n" +
			"  --supersedes-rule / --removes-rule\n" +
			"                 replace the whole retirement list; a single empty value (\"\")\n" +
			"                 clears it\n" +
			"Refused once the log is sealed: use supersede/deprecate instead.",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: "title", Usage: "new ADR title (renames the file to match)"},
			&cli.StringFlag{Name: "source", Usage: "new source ref (validated when sourceTracking.type != none)"},
			&cli.StringFlag{Name: "body-file", Usage: "path to a full replacement MADR body (the ## sections), or - for stdin"},
			approveFlag(),
		}, ruleSurfaceFlags()...),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runEdit(cmd)
		},
	}
}

// editFacetFlags are the flags that constitute an edit; at least one must
// be provided (an edit that changes nothing is a mistake, not a no-op).
var editFacetFlags = []string{"title", "source", "body-file", "rule", "supersedes-rule", "removes-rule"}

func runEdit(cmd *cli.Command) error {
	m, err := openRepo(cmd)
	if err != nil {
		return err
	}
	if m.cfg.Phase != config.PhaseDraft {
		return fmt.Errorf(
			"edit: phase is sealed — ADR files are append-only; supersede or deprecate the decision instead")
	}

	id := cmd.Args().Get(0)
	if id == "" {
		return fmt.Errorf("edit: usage: constitution adr edit <id> [facet flags]")
	}
	anySet := false
	for _, f := range editFacetFlags {
		if cmd.IsSet(f) {
			anySet = true
			break
		}
	}
	if !anySet {
		return fmt.Errorf("edit: nothing to change — provide at least one of --%s", strings.Join(editFacetFlags, ", --"))
	}

	oldPath, found, err := adr.FindByID(m.adrDir, id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("edit: no ADR with id %q found in %s", id, m.adrDir)
	}
	old, err := adr.Parse(oldPath)
	if err != nil {
		return err
	}
	if old.Status != adr.StatusAccepted {
		return fmt.Errorf(
			"edit: %s is not accepted (status: %s); revising a retired record is meaningless — edit the ADR that replaced it",
			id, old.Status)
	}

	// --- assemble the edited record: each set flag replaces its facet ---
	title := old.Title
	if cmd.IsSet("title") {
		title = cmd.String("title")
		if strings.TrimSpace(title) == "" {
			return fmt.Errorf("edit: --title must not be empty")
		}
	}
	source := old.Source
	if cmd.IsSet("source") {
		source = cmd.String("source")
		if err := validateSource(m.cfg.SourceTracking, source); err != nil {
			return err
		}
	}
	supersedesRules, err := editRefList(cmd, "supersedes-rule", old.SupersedesRules)
	if err != nil {
		return err
	}
	removesRules, err := editRefList(cmd, "removes-rule", old.RemovesRules)
	if err != nil {
		return err
	}
	body, label, err := editBody(cmd, m, oldPath, old)
	if err != nil {
		return err
	}

	edited := adr.Compose(adr.NewADR{
		ID:              old.ID,
		Title:           title,
		Date:            old.Date,
		Source:          source,
		Supersedes:      old.Supersedes,
		SupersedesRules: supersedesRules,
		RemovesRules:    removesRules,
		Body:            string(body),
	})
	parsed, err := adr.ParseBytesUnnamed(edited, label)
	if err != nil {
		return err
	}

	toAppend, err := m.resolveNewCategories(parsed, cmd.StringSlice("new-category"))
	if err != nil {
		return err
	}

	// Fold preflight on the log with the edited record spliced in for the
	// old one: editing away a rule that a later draft ADR retires must fail
	// here (dangling ref), before the consent gate.
	existing, err := m.parseLog()
	if err != nil {
		return err
	}
	preflight := make([]adr.ADR, 0, len(existing))
	for i := range existing {
		if existing[i].ID == old.ID {
			preflight = append(preflight, *parsed)
			continue
		}
		preflight = append(preflight, existing[i])
	}
	if err := m.preflightFold(preflight, toAppend); err != nil {
		return err
	}

	// --- consent gate ---
	if err := m.gate().confirm("edit " + id); err != nil {
		return err
	}

	// --- writes ---
	if err := m.appendCategories(toAppend); err != nil {
		return err
	}
	newPath := filepath.Join(m.adrDir, adr.Filename(id, title))
	if err := atomicwrite.WriteFile(newPath, edited, 0o644); err != nil {
		return err
	}
	crashCheckpoint("after-edited-file")
	if newPath != oldPath {
		// A title change renames the file to the new slug: write-new then
		// remove-old, renumber's crash pattern — a crash between the two
		// leaves two files sharing an id, which draft guard reports as an
		// id_collision and the operator resolves by deleting the stale one.
		if err := os.Remove(oldPath); err != nil {
			return err
		}
	}
	crashCheckpoint("after-old-removed")

	if _, err := fmt.Fprintf(m.stdout, "edited %s\n", newPath); err != nil {
		return err
	}
	return m.regen()
}

// editRefList resolves a retirement-ref facet: unset keeps the record's
// current refs, a single empty value clears the list, anything else is a
// full replacement (format-validated; semantic checks are the preflight's).
func editRefList(cmd *cli.Command, flagName string, old []adr.RuleRef) ([]string, error) {
	if !cmd.IsSet(flagName) {
		kept := make([]string, len(old))
		for i, r := range old {
			kept[i] = r.String()
		}
		return kept, nil
	}
	vals := cmd.StringSlice(flagName)
	empties := 0
	for _, v := range vals {
		if strings.TrimSpace(v) == "" {
			empties++
		}
	}
	if empties > 0 {
		if len(vals) > 1 {
			return nil, fmt.Errorf(
				"--%s: an empty value clears the list and cannot be combined with refs", flagName)
		}
		return nil, nil
	}
	return ruleRefFlags(flagName, vals)
}

// editBody resolves the body facet from --body-file and/or --rule:
//
//   - --body-file replaces the entire body (Rules included or dropped, per
//     the file), with --rule composable onto a Rules-free file — the exact
//     contract adr new has;
//   - --rule alone replaces only the "## Rules" section, keeping the
//     record's other sections verbatim;
//   - neither keeps the body untouched.
//
// The returned label names the error source for validation/parse failures.
func editBody(cmd *cli.Command, m *mutContext, oldPath string, old *adr.ADR) ([]byte, string, error) {
	ruleFlags := cmd.StringSlice("rule")

	var body []byte
	label := oldPath
	if cmd.IsSet("body-file") {
		var err error
		body, err = readBody(cmd.String("body-file"), m.stdin)
		if err != nil {
			return nil, "", err
		}
		if len(ruleFlags) > 0 && hasRulesSection(body) {
			return nil, "", fmt.Errorf(
				"both --rule and a --body-file that already contains a \"## Rules\" section were supplied; provide the rules exactly once (drop --rule, or remove the section from the body)")
		}
		label = bodyLabel(ruleFlags)
	} else {
		raw, err := os.ReadFile(oldPath)
		if err != nil {
			return nil, "", err
		}
		_, oldBody, err := adr.SplitFrontmatter(raw)
		if err != nil {
			return nil, "", fmt.Errorf("edit: %s: %w", oldPath, err)
		}
		body = oldBody
		if len(ruleFlags) > 0 {
			body = stripRulesSection(body)
			label = "--rule composed \"## Rules\""
		}
	}

	body, err := composeRulesSection(body, ruleFlags)
	if err != nil {
		return nil, "", err
	}
	if err := adr.ValidateBody(body, label); err != nil {
		return nil, "", err
	}
	return body, label, nil
}

// stripRulesSection removes the "## Rules" section (heading through the
// line before the next "## " heading or EOF) from a raw body, so a --rule
// facet can replace it wholesale. A body without one is returned unchanged.
func stripRulesSection(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	var out []string
	inRules := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			inRules = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == adr.RulesSection
		}
		if !inRules {
			out = append(out, line)
		}
	}
	return []byte(strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n")
}
