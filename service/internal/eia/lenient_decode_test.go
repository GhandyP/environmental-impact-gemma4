package eia

import (
	"strings"
	"testing"
)

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

// TestExtractDropsNestedElements verifies the R3-lenient-object-elements fix:
// a nested object inside a list of strings must be dropped, not stringified
// into noise like "map[a:1]".
func TestExtractDropsNestedElements(t *testing.T) {
	raw := `{"hazard_level":"low","summary":"ok","visible_evidence":["smoke",{"depth":2},"dust",[1,2],42,true],"likely_impact_factors":["x"],"likely_processes":["y"],"recommendations":["z"],"uncertainty":["u"],"confidence":0.5}`
	a, err := Extract(raw)
	if err != nil {
		t.Fatalf("mixed array must not fail the analysis: %v", err)
	}
	got := strings.Join(a.VisibleEvidence, "|")
	if got != "smoke|dust|42|true" {
		t.Fatalf("nested values must be dropped and scalars kept as text, got %q", got)
	}
	for _, item := range a.VisibleEvidence {
		if strings.Contains(item, "map[") || strings.Contains(item, "[") {
			t.Fatalf("nested structure leaked into text: %q", item)
		}
	}
}
