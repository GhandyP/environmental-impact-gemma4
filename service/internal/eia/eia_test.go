package eia

import (
	"math"
	"strings"
	"testing"
)

const testJSON = `{"hazard_level":"low","summary":"A clear scene.","visible_evidence":["soil"],"likely_impact_factors":["runoff"],"likely_processes":["erosion"],"recommendations":["monitor"],"uncertainty":["limited view"],"confidence":0.8}`

func TestExtractJSONVariants(t *testing.T) {
	for _, raw := range []string{testJSON, "```json\n" + testJSON + "\n```", "prefix text\n" + testJSON + "\ntrailing text"} {
		a, err := Extract(raw)
		if err != nil || a.Summary == "" {
			t.Fatalf("Extract(%q): %v, %#v", raw, err, a)
		}
	}
}
func TestExtractValidationErrors(t *testing.T) {
	for _, raw := range []string{strings.Replace(testJSON, `"low"`, `"unknown"`, 1), strings.Replace(testJSON, `0.8`, `1.1`, 1)} {
		if _, err := Extract(raw); err == nil {
			t.Errorf("expected validation error for %s", raw)
		}
	}
}
func TestExtractNormalizesAnalysis(t *testing.T) {
	raw := `{"hazard_level":"low","summary":"  A clear scene.  ","visible_evidence":["  soil  ","soil"," ","water"],"likely_impact_factors":[" runoff ","runoff",""],"likely_processes":[" erosion ","erosion"],"recommendations":[" monitor ","monitor","  "],"uncertainty":[" limited view ","limited view"],"confidence":0.8}`
	a, err := Extract(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.Summary != "A clear scene." || strings.Join(a.VisibleEvidence, ",") != "soil,water" || strings.Join(a.LikelyImpactFactors, ",") != "runoff" {
		t.Fatalf("analysis was not normalized: %#v", a)
	}
}
func TestExtractAcceptsSixVisibleEvidenceItems(t *testing.T) {
	raw := strings.Replace(testJSON, `"soil"`, `"a","b","c","d","e","f"`, 1)
	if _, err := Extract(raw); err != nil {
		t.Fatalf("six visible evidence items should be accepted: %v", err)
	}
}
func TestValidateRejectsNaNConfidence(t *testing.T) {
	a := &Analysis{HazardLevel: "low", Summary: "summary", Confidence: math.NaN()}
	if err := a.Validate(); err == nil {
		t.Fatal("expected NaN confidence validation error")
	}
}
func TestBuildPromptSpanishContract(t *testing.T) {
	p := BuildPrompt(LangES)
	if !strings.Contains(p, "impacto") {
		t.Error("Spanish prompt lacks impacto")
	}
	for _, key := range []string{"hazard_level", "summary", "visible_evidence", "likely_impact_factors", "likely_processes", "recommendations", "uncertainty", "confidence"} {
		if !strings.Contains(p, key) {
			t.Errorf("prompt lacks %q", key)
		}
	}
}
func TestParseLang(t *testing.T) {
	for _, s := range []string{"en", "es"} {
		if _, err := ParseLang(s); err != nil {
			t.Error(err)
		}
	}
	if _, err := ParseLang("fr"); err == nil {
		t.Error("expected rejection")
	}
}
