package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAnalyzer struct {
	name, model string
	available   bool
	result      string
	err         error
}

func (f fakeAnalyzer) Name() string                   { return f.name }
func (f fakeAnalyzer) Model() string                  { return f.model }
func (f fakeAnalyzer) Available(context.Context) bool { return f.available }
func (f fakeAnalyzer) Info() ProviderInfo             { return ProviderInfo{f.name, f.available, f.model} }
func (f fakeAnalyzer) Generate(context.Context, string, []byte, string) (string, error) {
	return f.result, f.err
}

func TestLocalGenerateAndAvailable(t *testing.T) {
	var got []byte
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(200)
			return
		}
		got, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"choices":[{"message":{"content":"response text"}}]}`))
	}))
	defer s.Close()
	p := NewLocal(s.URL, "model", s.Client())
	if !p.Available(context.Background()) {
		t.Fatal("expected available")
	}
	out, err := p.Generate(context.Background(), "my prompt", []byte("x"), "")
	if err != nil || out != "response text" {
		t.Fatalf("got %q, %v", out, err)
	}
	body := string(got)
	for _, want := range []string{"image_url", "data:image/jpeg;base64,", "my prompt"} {
		if !strings.Contains(body, want) {
			t.Errorf("request lacks %q", want)
		}
	}
	var decoded map[string]any
	if json.Unmarshal(got, &decoded) != nil {
		t.Error("invalid request JSON")
	}
	// Verified against a real llama-server started without --reasoning off:
	// without this field Gemma 4 burns the whole budget on reasoning_content
	// and never emits the JSON analysis.
	if effort, _ := decoded["reasoning_effort"].(string); effort != "none" {
		t.Errorf("local request must disable reasoning, got reasoning_effort=%v", decoded["reasoning_effort"])
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	url := closed.URL
	closed.Close()
	if NewLocal(url, "", nil).Available(context.Background()) {
		t.Error("closed server available")
	}
	if NewLocal("", "", nil).Available(context.Background()) {
		t.Error("empty base available")
	}
}
func TestGeminiEmpty(t *testing.T) {
	p := NewGemini("", "model", 0)
	if p.Available(context.Background()) {
		t.Error("empty key available")
	}
	if _, err := p.Generate(context.Background(), "", nil, ""); err == nil {
		t.Error("expected error")
	}
}
func TestHybrid(t *testing.T) {
	first := fakeAnalyzer{"first", "1", true, "", context.Canceled}
	second := fakeAnalyzer{"second", "2", true, "ok", nil}
	h := NewHybrid([]Analyzer{first, second})
	if out, err := h.Generate(context.Background(), "", nil, ""); err != nil || out != "ok" || h.Name() != "second" {
		t.Fatalf("hybrid: %q %v", out, err)
	}
	if _, err := NewHybrid([]Analyzer{fakeAnalyzer{available: false}}).Generate(context.Background(), "", nil, ""); err == nil || err.Error() != "no model provider available" {
		t.Fatalf("unexpected unavailable error: %v", err)
	}
	if _, err := NewHybrid([]Analyzer{fakeAnalyzer{available: true, err: context.Canceled}, fakeAnalyzer{available: true, err: context.DeadlineExceeded}}).Generate(context.Background(), "", nil, ""); err != context.DeadlineExceeded {
		t.Fatalf("last error: %v", err)
	}
	if NewHybrid(nil).Available(context.Background()) {
		t.Error("empty hybrid available")
	}
}
