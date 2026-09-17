package eia

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

type Lang string

const (
	LangEN Lang = "en"
	LangES Lang = "es"
)

func ParseLang(s string) (Lang, error) {
	switch s {
	case "en":
		return LangEN, nil
	case "es":
		return LangES, nil
	default:
		return "", fmt.Errorf("unsupported language %q", s)
	}
}

type Analysis struct {
	HazardLevel         string   `json:"hazard_level"`
	Summary             string   `json:"summary"`
	VisibleEvidence     []string `json:"visible_evidence"`
	LikelyImpactFactors []string `json:"likely_impact_factors"`
	LikelyProcesses     []string `json:"likely_processes"`
	Recommendations     []string `json:"recommendations"`
	Uncertainty         []string `json:"uncertainty"`
	Confidence          float64  `json:"confidence"`
}

func (a *Analysis) Normalize() {
	if a == nil {
		return
	}
	a.Summary = strings.TrimSpace(a.Summary)
	normalize := func(values []string) []string {
		seen := make(map[string]struct{}, len(values))
		cleaned := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			cleaned = append(cleaned, value)
		}
		return cleaned
	}
	a.VisibleEvidence = normalize(a.VisibleEvidence)
	a.LikelyImpactFactors = normalize(a.LikelyImpactFactors)
	a.LikelyProcesses = normalize(a.LikelyProcesses)
	a.Recommendations = normalize(a.Recommendations)
	a.Uncertainty = normalize(a.Uncertainty)
}

func (a *Analysis) Validate() error {
	if a == nil {
		return errors.New("analysis is nil")
	}
	switch a.HazardLevel {
	case "low", "medium", "high", "critical":
	default:
		return fmt.Errorf("invalid hazard_level %q", a.HazardLevel)
	}
	if strings.TrimSpace(a.Summary) == "" {
		return errors.New("summary is empty")
	}
	if math.IsNaN(a.Confidence) || a.Confidence < 0 || a.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	return nil
}
func BuildPrompt(lang Lang) string {
	if lang == LangES {
		return `Sos un analista senior de impacto ambiental.

Inspeccioná la imagen y devolvé ÚNICAMENTE JSON válido con exactamente estas claves:
- hazard_level: uno de ["low", "medium", "high", "critical"]
- summary: una oración
- visible_evidence: 3 a 5 cadenas breves
- likely_impact_factors: 3 a 5 cadenas breves
- likely_processes: 2 a 4 cadenas breves
- recommendations: 3 a 5 cadenas breves
- uncertainty: 1 a 3 cadenas breves
- confidence: un número de 0 a 1

Reglas:
- No uses markdown ni cercos de código.
- No inventes detalles que no estén claramente respaldados por la imagen.
- Si la imagen es ambigua, indicá la ambigüedad en uncertainty.`
	}
	return `You are a senior environmental impact analyst.

Inspect the image and return ONLY valid JSON with exactly these keys:
- hazard_level: one of ["low", "medium", "high", "critical"]
- summary: one sentence
- visible_evidence: 3 to 5 short strings
- likely_impact_factors: 3 to 5 short strings
- likely_processes: 2 to 4 short strings
- recommendations: 3 to 5 short strings
- uncertainty: 1 to 3 short strings
- confidence: a number from 0 to 1

Rules:
- Do not use markdown or code fences.
- Do not invent details that are not clearly supported by the image.
- If the image is ambiguous, say so in uncertainty.`
}
func Extract(raw string) (*Analysis, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "```json"))
		if strings.HasSuffix(raw, "```") {
			raw = strings.TrimSpace(strings.TrimSuffix(raw, "```"))
		}
	}
	start := strings.IndexByte(raw, '{')
	if start < 0 {
		return nil, errors.New("no JSON object found")
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '{' {
			depth++
		}
		if c == '}' {
			depth--
			if depth == 0 {
				var a Analysis
				if err := json.Unmarshal([]byte(raw[start:i+1]), &a); err != nil {
					return nil, err
				}
				a.Normalize()
				return &a, a.Validate()
			}
		}
	}
	return nil, errors.New("unbalanced JSON object")
}
