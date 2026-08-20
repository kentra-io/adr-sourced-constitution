// Package config loads and validates constitution.yml (spec §7,
// implementation-plan.md §2.10): the project config `constitution init`
// writes and every command reads, at repo root, sibling of constitution/.
package config

import (
	"fmt"
	"os"

	yaml "go.yaml.in/yaml/v3"
)

// SchemaVersion is the only schemaVersion this build understands. Plan
// §2.10: "unknown schemaVersion ⇒ refuse with a clear message. No
// migration machinery in v1."
const SchemaVersion = 1

// Config is the constitution.yml schema (plan §2.10, §2.5, §2.8; v0.2
// proposal §3 for phase).
type Config struct {
	SchemaVersion     int               `yaml:"schemaVersion"`
	AgentInstructions AgentInstructions `yaml:"agentInstructions"`
	Consent           Consent           `yaml:"consent"`
	SourceTracking    SourceTracking    `yaml:"sourceTracking"`
	// Phase is the founding phase (v0.2 proposal D1/A3): "draft" means the
	// log is a fully mutable working set (edit/rm allowed, no manifest
	// baseline, guard checks parse + vocabulary only); "sealed" means
	// append-only forever with the full guard semantics. REQUIRED — the one
	// config field with no default, because defaulting either way silently
	// picks an immutability model (A3: sealing is always an explicit,
	// human-approved act; D4: no absent-field compatibility logic).
	Phase string `yaml:"phase"`
	// Categories is the project's category vocabulary (spec §4.2), in the
	// order the projection groups sections by (plan §3: "categories in
	// config order"). Governed: a new category is introduced by an ADR
	// (plan §2.5); `regen` hard-errors on any ADR category outside it.
	Categories []string `yaml:"categories"`
	// Skills records which agent-skill trees `constitution init` fans the
	// Layer-2 skills out to and `regen` keeps refreshed (plan §6). Optional:
	// absent/empty means no fan-out (the pre-M4 config shape still parses).
	Skills Skills `yaml:"skills,omitempty"`
}

// AgentInstructions records which agent-instruction file(s) get the
// managed pointer block (spec §7 item 1, §9.1, plan §2.1/§5). The values
// are the target keys "claude" (→ CLAUDE.md, an `@import` pointer) and
// "agents" (→ AGENTS.md, a short textual pointer). Written by `init`,
// honored (not overridden by flags) on re-runs; `regen` refreshes the
// blocks named here.
type AgentInstructions struct {
	Targets []string `yaml:"targets"` // "claude", "agents"
}

// Skills records the agent-skill fan-out trees (plan §6). Values are the
// tree keys "claude" (→ .claude/skills/), "agents" (→ .agents/skills/,
// covering Codex + Gemini), and "cursor" (→ .cursor/skills/). Default on
// `init` is all three.
type Skills struct {
	Trees []string `yaml:"trees,omitempty"`
}

// Target-key and skills-tree vocabularies (plan §2.1/§5/§6).
const (
	TargetClaude = "claude"
	TargetAgents = "agents"

	SkillTreeClaude = "claude"
	SkillTreeAgents = "agents"
	SkillTreeCursor = "cursor"
)

// Config.Phase values (v0.2 proposal D1/A3).
const (
	PhaseDraft  = "draft"
	PhaseSealed = "sealed"
)

// Consent policy vocabulary (plan §2.4): "strict" | "off". Not enforced
// by M1 (regen is a non-mutating command); consumed by M2's mutating
// verbs.
type Consent struct {
	Policy string `yaml:"policy"`
}

// Consent.Policy values (plan §2.4).
const (
	ConsentStrict = "strict"
	ConsentOff    = "off"
)

// SourceTracking configures the source-ref format (spec §7 item 3, plan
// §2.8). Type "none" means no ADR may carry a `source` field at all.
// Presence/shape enforcement of `source` against this config is the write
// path's job (M2's `adr new`); M1 only parses/validates the config shape.
type SourceTracking struct {
	Type    string `yaml:"type"`
	Pattern string `yaml:"pattern"`
}

// SourceTracking.Type values (spec §7 item 3, plan §2.8).
const (
	SourceTrackingNone        = "none"
	SourceTrackingGeneric     = "generic"
	SourceTrackingGitHubIssue = "github-issue"
	SourceTrackingJira        = "jira"
)

var validSourceTrackingTypes = map[string]bool{
	SourceTrackingNone:        true,
	SourceTrackingGeneric:     true,
	SourceTrackingGitHubIssue: true,
	SourceTrackingJira:        true,
}

var validTargets = map[string]bool{
	TargetClaude: true,
	TargetAgents: true,
}

var validSkillTrees = map[string]bool{
	SkillTreeClaude: true,
	SkillTreeAgents: true,
	SkillTreeCursor: true,
}

