package config

import "sort"

// Field describes one constitution.yml config field for machine consumers
// (issue #18): a skill or other tool reads Schema() instead of hardcoding
// the vocabulary, which is how "github" vs "github-issue" (issue #17)
// drifted in the first place.
type Field struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  any      `json:"default,omitempty"`
	Values   []string `json:"values,omitempty"`
	Doc      string   `json:"doc"`
}

// Schema returns one Field per constitution.yml config field, in Config's
// declaration order. Every enumerated Values list is derived via
// sortedKeys from the same map-shaped vocabulary Config.validate
// enforces, so Schema() can never re-list an enum value out of step
// with validate.
func Schema() []Field {
	return []Field{
		{
			Key:      "schemaVersion",
			Type:     "int",
			Required: true,
			Doc:      "The only schemaVersion this build understands; every other value is refused (no migration machinery in v1).",
		},
		{
			Key:      "agentInstructions.targets",
			Type:     "[]string",
			Required: false,
			Values:   sortedKeys(validTargets),
			Doc:      "Which agent-instruction file(s) get the managed pointer block: \"claude\" writes an @import pointer to CLAUDE.md, \"agents\" a short textual pointer to AGENTS.md.",
		},
		{
			Key:      "consent.policy",
			Type:     "string",
			Required: false,
			Default:  ConsentStrict,
			Values:   sortedKeys(validConsentPolicies),
			Doc:      "Whether mutating verbs require interactive or --approve confirmation (\"strict\") or proceed unattended (\"off\").",
		},
		{
			Key:      "sourceTracking.type",
			Type:     "string",
			Required: false,
			Default:  SourceTrackingNone,
			Values:   sortedKeys(validSourceTrackingTypes),
			Doc:      "The source-ref format ADRs may carry in a `source` field. \"none\" forbids `source` entirely; the others require it and check it against a type-specific (or sourceTracking.pattern-overridden) shape.",
		},
		{
			Key:      "sourceTracking.pattern",
			Type:     "string",
			Required: false,
			Doc:      "Overrides the default source-ref regexp for sourceTracking.type's github-issue/jira defaults, or supplies one for \"generic\". Free-form: not checked against a fixed vocabulary.",
		},
		{
			Key:      "phase",
			Type:     "string",
			Required: true,
			Values:   sortedKeys(validPhases),
			Doc:      "The founding phase: \"draft\" (fully mutable working set, no manifest baseline) or \"sealed\" (append-only forever, full guard semantics). No default — sealing is always an explicit, human-approved act.",
		},
		{
			Key:      "categories",
			Type:     "[]string",
			Required: true,
			Doc:      "The project's category vocabulary, in the order the projection groups sections by. At least one entry is required; entries must be non-empty and unique.",
		},
		{
			Key:      "skills.trees",
			Type:     "[]string",
			Required: false,
			Values:   sortedKeys(validSkillTrees),
			Doc:      "Which agent-skill trees `constitution init` fans Layer-2 skills out to and `regen` keeps refreshed.",
		},
	}
}

// sortedKeys returns m's keys in sorted order, giving Schema() a
// deterministic Values list over the package's map-shaped vocabularies
// without re-listing their members as separate string literals.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
