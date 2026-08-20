package config

import (
	"strings"
	"testing"
)

// baseValidConfig returns a Config that passes validate() with every
// optional field left at its zero value, so a single field under test can
// vary in isolation.
func baseValidConfig() Config {
	return Config{
		SchemaVersion: SchemaVersion,
		Phase:         PhaseSealed,
		Categories:    []string{"architecture"},
	}
}

// fieldByKey looks up a Field from Schema() by its Key, failing the test
// if Schema() has drifted and no longer declares it.
func fieldByKey(t *testing.T, key string) Field {
	t.Helper()
	for _, f := range Schema() {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("Schema() has no field %q", key)
	return Field{}
}

// TestSchemaEnumsAcceptedByValidator proves every value Schema() declares
// for an enumerated field is one Config.validate actually accepts: the
// published schema can never advertise a value the validator rejects.
func TestSchemaEnumsAcceptedByValidator(t *testing.T) {
	tests := []struct {
		key   string
		apply func(cfg *Config, value string)
	}{
		{"sourceTracking.type", func(cfg *Config, v string) { cfg.SourceTracking.Type = v }},
		{"consent.policy", func(cfg *Config, v string) { cfg.Consent.Policy = v }},
		{"phase", func(cfg *Config, v string) { cfg.Phase = v }},
		{"agentInstructions.targets", func(cfg *Config, v string) { cfg.AgentInstructions.Targets = []string{v} }},
		{"skills.trees", func(cfg *Config, v string) { cfg.Skills.Trees = []string{v} }},
	}

	for _, tt := range tests {
		f := fieldByKey(t, tt.key)
		if len(f.Values) == 0 {
			t.Fatalf("field %q declares no Values to test", tt.key)
		}
		for _, v := range f.Values {
			t.Run(tt.key+"="+v, func(t *testing.T) {
				cfg := baseValidConfig()
				tt.apply(&cfg, v)
				if err := cfg.validate("constitution.yml"); err != nil {
					t.Errorf("validate() rejected declared value %q for %s: %v", v, tt.key, err)
				}
			})
		}
	}
}

// TestSchemaRejectsUndeclaredEnumValue proves a value absent from
// Schema()'s declared sourceTracking.type Values — specifically "github",
// the exact value that caused issue #17 (the actual enum member is
// "github-issue") — is one Config.validate rejects: the published schema
// can never omit a value the validator would accept.
func TestSchemaRejectsUndeclaredEnumValue(t *testing.T) {
	f := fieldByKey(t, "sourceTracking.type")
	for _, v := range f.Values {
		if v == "github" {
			t.Fatalf("Schema() declares %q for sourceTracking.type; it must NOT, or this test proves nothing", v)
		}
	}

	cfg := baseValidConfig()
	cfg.SourceTracking.Type = "github"
	err := cfg.validate("constitution.yml")
	if err == nil {
		t.Fatal(`validate() accepted "github" for sourceTracking.type, want an error (only "github-issue" is valid)`)
	}
	if !strings.Contains(err.Error(), "sourceTracking.type") {
		t.Errorf("validate() error = %q, want it to mention sourceTracking.type", err.Error())
	}
}

// TestSchemaEnumValuesAreDerivedFromValidators is issue #26's
// regression guard. Schema() must not RE-LIST any enumerated
// vocabulary: every Values list has to come from the same map
// validate enforces, so adding a value in one place cannot leave the
// other stale. The check is structural (Values == sortedKeys(map)),
// not a restatement of the same literals in a test file — #17's
// lesson was that a literal list in a test proves nothing.
func TestSchemaEnumValuesAreDerivedFromValidators(t *testing.T) {
	tests := []struct {
		key   string
		vocab map[string]bool
	}{
		{"agentInstructions.targets", validTargets},
		{"consent.policy", validConsentPolicies},
		{"sourceTracking.type", validSourceTrackingTypes},
		{"phase", validPhases},
		{"skills.trees", validSkillTrees},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			f := fieldByKey(t, tt.key)
			want := sortedKeys(tt.vocab)
			if len(f.Values) != len(want) {
				t.Fatalf("Schema() %s Values = %v, want %v (derived from the validator map)",
					tt.key, f.Values, want)
			}
			for i := range want {
				if f.Values[i] != want[i] {
					t.Errorf("Schema() %s Values[%d] = %q, want %q", tt.key, i, f.Values[i], want[i])
				}
			}
		})
	}
}
