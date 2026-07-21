package adr

import "testing"

func TestParseRuleRef(t *testing.T) {
	r, err := ParseRuleRef("ADR-0004/testing/three-tier-tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := RuleRef{ADRID: "ADR-0004", Category: "testing", Slug: "three-tier-tests"}
	if r != want {
		t.Fatalf("got %+v, want %+v", r, want)
	}
	if got := r.String(); got != "ADR-0004/testing/three-tier-tests" {
		t.Fatalf("String() = %q", got)
	}
}

func TestParseRuleRefErrors(t *testing.T) {
	cases := []struct{ in, wantMsg string }{
		{"ADR-0004/testing", `rule ref must be "ADR-NNNN/<category>/<slug>" (got "ADR-0004/testing")`},
		{"adr-4/testing/x", `rule ref must be "ADR-NNNN/<category>/<slug>" (got "adr-4/testing/x"): id "adr-4" must match "ADR-NNNN"`},
		{"ADR-0004/Testing/x", `rule ref must be "ADR-NNNN/<category>/<slug>" (got "ADR-0004/Testing/x"): category "Testing" must be kebab-case`},
		{"ADR-0004/testing/Three_Tier", `rule ref must be "ADR-NNNN/<category>/<slug>" (got "ADR-0004/testing/Three_Tier"): slug "Three_Tier" must be kebab-case`},
	}
	for _, c := range cases {
		_, err := ParseRuleRef(c.in)
		if err == nil || err.Error() != c.wantMsg {
			t.Errorf("ParseRuleRef(%q) error = %v, want %q", c.in, err, c.wantMsg)
		}
	}
}

func TestIsKebab(t *testing.T) {
	for s, want := range map[string]bool{
		"a": true, "three-tier-tests": true, "a1-b2": true,
		"": false, "-a": false, "a-": false, "a--b": false, "A": false, "a_b": false,
	} {
		if got := isKebab(s); got != want {
			t.Errorf("isKebab(%q) = %v, want %v", s, got, want)
		}
	}
}
