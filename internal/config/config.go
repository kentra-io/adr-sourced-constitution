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

// Config is the constitution.yml schema (plan §2.10, §2.5, §2.8).
type Config struct {
	SchemaVersion     int               `yaml:"schemaVersion"`
	AgentInstructions AgentInstructions `yaml:"agentInstructions"`
	Consent           Consent           `yaml:"consent"`
	SourceTracking    SourceTracking    `yaml:"sourceTracking"`
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

// Load reads and validates constitution.yml at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: not valid YAML: %w", path, err)
	}

	if err := cfg.validate(path); err != nil {
		return nil, err
	}
	return &cfg, nil
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

	if c.Consent.Policy == "" {
		c.Consent.Policy = ConsentStrict
	} else if c.Consent.Policy != ConsentStrict && c.Consent.Policy != ConsentOff {
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

	return nil
}