var validPhases = map[string]bool{
	PhaseDraft:  true,
	PhaseSealed: true,
}

var validConsentPolicies = map[string]bool{
	ConsentStrict: true,
	ConsentOff:    true,
}

// Load reads and validates constitution.yml at path.
func Load(path string) (*Config, error) {
	cfg, err := loadRaw(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.validate(path); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadLenient reads constitution.yml at path WITHOUT validating it.
//
// It exists for ONE caller — `constitution config set` (issue #27) —
// and every read path must keep using Load. The guarantee that
// matters is that an invalid config is never WRITTEN, which config
// set still enforces by validating the whole result before its
// atomic write; the guarantee that backfired is that an invalid
// config could never be EDITED, which froze the one supported writer
// against exactly the files needing repair.
//
// Note the defaulting difference: validate applies the
// consent.policy/sourceTracking.type defaults as a side effect, so a
// LoadLenient'd Config carries raw zero values until the caller's own
// Validate() runs.
func LoadLenient(path string) (*Config, error) {
	return loadRaw(path)
}

// loadRaw reads and unmarshals constitution.yml at path, applying no
// validation and no defaulting. Shared by Load and LoadLenient so
// the two cannot drift in how the file is read.
func loadRaw(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: not valid YAML: %w", path, err)
	}
	return &cfg, nil
}

// Validate re-runs Config's full validation rules — the same ones Load
// enforces against a file on disk — against an in-memory Config that has no
// (or not yet the real) backing file. Used by mutating verbs that build or
// mutate a Config value before deciding whether to persist it (`constitution
// config set`): re-validate the WHOLE resulting Config before writing a
// single byte, so an illegal value never touches the real constitution.yml.
//
// Shares validate's one implementation (no rule duplication) and its
// defaulting side effect: an empty consent.policy/sourceTracking.type is
// defaulted to "strict"/"none" exactly as Load's defaulting does. Error
// messages use the literal label "constitution.yml" in place of a real
// path, since there may be none yet.
func (c *Config) Validate() error {
	return c.validate("constitution.yml")
}

func (c *Config) validate(path string) error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf(
			"%s: unsupported schemaVersion %d (this build only supports schemaVersion %d); refusing to run against an unrecognized config schema",
			path, c.SchemaVersion, SchemaVersion,
		)
	}

	if len(c.Categories) == 0 {
		return fmt.Errorf("%s: field %q: at least one category is required", path, "categories")
	}
	seen := make(map[string]bool, len(c.Categories))
	for _, cat := range c.Categories {
		if cat == "" {
			return fmt.Errorf("%s: field %q: category entries must not be empty", path, "categories")
		}
		if seen[cat] {
			return fmt.Errorf("%s: field %q: duplicate category %q", path, "categories", cat)
		}
		seen[cat] = true
	}

	switch {
	case validPhases[c.Phase]:
	case c.Phase == "":
		return fmt.Errorf(
			"%s: field %q: required — add 'phase: sealed' for an existing (append-only) log, or 'phase: draft' for an unsealed one",
			path, "phase",
		)
	default:
		return fmt.Errorf(
			"%s: field %q: must be %q or %q (got %q)",
			path, "phase", PhaseDraft, PhaseSealed, c.Phase,
		)
	}

	if c.Consent.Policy == "" {
		c.Consent.Policy = ConsentStrict
	} else if !validConsentPolicies[c.Consent.Policy] {
		return fmt.Errorf(
			"%s: field %q: must be %q or %q (got %q)",
			path, "consent.policy", ConsentStrict, ConsentOff, c.Consent.Policy,
		)
	}

	if c.SourceTracking.Type == "" {
		c.SourceTracking.Type = SourceTrackingNone
	} else if !validSourceTrackingTypes[c.SourceTracking.Type] {
		return fmt.Errorf(
			"%s: field %q: must be one of %q, %q, %q, %q (got %q)",
			path, "sourceTracking.type",
			SourceTrackingNone, SourceTrackingGeneric, SourceTrackingGitHubIssue, SourceTrackingJira,
			c.SourceTracking.Type,
		)
	}

	for _, t := range c.AgentInstructions.Targets {
		if !validTargets[t] {
			return fmt.Errorf(
				"%s: field %q: unknown target %q (allowed: %q, %q)",
				path, "agentInstructions.targets", t, TargetClaude, TargetAgents,
			)
		}
	}

	for _, tr := range c.Skills.Trees {
		if !validSkillTrees[tr] {
			return fmt.Errorf(
				"%s: field %q: unknown skills tree %q (allowed: %q, %q, %q)",
				path, "skills.trees", tr, SkillTreeClaude, SkillTreeAgents, SkillTreeCursor,
			)
		}
	}

	return nil
}
