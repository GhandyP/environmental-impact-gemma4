package eia

import (
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
