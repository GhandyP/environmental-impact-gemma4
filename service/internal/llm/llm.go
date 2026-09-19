package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/genai"
)

type ProviderInfo struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Model     string `json:"model"`
}
type Analyzer interface {
	Name() string
	Model() string
	Available(context.Context) bool
	Info() ProviderInfo
	Generate(context.Context, string, []byte, string) (string, error)
}

type local struct {
	base, model string
	client      *http.Client
}

func NewLocal(baseURL, model string, client *http.Client) Analyzer {
	if client == nil {
		client = http.DefaultClient
	}
	return &local{strings.TrimRight(baseURL, "/"), model, client}
}
func (p *local) Name() string  { return "local" }
func (p *local) Model() string { return p.model }
func (p *local) Available(ctx context.Context) bool {
	if p.base == "" {
		return false
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(c, http.MethodGet, p.base+"/health", nil)
	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
func (p *local) Info() ProviderInfo {
	return ProviderInfo{p.Name(), p.Available(context.Background()), p.model}
}

// localRequestBody builds the OpenAI-compatible payload sent to llama-server.
//
// reasoning_effort is pinned to "none" on purpose. Verified against a real
// llama.cpp server started WITHOUT --reasoning off: without this field Gemma 4
// spends the whole token budget on reasoning_content and never emits the JSON
// analysis, which the service then reports as a parse failure. Sending it per
// request makes the service independent of how the server was launched.
func localRequestBody(model, prompt, mime string, image []byte) map[string]any {
	return map[string]any{
		"model":            model,
		"reasoning_effort": "none",
		"max_tokens":       512,
		"messages": []any{map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image)}},
				map[string]any{"type": "text", "text": prompt},
			},
		}},
	}
}

func (p *local) Generate(ctx context.Context, prompt string, image []byte, mime string) (string, error) {
	if p.base == "" {
		return "", errors.New("local provider unavailable")
	}
	if mime == "" {
		mime = "image/jpeg"
	}
	body := localRequestBody(p.model, prompt, mime, image)
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("local provider returned %s", resp.Status)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("local response has no choices")
	}
	return out.Choices[0].Message.Content, nil
}

type gemini struct {
	key, model string
	timeout    time.Duration
	mu         sync.Mutex
	client     *genai.Client
}

func NewGemini(key, model string, timeout time.Duration) Analyzer {
	return &gemini{key: key, model: model, timeout: timeout}
}
func (p *gemini) Name() string                   { return "gemini" }
func (p *gemini) Model() string                  { return p.model }
func (p *gemini) Available(context.Context) bool { return p.key != "" }
func (p *gemini) Info() ProviderInfo {
	return ProviderInfo{p.Name(), p.Available(context.Background()), p.model}
}
func (p *gemini) Generate(ctx context.Context, prompt string, image []byte, mime string) (string, error) {
	if p.key == "" {
		return "", errors.New("gemini provider unavailable")
	}
	if mime == "" {
		mime = "image/jpeg"
	}
	if p.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		c, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: p.key, Backend: genai.BackendGeminiAPI})
		if err != nil {
			return "", err
		}
		p.client = c
	}
	resp, err := p.client.Models.GenerateContent(ctx, p.model, []*genai.Content{{Parts: []*genai.Part{{Text: prompt}, {InlineData: &genai.Blob{Data: image, MIMEType: mime}}}}}, &genai.GenerateContentConfig{ResponseMIMEType: "application/json", MaxOutputTokens: 512})
	if err != nil {
		return "", err
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", errors.New("gemini response has no candidates")
	}
	var b strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		b.WriteString(part.Text)
	}
	return b.String(), nil
}

type hybrid struct {
	providers []Analyzer
	last      Analyzer
}

func NewHybrid(a []Analyzer) Analyzer { return &hybrid{providers: a} }
func (h *hybrid) Available(ctx context.Context) bool {
	for _, p := range h.providers {
		if p.Available(ctx) {
			return true
		}
	}
	return false
}
func (h *hybrid) Name() string {
	if h.last != nil {
		return h.last.Name()
	}
	return "hybrid"
}
func (h *hybrid) Model() string {
	if h.last != nil {
		return h.last.Model()
	}
	return ""
}
func (h *hybrid) Info() ProviderInfo {
	return ProviderInfo{h.Name(), h.Available(context.Background()), h.Model()}
}
func (h *hybrid) Generate(ctx context.Context, prompt string, image []byte, mime string) (string, error) {
	var last error
	found := false
	for _, p := range h.providers {
		if !p.Available(ctx) {
			continue
		}
		found = true
		h.last = p
		s, err := p.Generate(ctx, prompt, image, mime)
		if err == nil {
			return s, nil
		}
		last = err
	}
	if !found {
		return "", errors.New("no model provider available")
	}
	return "", last
}
