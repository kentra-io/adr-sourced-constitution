package adr

import (
	"strings"
	"testing"
)

func TestParseRulesSectionHappy(t *testing.T) {
	content := strings.TrimSpace(`
### architecture

#### hex-core
Structure the service as hexagonal (ports and adapters); the domain
core imports no framework or adapter types.

#### explicit-boundary-mapping
Boundary mapping is explicit per adapter.

### testing

#### three-tier-tests
Three tiers: per-class unit, domain-with-fakes, integration.
`)
	rules, err := ParseRulesSection(content, "x.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Rule{
		{Category: "architecture", Slug: "hex-core", Text: "Structure the service as hexagonal (ports and adapters); the domain\ncore imports no framework or adapter types."},
		{Category: "architecture", Slug: "explicit-boundary-mapping", Text: "Boundary mapping is explicit per adapter."},
		{Category: "testing", Slug: "three-tier-tests", Text: "Three tiers: per-class unit, domain-with-fakes, integration."},
	}
	if len(rules) != len(want) {
		t.Fatalf("got %d rules, want %d: %+v", len(rules), len(want), rules)
	}
	for i := range want {
		if rules[i] != want[i] {
			t.Errorf("rule %d = %+v, want %+v", i, rules[i], want[i])
		}
	}
}

// Every rejection carries a *ParseError naming the Rules section.
func TestParseRulesSectionErrors(t *testing.T) {
	cases := []struct{ name, content, wantSubstr string }{
		{"rule entry before first category",
			"#### x\nText.", `rule entry "x" must appear under a "### <category>" heading`},
		{"untagged text before first category",
			"loose prose\n\n### testing\n\n#### a\nText.", `must appear under a "### <category>" heading`},
		{"text under category before first slug",
			"### testing\nloose prose\n\n#### a\nText.", `must appear under a "#### <slug>" rule heading`},
		{"empty rule body",
			"### testing\n\n#### a\n\n#### b\nText.", `rule "testing/a" has no text`},
		{"duplicate slug in category",
			"### testing\n\n#### a\nOne.\n\n#### a\nTwo.", `duplicate rule slug "a" in category "testing"`},
		{"bad slug",
			"### testing\n\n#### Not_Kebab\nText.", `rule slug "Not_Kebab" must be kebab-case`},
		{"bad category",
			"### Testing\n\n#### a\nText.", `category "Testing" must be kebab-case`},
		{"stray deeper heading in text",
			"### testing\n\n#### a\nText.\n##### sub", `must not contain Markdown heading lines`},
		{"empty section", "", `the "## Rules" section is present but empty`},
		{"trailing category with no entries",
			"### a\n\n#### x\nT.\n\n### b\n", `category "b" has no rule entries`},
		{"empty first category",
			"### a\n\n### b\n\n#### x\nT.", `category "a" has no rule entries`},
		{"intermediate empty category",
			"### a\n\n#### x\nT.\n\n### b\n\n### c\n\n#### y\nU.", `category "b" has no rule entries`},
		{"re-opened category",
			"### a\n\n#### x\nX.\n\n### b\n\n#### y\nY.\n\n### a\n\n#### z\nZ.", `category "a" appears more than once`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRulesSection(c.content, "x.md")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			pe, ok := err.(*ParseError)
			if !ok {
				t.Fatalf("error type %T, want *ParseError", err)
			}
			if pe.Field != RulesSection {
				t.Errorf("Field = %q, want %q", pe.Field, RulesSection)
			}
			if !strings.Contains(pe.Msg, c.wantSubstr) {
				t.Errorf("Msg = %q, want substring %q", pe.Msg, c.wantSubstr)
			}
		})
	}
}
