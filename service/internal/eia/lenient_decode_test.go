package eia

import "testing"

// TestExtractAcceptsStringWhereArrayExpected reproduces the real failure seen
// against a local Gemma model: the model collapsed single-item lists to a
// plain string, and json.Unmarshal rejected the whole analysis.
func TestExtractAcceptsStringWhereArrayExpected(t *testing.T) {
	raw := `{"hazard_level":"high","summary":"Smokestacks emit pollution.","visible_evidence":"thick smoke plumes","likely_impact_factors":["air pollution"],"likely_processes":["combustion"],"recommendations":["monitor emissions"],"uncertainty":"operation status unclear","confidence":0.7}`
	a, err := Extract(raw)
	if err != nil {
		t.Fatalf("string-valued list field must be accepted: %v", err)
	}
	if len(a.VisibleEvidence) != 1 || a.VisibleEvidence[0] != "thick smoke plumes" {
		t.Fatalf("visible_evidence not coerced: %#v", a.VisibleEvidence)
	}
	if len(a.Uncertainty) != 1 || a.Uncertainty[0] != "operation status unclear" {
		t.Fatalf("uncertainty not coerced: %#v", a.Uncertainty)
	}
}
