package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/GhandyP/environmental-impact-gemma4/service/internal/config"
	"github.com/GhandyP/environmental-impact-gemma4/service/internal/llm"
)

type apiFake struct {
	name, model string
	available   bool
	response    string
	err         error
}

func (f apiFake) Name() string                   { return f.name }
func (f apiFake) Model() string                  { return f.model }
func (f apiFake) Available(context.Context) bool { return f.available }
func (f apiFake) Info() llm.ProviderInfo {
	return llm.ProviderInfo{Name: f.name, Model: f.model, Available: f.available}
}
func (f apiFake) Generate(context.Context, string, []byte, string) (string, error) {
	return f.response, f.err
}

const apiJSON = `{"hazard_level":"low","summary":"safe","confidence":0}`

func pngBytes() []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	return b.Bytes()
}
func multipartRequest(t *testing.T, data []byte, lang string) *http.Request {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="x.bin"`)
	h.Set("Content-Type", "application/octet-stream")
	part, _ := mw.CreatePart(h)
	part.Write(data)
	mw.WriteField("lang", lang)
	mw.Close()
	r := httptest.NewRequest(http.MethodPost, "/analyze", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}
func testHandler(a []llm.Analyzer, max int64) http.Handler {
	return NewHandler(config.Config{MaxImageBytes: max, ProviderTimeout: time.Second}, a)
}
func TestAnalyzeMultipart(t *testing.T) {
	r := multipartRequest(t, pngBytes(), "es")
	w := httptest.NewRecorder()
	testHandler([]llm.Analyzer{apiFake{"fake", "m", true, apiJSON, nil}}, 10000).ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("X-EIA-Provider") != "fake" {
		t.Fatalf("status %d header %q body %s", w.Code, w.Header().Get("X-EIA-Provider"), w.Body)
	}
	for _, s := range []string{`"provider"`, `"model"`, `"lang"`, `"analysis"`} {
		if !strings.Contains(w.Body.String(), s) {
			t.Error("missing " + s)
		}
	}
}
func TestAnalyzeBadInputs(t *testing.T) {
	h := testHandler([]llm.Analyzer{apiFake{"f", "m", true, apiJSON, nil}}, 100)
	for _, data := range [][]byte{{1, 2, 3}, bytes.Repeat([]byte{1}, 101)} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, multipartRequest(t, data, "en"))
		if w.Code != 400 {
			t.Errorf("bad image status %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, multipartRequest(t, pngBytes(), "xx"))
	if w.Code != 400 {
		t.Errorf("bad lang status %d", w.Code)
	}
}
func TestAnalyzeProviderErrors(t *testing.T) {
	w := httptest.NewRecorder()
	testHandler([]llm.Analyzer{apiFake{"f", "m", true, "garbage", nil}}, 10000).ServeHTTP(w, multipartRequest(t, pngBytes(), "en"))
	if w.Code != 502 {
		t.Errorf("extract status %d", w.Code)
	}
	w = httptest.NewRecorder()
	testHandler(nil, 10000).ServeHTTP(w, multipartRequest(t, pngBytes(), "en"))
	if w.Code != 503 {
		t.Errorf("no provider status %d", w.Code)
	}
}
func TestHealthAndHome(t *testing.T) {
	h := testHandler([]llm.Analyzer{apiFake{"a", "1", true, "", nil}, apiFake{"b", "2", false, "", nil}}, 1000)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var out struct {
		Providers []llm.ProviderInfo `json:"providers"`
	}
	if json.Unmarshal(w.Body.Bytes(), &out) != nil || len(out.Providers) != 2 {
		t.Fatalf("health: %s", w.Body)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(w.Body.String(), "EIA Service") {
		t.Error("missing title")
	}
}
