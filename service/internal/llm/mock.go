package llm

import (
	"context"
)

// mock is a deterministic provider for demos and integration checks.
// It never touches the network and always reports available.
type mock struct{ model string }

// NewMock returns a provider that answers with a fixed valid EIA JSON
// document. Enable it with EIA_MOCK=1 when no GPU or API key is available.
func NewMock(model string) Analyzer { return &mock{model: model} }

func (m *mock) Name() string                   { return "mock" }
func (m *mock) Model() string                  { return m.model }
func (m *mock) Available(context.Context) bool { return true }
func (m *mock) Info() ProviderInfo             { return ProviderInfo{m.Name(), true, m.model} }
func (m *mock) Generate(context.Context, string, []byte, string) (string, error) {
	return `{"hazard_level":"high",
  "summary":"Coal-fired smokestacks release dense emissions over an industrial riverside area.",
  "visible_evidence":["Tall smokestacks emitting thick smoke","Industrial complex adjacent to a waterway","No visible emission controls"],
  "likely_impact_factors":["Air quality degradation","Particulate deposition on nearby communities","Thermal or chemical discharge risk to the river"],
  "likely_processes":["Fossil fuel combustion","Wastewater discharge to surface water"],
  "recommendations":["Install or verify flue gas filtration","Continuous emissions monitoring","Environmental sampling downwind and downstream"],
  "uncertainty":["Era and current operation status are unclear from a single photo"],
  "confidence":0.72}`, nil
}
